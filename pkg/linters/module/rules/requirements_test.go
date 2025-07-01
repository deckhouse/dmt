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

func TestNewRequirementsRule(t *testing.T) {
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

func TestRequirementsRule_CheckRequirements(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name           string
		modulePath     string
		moduleContent  string
		expectedErrors []string
		setupFiles     func(string) error
	}{
		{
			name:       "module.yaml does not exist",
			modulePath: tempDir,
			setupFiles: func(_ string) error {
				// Don't create any files
				return nil
			},
			expectedErrors: []string{},
		},
		{
			name:       "module.yaml exists but is invalid yaml",
			modulePath: tempDir,
			setupFiles: func(path string) error {
				content := `invalid: yaml: content: [`
				return os.WriteFile(filepath.Join(path, ModuleConfigFilename), []byte(content), 0600)
			},
			expectedErrors: []string{
				"Cannot parse file \"module.yaml\"",
			},
		},
		{
			name:       "module.yaml exists with valid content but no stage",
			modulePath: tempDir,
			setupFiles: func(path string) error {
				content := `name: test-module
namespace: test`
				return os.WriteFile(filepath.Join(path, ModuleConfigFilename), []byte(content), 0600)
			},
			expectedErrors: []string{},
		},
		{
			name:       "module.yaml with stage but no requirements",
			modulePath: tempDir,
			setupFiles: func(path string) error {
				content := `name: test-module
namespace: test
stage: "General Availability"`
				return os.WriteFile(filepath.Join(path, ModuleConfigFilename), []byte(content), 0600)
			},
			expectedErrors: []string{
				"stage should be used with requirements: deckhouse >= 1.68.0",
			},
		},
		{
			name:       "module.yaml with stage and valid requirements",
			modulePath: tempDir,
			setupFiles: func(path string) error {
				content := `name: test-module
namespace: test
stage: "General Availability"
requirements:
  deckhouse: ">= 1.68.0"`
				return os.WriteFile(filepath.Join(path, ModuleConfigFilename), []byte(content), 0600)
			},
			expectedErrors: []string{},
		},
		{
			name:       "module.yaml with stage and invalid deckhouse constraint",
			modulePath: tempDir,
			setupFiles: func(path string) error {
				content := `name: test-module
namespace: test
stage: "General Availability"
requirements:
  deckhouse: "invalid-constraint"`
				return os.WriteFile(filepath.Join(path, ModuleConfigFilename), []byte(content), 0600)
			},
			expectedErrors: []string{
				"invalid deckhouse version constraint: invalid-constraint",
			},
		},
		{
			name:       "module.yaml with stage and requirements below minimum",
			modulePath: tempDir,
			setupFiles: func(path string) error {
				content := `name: test-module
namespace: test
stage: "General Availability"
requirements:
  deckhouse: ">= 1.67.0"`
				return os.WriteFile(filepath.Join(path, ModuleConfigFilename), []byte(content), 0600)
			},
			expectedErrors: []string{
				"requirements: deckhouse range should start no lower than 1.68.0 (currently: 1.67.0)",
			},
		},
		{
			name:       "module.yaml with stage and complex valid constraint",
			modulePath: tempDir,
			setupFiles: func(path string) error {
				content := `name: test-module
namespace: test
stage: "General Availability"
requirements:
  deckhouse: ">= 1.68.0, < 2.0.0"`
				return os.WriteFile(filepath.Join(path, ModuleConfigFilename), []byte(content), 0600)
			},
			expectedErrors: []string{},
		},
		{
			name:       "module.yaml with stage and exact version constraint",
			modulePath: tempDir,
			setupFiles: func(path string) error {
				content := `name: test-module
namespace: test
stage: "General Availability"
requirements:
  deckhouse: "= 1.68.0"`
				return os.WriteFile(filepath.Join(path, ModuleConfigFilename), []byte(content), 0600)
			},
			expectedErrors: []string{},
		},
		{
			name:       "module.yaml with stage and greater than constraint",
			modulePath: tempDir,
			setupFiles: func(path string) error {
				content := `name: test-module
namespace: test
stage: "General Availability"
requirements:
  deckhouse: "> 1.68.0"`
				return os.WriteFile(filepath.Join(path, ModuleConfigFilename), []byte(content), 0600)
			},
			expectedErrors: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test files
			if tt.setupFiles != nil {
				err := tt.setupFiles(tt.modulePath)
				require.NoError(t, err)
			}

			// Create rule and error list
			rule := NewRequirementsRule()
			errorList := errors.NewLintRuleErrorsList()

			// Run the check
			rule.CheckRequirements(tt.modulePath, errorList)

			// Verify results
			if len(tt.expectedErrors) == 0 {
				assert.False(t, errorList.ContainsErrors(), "Expected no errors but got: %v", errorList.GetErrors())
			} else {
				assert.True(t, errorList.ContainsErrors(), "Expected errors but got none")
				errs := errorList.GetErrors()
				assert.Len(t, errs, len(tt.expectedErrors), "Expected %d errors, got %d", len(tt.expectedErrors), len(errs))

				for i, expectedError := range tt.expectedErrors {
					if i < len(errs) {
						assert.Contains(t, errs[i].Text, expectedError, "Error %d should contain '%s'", i, expectedError)
					}
				}
			}
		})
	}
}

func Test_checkStage(t *testing.T) {
	tests := []struct {
		name           string
		module         *DeckhouseModule
		expectedErrors []string
	}{
		{
			name:           "nil module",
			module:         nil,
			expectedErrors: []string{},
		},
		{
			name: "module with empty stage",
			module: &DeckhouseModule{
				Name:  "test-module",
				Stage: "",
			},
			expectedErrors: []string{},
		},
		{
			name: "module with stage but nil requirements",
			module: &DeckhouseModule{
				Name:  "test-module",
				Stage: "General Availability",
			},
			expectedErrors: []string{
				"stage should be used with requirements: deckhouse >= 1.68.0",
			},
		},
		{
			name: "module with stage but empty deckhouse requirement",
			module: &DeckhouseModule{
				Name:  "test-module",
				Stage: "General Availability",
				Requirements: &ModuleRequirements{
					ModulePlatformRequirements: ModulePlatformRequirements{
						Deckhouse: "",
					},
				},
			},
			expectedErrors: []string{
				"stage should be used with requirements: deckhouse >= 1.68.0",
			},
		},
		{
			name: "module with stage and valid deckhouse requirement",
			module: &DeckhouseModule{
				Name:  "test-module",
				Stage: "General Availability",
				Requirements: &ModuleRequirements{
					ModulePlatformRequirements: ModulePlatformRequirements{
						Deckhouse: ">= 1.68.0",
					},
				},
			},
			expectedErrors: []string{},
		},
		{
			name: "module with stage and invalid deckhouse constraint",
			module: &DeckhouseModule{
				Name:  "test-module",
				Stage: "General Availability",
				Requirements: &ModuleRequirements{
					ModulePlatformRequirements: ModulePlatformRequirements{
						Deckhouse: "invalid-constraint",
					},
				},
			},
			expectedErrors: []string{
				"invalid deckhouse version constraint: invalid-constraint",
			},
		},
		{
			name: "module with stage and requirement below minimum",
			module: &DeckhouseModule{
				Name:  "test-module",
				Stage: "General Availability",
				Requirements: &ModuleRequirements{
					ModulePlatformRequirements: ModulePlatformRequirements{
						Deckhouse: ">= 1.67.0",
					},
				},
			},
			expectedErrors: []string{
				"requirements: deckhouse range should start no lower than 1.68.0 (currently: 1.67.0)",
			},
		},
		{
			name: "module with stage and exact minimum version",
			module: &DeckhouseModule{
				Name:  "test-module",
				Stage: "General Availability",
				Requirements: &ModuleRequirements{
					ModulePlatformRequirements: ModulePlatformRequirements{
						Deckhouse: "= 1.68.0",
					},
				},
			},
			expectedErrors: []string{},
		},
		{
			name: "module with stage and greater than minimum",
			module: &DeckhouseModule{
				Name:  "test-module",
				Stage: "General Availability",
				Requirements: &ModuleRequirements{
					ModulePlatformRequirements: ModulePlatformRequirements{
						Deckhouse: "> 1.68.0",
					},
				},
			},
			expectedErrors: []string{},
		},
		{
			name: "module with stage and complex valid constraint",
			module: &DeckhouseModule{
				Name:  "test-module",
				Stage: "General Availability",
				Requirements: &ModuleRequirements{
					ModulePlatformRequirements: ModulePlatformRequirements{
						Deckhouse: ">= 1.68.0, < 2.0.0",
					},
				},
			},
			expectedErrors: []string{},
		},
		{
			name: "module with stage and complex constraint with lower bound below minimum",
			module: &DeckhouseModule{
				Name:  "test-module",
				Stage: "General Availability",
				Requirements: &ModuleRequirements{
					ModulePlatformRequirements: ModulePlatformRequirements{
						Deckhouse: ">= 1.67.0, < 2.0.0",
					},
				},
			},
			expectedErrors: []string{
				"requirements: deckhouse range should start no lower than 1.68.0 (currently: 1.67.0)",
			},
		},
		{
			name: "module with stage and multiple ranges with one below minimum",
			module: &DeckhouseModule{
				Name:  "test-module",
				Stage: "General Availability",
				Requirements: &ModuleRequirements{
					ModulePlatformRequirements: ModulePlatformRequirements{
						Deckhouse: ">= 1.67.0 || >= 1.68.0",
					},
				},
			},
			expectedErrors: []string{
				"requirements: deckhouse range should start no lower than 1.68.0 (currently: 1.67.0)",
			},
		},
		{
			name: "module with stage and tilde constraint",
			module: &DeckhouseModule{
				Name:  "test-module",
				Stage: "General Availability",
				Requirements: &ModuleRequirements{
					ModulePlatformRequirements: ModulePlatformRequirements{
						Deckhouse: "~1.68.0",
					},
				},
			},
			expectedErrors: []string{},
		},
		{
			name: "module with stage and caret constraint",
			module: &DeckhouseModule{
				Name:  "test-module",
				Stage: "General Availability",
				Requirements: &ModuleRequirements{
					ModulePlatformRequirements: ModulePlatformRequirements{
						Deckhouse: "^1.68.0",
					},
				},
			},
			expectedErrors: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorList := errors.NewLintRuleErrorsList()
			checkStage(tt.module, errorList)

			if len(tt.expectedErrors) == 0 {
				assert.False(t, errorList.ContainsErrors(), "Expected no errors but got: %v", errorList.GetErrors())
			} else {
				assert.True(t, errorList.ContainsErrors(), "Expected errors but got none")
				errs := errorList.GetErrors()
				assert.Len(t, errs, len(tt.expectedErrors), "Expected %d errors, got %d", len(tt.expectedErrors), len(errs))

				for i, expectedError := range tt.expectedErrors {
					if i < len(errs) {
						assert.Contains(t, errs[i].Text, expectedError, "Error %d should contain '%s'", i, expectedError)
					}
				}
			}
		})
	}
}

func Test_findMinimalAllowedVersion(t *testing.T) {
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
			expectedResult: "1.69.0",
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
			expectedResult: "",
		},
		{
			name:           "complex constraint with multiple operators",
			constraintStr:  ">= 1.67.0, < 2.0.0, > 1.66.0",
			expectedResult: "1.67.0",
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

func Test_getDeckhouseModule(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name           string
		modulePath     string
		moduleContent  string
		expectedErrors []string
		expectedModule *DeckhouseModule
		setupFiles     func(string) error
	}{
		{
			name:       "module.yaml does not exist",
			modulePath: tempDir,
			setupFiles: func(_ string) error {
				// Don't create any files
				return nil
			},
			expectedErrors: []string{},
			expectedModule: nil,
		},
		{
			name:       "module.yaml exists but is invalid yaml",
			modulePath: tempDir,
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
			modulePath: tempDir,
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
			modulePath: tempDir,
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
			modulePath: tempDir,
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
			if len(tt.expectedErrors) == 0 {
				assert.False(t, errorList.ContainsErrors(), "Expected no errors but got: %v", errorList.GetErrors())
			} else {
				assert.True(t, errorList.ContainsErrors(), "Expected errors but got none")
				errs := errorList.GetErrors()
				assert.Len(t, errs, len(tt.expectedErrors), "Expected %d errors, got %d", len(tt.expectedErrors), len(errs))

				for i, expectedError := range tt.expectedErrors {
					if i < len(errs) {
						assert.Contains(t, errs[i].Text, expectedError, "Error %d should contain '%s'", i, expectedError)
					}
				}
			}

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
func TestConstants(t *testing.T) {
	assert.Equal(t, "requirements", RequirementsRuleName)
	assert.Equal(t, "1.68.0", MinimalDeckhouseVersionForStage)
}
