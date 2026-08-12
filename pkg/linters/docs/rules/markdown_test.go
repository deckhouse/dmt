/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

func TestNewMarkdownRule(t *testing.T) {
	rule := NewMarkdownRule(nil, errors.NewLintRuleErrorsList())
	assert.Equal(t, MarkdownlintRuleName, rule.GetName())
	assert.Equal(t, "markdownlint", rule.GetName())
}

// violatingMarkdown has no top-level heading on the first line (MD041) and no
// trailing newline (MD047), so markdownlint reports at least one finding for it.
const violatingMarkdown = "not a heading"

func TestMarkdownRule_CheckFiles_SkipsSubfolders(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string // relative to module root
		wantAny    bool              // expect at least one markdownlint finding
		wantNoPath string            // no finding may reference this path fragment
	}{
		{
			name: "top-level docs file is linted",
			files: map[string]string{
				"docs/BAD.md": violatingMarkdown,
			},
			wantAny: true,
		},
		{
			name: "file in docs subfolder is skipped",
			files: map[string]string{
				"docs/internal/BAD.md": violatingMarkdown,
			},
			wantAny: false,
		},
		{
			name: "file in nested docs subfolder is skipped",
			files: map[string]string{
				"docs/internal/deep/BAD.md": violatingMarkdown,
			},
			wantAny: false,
		},
		{
			name: "only the top-level file is reported when both exist",
			files: map[string]string{
				"docs/BAD.md":          violatingMarkdown,
				"docs/internal/BAD.md": violatingMarkdown,
			},
			wantAny:    true,
			wantNoPath: "internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := minimock.NewController(t)
			mockModule := mocks.NewModuleMock(mc)

			tempDir := t.TempDir()
			mockModule.GetPathMock.Return(tempDir)
			mockModule.GetNameMock.Return("test-module")

			for relPath, content := range tt.files {
				fullPath := filepath.Join(tempDir, relPath)
				require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
				require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o600))
			}

			errorList := errors.NewLintRuleErrorsList()
			rule := NewMarkdownRule(mockModule, errorList)

			rule.Check(t.Context())

			errs := errorList.GetErrors()
			if tt.wantAny {
				assert.NotEmpty(t, errs, "expected at least one markdownlint finding for a top-level docs file")
			} else {
				assert.Empty(t, errs, "expected no markdownlint findings when violations live only in docs subfolders")
			}

			if tt.wantNoPath != "" {
				for _, e := range errs {
					assert.NotContains(t, e.FilePath, tt.wantNoPath,
						"markdownlint should not report findings from a skipped docs subfolder")
				}
			}
		})
	}
}
