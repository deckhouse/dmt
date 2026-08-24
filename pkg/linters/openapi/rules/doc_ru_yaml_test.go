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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestDocRuYAMLRule_Run(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "valid mapping",
			content: "type: object\nproperties:\n  foo:\n    type: string\n",
			wantErr: false,
		},
		{
			name:    "empty file is valid",
			content: "",
			wantErr: false,
		},
		{
			name:    "sequence root is rejected (not a mapping)",
			content: "- a\n- b\n",
			wantErr: true,
		},
		{
			name:    "scalar root is rejected (not a mapping)",
			content: "just a string\n",
			wantErr: true,
		},
		{
			name:    "unterminated quote",
			content: "properties:\n  foo:\n    description: 'unterminated\n",
			wantErr: true,
		},
		{
			name:    "tab indentation is invalid yaml",
			content: "properties:\n\tfoo: bar\n",
			wantErr: true,
		},
		{
			name:    "duplicate key is rejected by strict parsing",
			content: "type: object\ntype: string\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			filePath := filepath.Join(dir, "doc-ru-config-values.yaml")
			require.NoError(t, os.WriteFile(filePath, []byte(tt.content), 0o600))

			errorList := errors.NewLintRuleErrorsList()
			rule := NewDocRuYAMLRule(&pkg.OpenAPILinterConfig{}, moduleAt(t, dir), errorList)

			rule.checkFile(filePath)

			errs := errorList.GetErrors()
			if !tt.wantErr {
				assert.Empty(t, errs)

				return
			}

			require.Len(t, errs, 1)
			assert.Equal(t, pkg.Error, errs[0].Level)
			assert.Contains(t, errs[0].Text, "doc-ru file is not valid YAML")
			assert.Equal(t, "doc-ru-config-values.yaml", errs[0].FilePath)
		})
	}
}

func TestDocRuYAMLRule_RespectsMaxLevel(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "doc-ru-config-values.yaml")
	require.NoError(t, os.WriteFile(filePath, []byte("a: 'b\n"), 0o600))

	warnLevel := pkg.Warn
	errorList := errors.NewLintRuleErrorsList().WithMaxLevel(&warnLevel)
	rule := NewDocRuYAMLRule(&pkg.OpenAPILinterConfig{}, moduleAt(t, dir), errorList)

	rule.checkFile(filePath)

	errs := errorList.GetErrors()
	require.Len(t, errs, 1)
	assert.Equal(t, pkg.Warn, errs[0].Level)
	assert.True(t, strings.Contains(errs[0].Text, "not valid YAML"))
}
