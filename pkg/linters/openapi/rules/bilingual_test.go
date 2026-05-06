package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestBilingualRule(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string
		checkFile  string
		wantErrors []string
	}{
		{
			name: "base file with translation present",
			files: map[string]string{
				"config-values.yaml":        "type: object",
				"doc-ru-config-values.yaml": "type: object",
			},
			checkFile:  "config-values.yaml",
			wantErrors: nil,
		},
		{
			name: "base file without translation",
			files: map[string]string{
				"config-values.yaml": "type: object",
			},
			checkFile:  "config-values.yaml",
			wantErrors: []string{"translation file is missing: expected \"doc-ru-config-values.yaml\""},
		},
		{
			name: "doc-ru file with base file present",
			files: map[string]string{
				"config-values.yaml":        "type: object",
				"doc-ru-config-values.yaml": "type: object",
			},
			checkFile:  "doc-ru-config-values.yaml",
			wantErrors: nil,
		},
		{
			name: "orphaned doc-ru file without base file",
			files: map[string]string{
				"doc-ru-config-values.yaml": "type: object",
			},
			checkFile:  "doc-ru-config-values.yaml",
			wantErrors: []string{"translation file has no corresponding base file: expected \"config-values.yaml\""},
		},
		{
			name: "CRD base file without translation",
			files: map[string]string{
				"my-crd.yaml": "apiVersion: apiextensions.k8s.io/v1",
			},
			checkFile:  "my-crd.yaml",
			wantErrors: []string{"translation file is missing: expected \"doc-ru-my-crd.yaml\""},
		},
		{
			name: "CRD base file with translation",
			files: map[string]string{
				"my-crd.yaml":        "apiVersion: apiextensions.k8s.io/v1",
				"doc-ru-my-crd.yaml": "apiVersion: apiextensions.k8s.io/v1",
			},
			checkFile:  "my-crd.yaml",
			wantErrors: nil,
		},
		{
			name: "file in subdirectory with translation",
			files: map[string]string{
				"crds/sub/my-crd.yaml":        "apiVersion: apiextensions.k8s.io/v1",
				"crds/sub/doc-ru-my-crd.yaml": "apiVersion: apiextensions.k8s.io/v1",
			},
			checkFile:  "crds/sub/my-crd.yaml",
			wantErrors: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			for name, content := range tt.files {
				fullPath := filepath.Join(dir, name)
				err := os.MkdirAll(filepath.Dir(fullPath), 0o755)
				require.NoError(t, err)
				err = os.WriteFile(fullPath, []byte(content), 0o600)
				require.NoError(t, err)
			}

			cfg := &pkg.OpenAPILinterConfig{}
			rule := NewBilingualRule(cfg, dir)
			errorList := errors.NewLintRuleErrorsList()

			filePath := filepath.Join(dir, tt.checkFile)
			rule.Run(filePath, errorList)

			errs := errorList.GetErrors()
			if tt.wantErrors == nil {
				assert.Empty(t, errs)
			} else {
				assert.Len(t, errs, len(tt.wantErrors))
				for i, err := range errs {
					assert.Contains(t, err.Text, tt.wantErrors[i])
					assert.Equal(t, pkg.Error, err.Level)
				}
			}
		})
	}
}

func TestBilingualRuleRespectsMaxLevel(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "my-crd.yaml")
	require.NoError(t, os.WriteFile(filePath, []byte("apiVersion: apiextensions.k8s.io/v1"), 0o600))

	rule := NewBilingualRule(&pkg.OpenAPILinterConfig{}, dir)
	warnLevel := pkg.Warn
	errorList := errors.NewLintRuleErrorsList().WithMaxLevel(&warnLevel)

	rule.Run(filePath, errorList)

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Equal(t, pkg.Warn, errs[0].Level)
	assert.Contains(t, errs[0].Text, "translation file is missing")
}
