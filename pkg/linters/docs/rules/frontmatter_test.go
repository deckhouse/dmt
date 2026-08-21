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

const (
	validFrontMatter = "---\ntitle: \"OK\"\ndescription: fine\n---\n\n# Heading\n\nbody\n"
	// starts with a delimiter but the block is never closed → EOF-style breakage.
	unterminatedFrontMatter = "---\ntitle: \"broken\"\ndescription: never closed\n\n# Heading\n"
	// terminated block whose YAML is malformed (unterminated quote).
	invalidYAMLFrontMatter = "---\ntitle: \"unterminated\n---\n\n# Heading\n"
	// a "---" thematic break in the body, not on the first line → not front matter.
	thematicBreakBody = "# Heading\n\nsome text\n\n---\n\nmore text\n"
	// valid YAML between the delimiters, but a scalar / sequence — not a mapping.
	scalarFrontMatter   = "---\njust a plain sentence\n---\n\n# Heading\n"
	sequenceFrontMatter = "---\n- a\n- b\n---\n\n# Heading\n"
)

func TestFrontMatterRule_CheckFiles(t *testing.T) {
	tests := []struct {
		name         string
		files        map[string]string // relative to module root
		wantFindings int
		wantContains string
		wantLevel    pkg.Level
	}{
		{
			name:         "valid front matter",
			files:        map[string]string{"docs/README.md": validFrontMatter},
			wantFindings: 0,
		},
		{
			name:         "no front matter",
			files:        map[string]string{"docs/README.md": "# Heading\n\nbody\n"},
			wantFindings: 0,
		},
		{
			name:         "thematic break in body is not front matter",
			files:        map[string]string{"docs/README.md": thematicBreakBody},
			wantFindings: 0,
		},
		{
			name:         "unterminated front matter is an error",
			files:        map[string]string{"docs/README.md": unterminatedFrontMatter},
			wantFindings: 1,
			wantContains: "unterminated YAML front matter",
			wantLevel:    pkg.Error,
		},
		{
			name:         "invalid yaml in front matter is an error",
			files:        map[string]string{"docs/README.md": invalidYAMLFrontMatter},
			wantFindings: 1,
			wantContains: "invalid YAML front matter",
			wantLevel:    pkg.Error,
		},
		{
			name:         "scalar between delimiters is a warning",
			files:        map[string]string{"docs/README.md": scalarFrontMatter},
			wantFindings: 1,
			wantContains: "must be a YAML mapping",
			wantLevel:    pkg.Warn,
		},
		{
			name:         "sequence between delimiters is a warning",
			files:        map[string]string{"docs/README.md": sequenceFrontMatter},
			wantFindings: 1,
			wantContains: "must be a YAML mapping",
			wantLevel:    pkg.Warn,
		},
		{
			name:         "broken front matter in a non-internal subfolder is flagged",
			files:        map[string]string{"docs/guide/page.md": unterminatedFrontMatter},
			wantFindings: 1,
			wantContains: "unterminated YAML front matter",
			wantLevel:    pkg.Error,
		},
		{
			name:         "broken front matter under docs/internal is skipped",
			files:        map[string]string{"docs/internal/notes.md": unterminatedFrontMatter},
			wantFindings: 0,
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

			rule := NewFrontMatterRule()
			errorList := errors.NewLintRuleErrorsList()

			rule.CheckFiles(mockModule, errorList)

			errs := errorList.GetErrors()
			assert.Len(t, errs, tt.wantFindings)

			if tt.wantContains != "" {
				require.NotEmpty(t, errs)
				assert.Contains(t, errs[0].Text, tt.wantContains)
				assert.Equal(t, tt.wantLevel, errs[0].Level)
			}
		})
	}
}
