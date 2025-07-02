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

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestNewRequirementsRuleBasic(t *testing.T) {
	tests := []struct {
		name     string
		expected *RequirementsRule
	}{
		{
			name: "create rule",
			expected: &RequirementsRule{
				RuleMeta: pkg.RuleMeta{
					Name: RequirementsRuleName,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewRequirementsRule()
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, RequirementsRuleName, result.GetName())
		})
	}
}

func TestFindMinimalAllowedVersion(t *testing.T) {
	tests := []struct {
		name           string
		constraintStr  string
		expectedResult string // empty string means nil result
	}{
		{
			name:           "simple >= constraint",
			constraintStr:  ">= 1.68.0",
			expectedResult: "1.68.0",
		},
		{
			name:           "simple > constraint",
			constraintStr:  "> 1.68.0",
			expectedResult: "1.68.0",
		},
		{
			name:           "simple = constraint",
			constraintStr:  "= 1.68.0",
			expectedResult: "1.68.0",
		},
		{
			name:           "range constraint with >= and <",
			constraintStr:  ">= 1.68.0, < 2.0.0",
			expectedResult: "1.68.0",
		},
		{
			name:           "range constraint with > and <=",
			constraintStr:  "> 1.67.0, <= 2.0.0",
			expectedResult: "1.67.0",
		},
		{
			name:           "multiple ranges with OR",
			constraintStr:  ">= 1.67.0 || >= 1.68.0",
			expectedResult: "1.67.0",
		},
		{
			name:           "multiple ranges with AND",
			constraintStr:  ">= 1.68.0, >= 1.69.0",
			expectedResult: "1.68.0",
		},
		{
			name:           "constraint with only <",
			constraintStr:  "< 2.0.0",
			expectedResult: "",
		},
		{
			name:           "constraint with only <=",
			constraintStr:  "<= 2.0.0",
			expectedResult: "",
		},
		{
			name:           "constraint with only != (not supported)",
			constraintStr:  "!= 1.68.0",
			expectedResult: "1.68.0",
		},
		{
			name:           "complex constraint with multiple operators",
			constraintStr:  ">= 1.67.0, < 2.0.0, > 1.66.0",
			expectedResult: "1.66.0",
		},
		{
			name:           "constraint with version prefix v",
			constraintStr:  ">= v1.68.0",
			expectedResult: "1.68.0",
		},
		{
			name:           "empty constraint",
			constraintStr:  "",
			expectedResult: "",
		},
		{
			name:           "constraint with invalid version format",
			constraintStr:  ">= invalid-version",
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var constraint *semver.Constraints

			if tt.constraintStr != "" {
				constraint, _ = semver.NewConstraint(tt.constraintStr)
			}

			result := findMinimalAllowedVersion(constraint)

			if tt.expectedResult == "" {
				assert.Nil(t, result, "Expected nil result for constraint: %s", tt.constraintStr)
			} else {
				assert.NotNil(t, result, "Expected non-nil result for constraint: %s", tt.constraintStr)
				assert.Equal(t, tt.expectedResult, result.String(), "Expected version %s, got %s", tt.expectedResult, result.String())
			}
		})
	}
}

func TestGetDeckhouseModule(t *testing.T) {
	helper := NewTestHelper(t)

	tests := []struct {
		name           string
		modulePath     string
		expectedErrors []string
		expectedModule *DeckhouseModule
		setupFiles     func(string) error
	}{
		{
			name:       "module.yaml does not exist",
			modulePath: helper.CreateTempModule("no-module"),
			setupFiles: func(_ string) error {
				// Don't create any files
				return nil
			},
			expectedErrors: []string{},
			expectedModule: nil,
		},
		{
			name:       "module.yaml exists but is invalid yaml",
			modulePath: helper.CreateTempModule("invalid-yaml"),
			setupFiles: func(path string) error {
				content := `invalid: yaml: content: [`
				return os.WriteFile(filepath.Join(path, ModuleConfigFilename), []byte(content), 0600)
			},
			expectedErrors: []string{
				"Cannot parse file \"module.yaml\"",
			},
			expectedModule: nil,
		},
		{
			name:       "module.yaml exists with valid content",
			modulePath: helper.CreateTempModule("valid-module"),
			setupFiles: func(path string) error {
				content := `name: test-module
namespace: test
stage: "General Availability"
requirements:
  deckhouse: ">= 1.68.0"`
				return os.WriteFile(filepath.Join(path, ModuleConfigFilename), []byte(content), 0600)
			},
			expectedErrors: []string{},
			expectedModule: &DeckhouseModule{
				Name:      "test-module",
				Namespace: "test",
				Stage:     "General Availability",
				Requirements: &ModuleRequirements{
					ModulePlatformRequirements: ModulePlatformRequirements{
						Deckhouse: ">= 1.68.0",
					},
				},
			},
		},
		{
			name:       "module.yaml exists with minimal content",
			modulePath: helper.CreateTempModule("minimal-module"),
			setupFiles: func(path string) error {
				content := `name: test-module`
				return os.WriteFile(filepath.Join(path, ModuleConfigFilename), []byte(content), 0600)
			},
			expectedErrors: []string{},
			expectedModule: &DeckhouseModule{
				Name: "test-module",
			},
		},
		{
			name:       "module.yaml exists but is not readable",
			modulePath: helper.CreateTempModule("unreadable-module"),
			setupFiles: func(path string) error {
				// Create file with no read permissions
				filePath := filepath.Join(path, ModuleConfigFilename)
				file, err := os.Create(filePath)
				if err != nil {
					return err
				}
				file.Close()
				return os.Chmod(filePath, 0000)
			},
			expectedErrors: []string{
				"Cannot read file \"module.yaml\"",
			},
			expectedModule: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test files
			if tt.setupFiles != nil {
				err := tt.setupFiles(tt.modulePath)
				require.NoError(t, err)
			}

			// Create error list
			errorList := errors.NewLintRuleErrorsList()

			// Run the function
			result, _ := getDeckhouseModule(tt.modulePath, errorList)

			// Verify results
			helper.AssertErrors(errorList, tt.expectedErrors)

			// Check returned module
			if tt.expectedModule == nil {
				assert.Nil(t, result, "Expected nil module")
			} else {
				assert.NotNil(t, result, "Expected non-nil module")
				assert.Equal(t, tt.expectedModule.Name, result.Name)
				assert.Equal(t, tt.expectedModule.Namespace, result.Namespace)
				assert.Equal(t, tt.expectedModule.Stage, result.Stage)
				if tt.expectedModule.Requirements != nil {
					assert.NotNil(t, result.Requirements)
					assert.Equal(t, tt.expectedModule.Requirements.Deckhouse, result.Requirements.Deckhouse)
				}
			}
		})
	}
}

// Test constants
func TestConstantsBasic(t *testing.T) {
	assert.Equal(t, "requirements", RequirementsRuleName)
	assert.Equal(t, "1.68.0", MinimalDeckhouseVersionForStage)
	assert.Equal(t, "1.71.0", MinimalDeckhouseVersionForReadinessProbes)
}
