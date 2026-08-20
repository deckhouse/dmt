/*
Copyright 2025 Flant JSC

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

func TestNewEnabledScriptRule(t *testing.T) {
	rule := NewEnabledScriptRule(nil, errors.NewLintRuleErrorsList())
	assert.Equal(t, EnabledScriptRuleName, rule.GetName())
}

func TestEnabledScriptRule_CheckEnabledScript(t *testing.T) {
	tests := []struct {
		name           string
		createFile     bool
		fileContent    string
		expectedErrors []string
	}{
		{
			name:           "no enabled file",
			createFile:     false,
			expectedErrors: []string{},
		},
		{
			name:           "empty enabled file",
			createFile:     true,
			fileContent:    "",
			expectedErrors: []string{},
		},
		{
			name:           "whitespace-only enabled file",
			createFile:     true,
			fileContent:    "\n\n  \t\n",
			expectedErrors: []string{},
		},
		{
			name:        "non-empty enabled file",
			createFile:  true,
			fileContent: "#!/bin/bash\necho true\n",
			expectedErrors: []string{
				"The enabled-script mechanism is deprecated and must be removed.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			if tt.createFile {
				enabledPath := filepath.Join(tempDir, "enabled")
				err := os.WriteFile(enabledPath, []byte(tt.fileContent), 0600)
				require.NoError(t, err)
			}

			errorList := errors.NewLintRuleErrorsList()
			rule := NewEnabledScriptRule(moduleAt(t, tempDir), errorList)

			rule.Check(t.Context())

			errs := errorList.GetErrors()

			errTexts := make([]string, 0, len(errs))
			for _, e := range errs {
				errTexts = append(errTexts, e.Text)
			}

			assert.Len(t, errTexts, len(tt.expectedErrors), "Expected %d errors, got %d: %v", len(tt.expectedErrors), len(errTexts), errTexts)

			for _, expectedError := range tt.expectedErrors {
				found := false

				for _, errText := range errTexts {
					if strings.Contains(errText, expectedError) {
						found = true
						break
					}
				}

				assert.True(t, found, "Expected error containing %q not found in %v", expectedError, errTexts)
			}

			// A deprecation notice must always be a warning, never an error.
			for _, e := range errs {
				assert.Equal(t, pkg.Warn, e.Level, "enabled-script finding must be a warning")
			}
		})
	}
}
