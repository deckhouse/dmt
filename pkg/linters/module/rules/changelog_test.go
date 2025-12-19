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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg/errors"
)

func TestChangelogRule_CheckChangelog(t *testing.T) {
	tests := []struct {
		name         string
		fileContent  string
		createFile   bool
		expectErrors int
	}{
		{
			name:         "changelog.yaml file missing",
			createFile:   false,
			expectErrors: 1,
		},
		{
			name:         "changelog.yaml file empty",
			fileContent:  "",
			createFile:   true,
			expectErrors: 1,
		},
		{
			name:         "changelog.yaml file present and non-empty",
			fileContent:  "# Changelog\n\n## v1.0.0\n- Initial release",
			createFile:   true,
			expectErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			if tt.createFile {
				err := os.WriteFile(filepath.Join(tempDir, changelogFilename), []byte(tt.fileContent), 0600)
				require.NoError(t, err)
			}

			rule := NewChangelogRule()
			errorList := errors.NewLintRuleErrorsList()

			rule.CheckChangelog(tempDir, errorList)

			if tt.expectErrors > 0 {
				assert.True(t, errorList.ContainsErrors(), "Expected errors but got none")
				assert.Len(t, errorList.GetErrors(), tt.expectErrors)
			} else {
				assert.False(t, errorList.ContainsErrors(), "Expected no errors but got some")
			}
		})
	}
}
