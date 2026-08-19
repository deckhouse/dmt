// Copyright 2026 Flant JSC
// Licensed under the Apache License, Version 2.0

package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestNewSizeRule(t *testing.T) {
	rule := NewSizeRule()
	assert.Equal(t, SizeRuleName, rule.GetName())
	assert.Equal(t, "size", rule.GetName())
}

// writeSparseFile creates a file of the requested logical size without actually
// writing the bytes, so the size rule sees a large docs/ directory cheaply.
func writeSparseFile(t *testing.T, path string, size int64) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))

	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(size))
	require.NoError(t, f.Close())
}

func TestSizeRule_CheckSize(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]int64 // path relative to module root -> size in bytes
		noDocsDir bool
		wantError bool
	}{
		{
			name:      "no docs directory is skipped",
			noDocsDir: true,
			wantError: false,
		},
		{
			name:      "small docs directory passes",
			files:     map[string]int64{"docs/README.md": 1024},
			wantError: false,
		},
		{
			name:      "docs exactly at the limit passes",
			files:     map[string]int64{"docs/README.md": maxDocsSizeBytes},
			wantError: false,
		},
		{
			name:      "single oversized file fails",
			files:     map[string]int64{"docs/big.png": maxDocsSizeBytes + 1},
			wantError: true,
		},
		{
			name: "sum across files and non-internal subfolders exceeds the limit",
			files: map[string]int64{
				"docs/README.md":       8 * 1024 * 1024,
				"docs/guides/setup.md": 8 * 1024 * 1024,
			},
			wantError: true,
		},
		{
			name: "large file in a non-internal subfolder fails",
			files: map[string]int64{
				"docs/guides/big.png": maxDocsSizeBytes + 1,
			},
			wantError: true,
		},
		{
			name: "docs/internal subtree is ignored",
			files: map[string]int64{
				"docs/README.md":         1024,
				"docs/internal/data.bin": maxDocsSizeBytes + 1,
			},
			wantError: false,
		},
		{
			name: "only the top-level docs/internal is excluded",
			files: map[string]int64{
				"docs/guides/internal/data.bin": maxDocsSizeBytes + 1,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := minimock.NewController(t)
			mockModule := mocks.NewModuleMock(mc)

			tempDir := t.TempDir()
			mockModule.GetPathMock.Return(tempDir)

			if !tt.noDocsDir {
				require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "docs"), 0o755))
			}

			for relPath, size := range tt.files {
				writeSparseFile(t, filepath.Join(tempDir, relPath), size)
			}

			rule := NewSizeRule()
			errorList := errors.NewLintRuleErrorsList()

			rule.CheckSize(mockModule, errorList)

			if tt.wantError {
				assert.NotEmpty(t, errorList.GetErrors(), "expected an error for oversized docs/ directory")
			} else {
				assert.Empty(t, errorList.GetErrors(), "expected no error for docs/ directory within the limit")
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		size int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{10 * 1024 * 1024, "10.0 MB"},
		{maxDocsSizeBytes + 1, "15.0 MB"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, formatBytes(tt.size))
	}
}
