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
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestNewNoLangKeyRule(t *testing.T) {
	rule := NewNoLangKeyRule()
	assert.Equal(t, NoLangKeyRuleName, rule.GetName())
	assert.Equal(t, "no-lang-key", rule.GetName())
}

func TestExtractFrontMatter(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "valid front matter",
			content: "---\ntitle: Test\nlang: ru\n---\nBody text",
			want:    "title: Test\nlang: ru",
		},
		{
			name:    "no front matter at all",
			content: "Just some text\nwithout front matter",
			want:    "",
		},
		{
			name:    "only opening delimiter",
			content: "---\ntitle: Test\nlang: ru",
			want:    "",
		},
		{
			name:    "empty front matter",
			content: "---\n---\nBody text",
			want:    "",
		},
		{
			name:    "delimiter with trailing spaces",
			content: "---   \ntitle: Test\n---   \nBody text",
			want:    "title: Test",
		},
		{
			name:    "content after front matter is not included",
			content: "---\nkey: value\n---\nlang: ru\nMore body text",
			want:    "key: value",
		},
		{
			name:    "multiple delimiters only first pair used",
			content: "---\nfirst: block\n---\n---\nsecond: block\n---",
			want:    "first: block",
		},
		{
			name:    "empty content",
			content: "",
			want:    "",
		},
		{
			name:    "single delimiter line",
			content: "---",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFrontMatter(tt.content)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFindLangKeyLine(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{
			name:    "lang on line 2",
			content: "---\nlang: ru\n---",
			want:    2,
		},
		{
			name:    "lang on line 3",
			content: "---\ntitle: Test\nlang: en\n---",
			want:    3,
		},
		{
			name:    "no lang key",
			content: "---\ntitle: Test\n---",
			want:    0,
		},
		{
			name:    "language key is not lang",
			content: "---\nlanguage: ru\n---",
			want:    0,
		},
		{
			name:    "lang without trailing space does not match",
			content: "---\nlang:ru\n---",
			want:    0,
		},
		{
			name:    "lang with multiple spaces",
			content: "---\nlang:   en\n---",
			want:    2,
		},
		{
			name:    "lang with tab",
			content: "---\nlang:\tru\n---",
			want:    2,
		},
		{
			name:    "indented lang key does not match",
			content: "---\n  lang: ru\n---",
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findLangKeyLine(tt.content)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNoLangKeyRule_CheckFiles(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string // relative to module root
		wantErrors int
		wantTexts  []string
	}{
		{
			name: "file without front matter - no errors",
			files: map[string]string{
				"docs/README.md": "# Hello\nJust text without front matter",
			},
			wantErrors: 0,
		},
		{
			name: "file with front matter but no lang key - no errors",
			files: map[string]string{
				"docs/CONFIGURATION.md": "---\ntitle: Configuration\n---\nSome content",
			},
			wantErrors: 0,
		},
		{
			name: "file with lang ru in front matter - error",
			files: map[string]string{
				"docs/CONFIGURATION.md": "---\ntitle: Test\nlang: ru\n---\nContent",
			},
			wantErrors: 1,
			wantTexts:  []string{"'lang' key"},
		},
		{
			name: "file with lang en in front matter - error",
			files: map[string]string{
				"docs/README.md": "---\nlang: en\ntitle: Test\n---\nContent",
			},
			wantErrors: 1,
			wantTexts:  []string{"'lang' key"},
		},
		{
			name: "file in subdirectory docs/sub/ is skipped",
			files: map[string]string{
				"docs/sub/NESTED.md": "---\nlang: ru\n---\nContent",
			},
			wantErrors: 0,
		},
		{
			name: "multiple files - only those with lang key produce errors",
			files: map[string]string{
				"docs/GOOD.md":     "---\ntitle: Good\n---\nContent",
				"docs/BAD.md":      "---\nlang: ru\ntitle: Bad\n---\nContent",
				"docs/ALSO_BAD.md": "---\nlang: en\n---\nContent",
			},
			wantErrors: 2,
			wantTexts:  []string{"'lang' key", "'lang' key"},
		},
		{
			name: "non-md file in docs is ignored",
			files: map[string]string{
				"docs/image.png": "binary content",
			},
			wantErrors: 0,
		},
		{
			name: "lang key outside front matter is not detected",
			files: map[string]string{
				"docs/SAFE.md": "---\ntitle: Safe\n---\nlang: ru\nBody text with lang key",
			},
			wantErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := minimock.NewController(t)
			mockModule := mocks.NewModuleMock(mc)

			tempDir := t.TempDir()
			mockModule.GetPathMock.Return(tempDir)

			for relPath, content := range tt.files {
				fullPath := filepath.Join(tempDir, relPath)
				require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
				require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o600))
			}

			rule := NewNoLangKeyRule()
			errorList := errors.NewLintRuleErrorsList()

			rule.CheckFiles(mockModule, errorList)

			errs := errorList.GetErrors()
			assert.Len(t, errs, tt.wantErrors)

			for i, wantText := range tt.wantTexts {
				if i < len(errs) {
					assert.Contains(t, errs[i].Text, wantText)
				}
			}
		})
	}
}

func TestNoLangKeyRule_CheckFiles_EmptyModulePath(t *testing.T) {
	mc := minimock.NewController(t)
	mockModule := mocks.NewModuleMock(mc)
	mockModule.GetPathMock.Return("")

	rule := NewNoLangKeyRule()
	errorList := errors.NewLintRuleErrorsList()

	rule.CheckFiles(mockModule, errorList)

	errs := errorList.GetErrors()
	assert.Empty(t, errs)
}

func TestNoLangKeyRule_CheckFiles_Excluded(t *testing.T) {
	mc := minimock.NewController(t)
	mockModule := mocks.NewModuleMock(mc)

	tempDir := t.TempDir()
	mockModule.GetPathMock.Return(tempDir)

	// Create a file with lang: key in front matter
	docsDir := filepath.Join(tempDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(docsDir, "EXCLUDED.md"),
		[]byte("---\nlang: ru\ntitle: Excluded\n---\nContent"),
		0o600,
	))

	rule := NewNoLangKeyRule()
	rule.ExcludeStringRules = []pkg.StringRuleExclude{
		pkg.StringRuleExclude("docs/EXCLUDED.md"),
	}
	errorList := errors.NewLintRuleErrorsList()

	rule.CheckFiles(mockModule, errorList)

	errs := errorList.GetErrors()
	assert.Empty(t, errs, "Expected excluded file to produce no errors")
}

func TestNoLangKeyRule_CheckFiles_ErrorLineNumber(t *testing.T) {
	mc := minimock.NewController(t)
	mockModule := mocks.NewModuleMock(mc)

	tempDir := t.TempDir()
	mockModule.GetPathMock.Return(tempDir)

	docsDir := filepath.Join(tempDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(docsDir, "TEST.md"),
		[]byte("---\ntitle: Test\nlang: ru\nweight: 10\n---\nContent"),
		0o600,
	))

	rule := NewNoLangKeyRule()
	errorList := errors.NewLintRuleErrorsList()

	rule.CheckFiles(mockModule, errorList)

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Text, "'lang' key")
	// The value should contain the line number information
	assert.Contains(t, errs[0].ObjectValue.(string), "Line 3")
}
