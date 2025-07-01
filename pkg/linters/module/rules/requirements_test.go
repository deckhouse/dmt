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
				"requirements: Stage usage requires minimum Deckhouse version, deckhouse version range should start no lower than 1.68.0 (currently: 1.67.0)",
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
	assert.Equal(t, "1.71.0", MinimalDeckhouseVersionForReadinessProbes)
}

func TestRequirementsRegistry_AllChecks(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name           string
		moduleContent  string
		setupFiles     func(string) error
		expectedErrors []string
	}{
		{
			name: "stage without requirements",
			moduleContent: `name: test-module
namespace: test
stage: "General Availability"`,
			setupFiles: func(path string) error {
				return os.WriteFile(filepath.Join(path, ModuleConfigFilename), []byte(`name: test-module
namespace: test
stage: "General Availability"`), 0600)
			},
			expectedErrors: []string{"stage should be used with requirements: deckhouse >= 1.68.0"},
		},
		{
			name: "go hooks without requirements",
			moduleContent: `name: test-module
namespace: test`,
			setupFiles: func(path string) error {
				if err := os.WriteFile(filepath.Join(path, ModuleConfigFilename), []byte(`name: test-module
namespace: test`), 0600); err != nil {
					return err
				}
				hooksDir := filepath.Join(path, "hooks")
				if err := os.MkdirAll(hooksDir, 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(hooksDir, "go.mod"), []byte(`module test`), 0600); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(hooksDir, "main.go"), []byte("package main\nfunc main() { app.Run() }"), 0600)
			},
			expectedErrors: []string{"requirements: for using go_hook, deckhouse version constraint must be specified (minimum: 1.68.0)"},
		},
		{
			name: "readiness probe + module-sdk >= 0.3 without requirements",
			moduleContent: `name: test-module
namespace: test`,
			setupFiles: func(path string) error {
				if err := os.WriteFile(filepath.Join(path, ModuleConfigFilename), []byte(`name: test-module
namespace: test`), 0600); err != nil {
					return err
				}
				hooksDir := filepath.Join(path, "hooks")
				if err := os.MkdirAll(hooksDir, 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(hooksDir, "go.mod"), []byte(`module test
require github.com/deckhouse/module-sdk v0.3.0`), 0600); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(hooksDir, "main.go"), []byte("package main\nfunc main() { app.WithReadiness() }"), 0600)
			},
			expectedErrors: []string{"requirements: for using readiness probes, deckhouse version constraint must be specified (minimum: 1.71.0)"},
		},
		{
			name: "module-sdk >= 0.3 without app.WithReadiness and without requirements",
			moduleContent: `name: test-module
namespace: test`,
			setupFiles: func(path string) error {
				if err := os.WriteFile(filepath.Join(path, ModuleConfigFilename), []byte(`name: test-module
namespace: test`), 0600); err != nil {
					return err
				}
				hooksDir := filepath.Join(path, "hooks")
				if err := os.MkdirAll(hooksDir, 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(hooksDir, "go.mod"), []byte(`module test
require github.com/deckhouse/module-sdk v0.3.0`), 0600); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(hooksDir, "main.go"), []byte("package main\nfunc main() { }"), 0600)
			},
			expectedErrors: []string{"requirements: for using module-sdk >= 0.3, deckhouse version constraint must be specified (minimum: 1.71.0)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modulePath := filepath.Join(tempDir, tt.name)
			if err := os.MkdirAll(modulePath, 0755); err != nil {
				t.Fatalf("failed to create module dir: %v", err)
			}
			if tt.setupFiles != nil {
				err := tt.setupFiles(modulePath)
				require.NoError(t, err)
			}
			rule := NewRequirementsRule()
			errorList := errors.NewLintRuleErrorsList()
			rule.CheckRequirements(modulePath, errorList)
			if len(tt.expectedErrors) == 0 {
				assert.False(t, errorList.ContainsErrors(), "Expected no errors but got: %v", errorList.GetErrors())
			} else {
				assert.True(t, errorList.ContainsErrors(), "Expected errors but got none")
				errs := errorList.GetErrors()
				for _, expectedError := range tt.expectedErrors {
					found := false
					for _, err := range errs {
						if assert.Contains(t, err.Text, expectedError) {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected error containing '%s'", expectedError)
				}
			}
		})
	}
}

// Add separate tests for each check
func TestRequirementsRegistry_StageCheck(t *testing.T) {
	tempDir := t.TempDir()
	modulePath := filepath.Join(tempDir, "test-stage")
	if err := os.MkdirAll(modulePath, 0755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}

	// Create module.yaml only with stage, without go files
	if err := os.WriteFile(filepath.Join(modulePath, ModuleConfigFilename), []byte(`name: test-module
namespace: test
stage: "General Availability"`), 0600); err != nil {
		t.Fatalf("failed to create module.yaml: %v", err)
	}

	rule := NewRequirementsRule()
	errorList := errors.NewLintRuleErrorsList()
	rule.CheckRequirements(modulePath, errorList)

	assert.True(t, errorList.ContainsErrors(), "Expected errors but got none")
	errs := errorList.GetErrors()
	assert.Len(t, errs, 1, "Expected 1 error, got %d", len(errs))
	assert.Contains(t, errs[0].Text, "stage should be used with requirements: deckhouse >= 1.68.0")
}

func TestRequirementsRegistry_ReadinessProbesCheck(t *testing.T) {
	tempDir := t.TempDir()
	modulePath := filepath.Join(tempDir, "test-readiness")
	if err := os.MkdirAll(modulePath, 0755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}

	// Create module.yaml without stage
	if err := os.WriteFile(filepath.Join(modulePath, ModuleConfigFilename), []byte(`name: test-module
namespace: test`), 0600); err != nil {
		t.Fatalf("failed to create module.yaml: %v", err)
	}

	// Create go.mod with module-sdk >= 0.3 and go file with app.WithReadiness
	hooksDir := filepath.Join(modulePath, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("failed to create hooks dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(hooksDir, "go.mod"), []byte(`module test
require github.com/deckhouse/module-sdk v0.3.0`), 0600); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	if err := os.WriteFile(filepath.Join(hooksDir, "main.go"), []byte("package main\nfunc main() { app.WithReadiness() }"), 0600); err != nil {
		t.Fatalf("failed to create main.go: %v", err)
	}

	rule := NewRequirementsRule()
	errorList := errors.NewLintRuleErrorsList()
	rule.CheckRequirements(modulePath, errorList)

	assert.True(t, errorList.ContainsErrors(), "Expected errors but got none")
	errs := errorList.GetErrors()
	assert.Len(t, errs, 1, "Expected 1 error, got %d", len(errs))
	assert.Contains(t, errs[0].Text, "requirements: for using readiness probes, deckhouse version constraint must be specified (minimum: 1.71.0)")
}

func TestRequirementsRegistry_ModuleSDKCheck(t *testing.T) {
	tempDir := t.TempDir()
	modulePath := filepath.Join(tempDir, "test-module-sdk")
	if err := os.MkdirAll(modulePath, 0755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}

	// Create module.yaml without stage
	if err := os.WriteFile(filepath.Join(modulePath, ModuleConfigFilename), []byte(`name: test-module
namespace: test`), 0600); err != nil {
		t.Fatalf("failed to create module.yaml: %v", err)
	}

	// Create go.mod with module-sdk >= 0.3, but without app.WithReadiness
	hooksDir := filepath.Join(modulePath, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("failed to create hooks dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(hooksDir, "go.mod"), []byte(`module test
require github.com/deckhouse/module-sdk v0.3.0`), 0600); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	if err := os.WriteFile(filepath.Join(hooksDir, "main.go"), []byte("package main\nfunc main() { }"), 0600); err != nil {
		t.Fatalf("failed to create main.go: %v", err)
	}

	rule := NewRequirementsRule()
	errorList := errors.NewLintRuleErrorsList()
	rule.CheckRequirements(modulePath, errorList)

	assert.True(t, errorList.ContainsErrors(), "Expected errors but got none")
	errs := errorList.GetErrors()
	assert.Len(t, errs, 1, "Expected 1 error, got %d", len(errs))
	assert.Contains(t, errs[0].Text, "requirements: for using module-sdk >= 0.3, deckhouse version constraint must be specified (minimum: 1.71.0)")
}

func TestHasAppRunCalls(t *testing.T) {
	tempDir := t.TempDir()
	modulePath := filepath.Join(tempDir, "test-app-run")
	if err := os.MkdirAll(modulePath, 0755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}

	hooksDir := filepath.Join(modulePath, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("failed to create hooks dir: %v", err)
	}

	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "app.Run() call",
			content:  "package main\nfunc main() { app.Run() }",
			expected: true,
		},
		{
			name:     "myApp.Run() call",
			content:  "package main\nfunc main() { myApp.Run() }",
			expected: true,
		},
		{
			name:     "hookApp.Run() call",
			content:  "package main\nfunc main() { hookApp.Run() }",
			expected: true,
		},
		{
			name:     "no Run() call",
			content:  "package main\nfunc main() { }",
			expected: false,
		},
		{
			name:     "app.WithReadiness() call",
			content:  "package main\nfunc main() { app.WithReadiness() }",
			expected: false,
		},
		{
			name:     "Run() without dot",
			content:  "package main\nfunc main() { Run() }",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up previous test file
			testFile := filepath.Join(hooksDir, "main.go")
			if err := os.WriteFile(testFile, []byte(tt.content), 0600); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			result := hasAppRunCalls(modulePath)
			assert.Equal(t, tt.expected, result, "Expected %v for content: %s", tt.expected, tt.content)
		})
	}
}
