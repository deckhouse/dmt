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
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg/errors"
)

func TestValidateComponentRequirementEdgeCases(t *testing.T) {
	helper := NewTestHelper(t)
	registry := NewRequirementsRegistry()

	tests := []struct {
		name           string
		checkName      string
		req            ComponentRequirement
		module         *DeckhouseModule
		expectedErrors []string
		description    string
	}{
		{
			name:      "kubernetes component without requirements",
			checkName: "test_check",
			req: ComponentRequirement{
				ComponentType: ComponentK8s,
				MinVersion:    "1.20.0",
				Description:   "Kubernetes version required",
			},
			module: &DeckhouseModule{
				Name:      "test-module",
				Namespace: "test",
			},
			expectedErrors: []string{"requirements [test_check]: Kubernetes version required, kubernetes version constraint is required"},
			description:    "Should fail when kubernetes component is required but not specified",
		},
		{
			name:      "kubernetes component with empty requirements",
			checkName: "test_check",
			req: ComponentRequirement{
				ComponentType: ComponentK8s,
				MinVersion:    "1.20.0",
				Description:   "Kubernetes version required",
			},
			module: &DeckhouseModule{
				Name:         "test-module",
				Namespace:    "test",
				Requirements: &ModuleRequirements{},
			},
			expectedErrors: []string{"requirements [test_check]: Kubernetes version required, kubernetes version constraint is required"},
			description:    "Should fail when kubernetes component is required but empty",
		},
		{
			name:      "unknown component type",
			checkName: "test_check",
			req: ComponentRequirement{
				ComponentType: "unknown",
				MinVersion:    "1.20.0",
				Description:   "Unknown component required",
			},
			module: &DeckhouseModule{
				Name:      "test-module",
				Namespace: "test",
			},
			expectedErrors: []string{"requirements [test_check]: unknown component type unknown"},
			description:    "Should fail when unknown component type is specified",
		},
		{
			name:      "module component type (placeholder)",
			checkName: "test_check",
			req: ComponentRequirement{
				ComponentType: ComponentModule,
				MinVersion:    "1.20.0",
				Description:   "Module version required",
			},
			module: &DeckhouseModule{
				Name:      "test-module",
				Namespace: "test",
			},
			expectedErrors: []string{},
			description:    "Should pass for module component type (placeholder implementation)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			errorList := errors.NewLintRuleErrorsList()
			registry.validateComponentRequirement(tt.checkName, tt.req, tt.module, errorList)
			helper.AssertErrors(errorList, tt.expectedErrors)
		})
	}
}

func TestFindGoModFilesWithModuleSDKEdgeCases(t *testing.T) {
	helper := NewTestHelper(t)

	tests := []struct {
		name        string
		modulePath  string
		minVersion  string
		setup       func(string) error
		expected    int // expected number of valid go.mod directories
		description string
	}{
		{
			name:        "hooks directory does not exist",
			modulePath:  helper.CreateTempModule("no-hooks"),
			minVersion:  "0.1.0",
			setup:       func(_ string) error { return nil },
			expected:    0,
			description: "Should return empty slice when hooks directory does not exist",
		},
		{
			name:       "hooks directory exists but no go.mod files",
			modulePath: helper.CreateTempModule("empty-hooks"),
			minVersion: "0.1.0",
			setup: func(path string) error {
				hooksDir := filepath.Join(path, "hooks")
				return os.MkdirAll(hooksDir, 0755)
			},
			expected:    0,
			description: "Should return empty slice when hooks directory exists but no go.mod files",
		},
		{
			name:       "go.mod exists but no module-sdk dependency",
			modulePath: helper.CreateTempModule("no-module-sdk"),
			minVersion: "0.1.0",
			setup: func(path string) error {
				helper.SetupGoHooks(path, "module test", "")
				return nil
			},
			expected:    0,
			description: "Should return empty slice when go.mod exists but no module-sdk dependency",
		},
		{
			name:       "go.mod with module-sdk but version below minimum",
			modulePath: helper.CreateTempModule("old-module-sdk"),
			minVersion: "0.3.0",
			setup: func(path string) error {
				helper.SetupGoHooks(path, GoModWithModuleSDK, "")
				return nil
			},
			expected:    0,
			description: "Should return empty slice when module-sdk version is below minimum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			if tt.setup != nil {
				err := tt.setup(tt.modulePath)
				require.NoError(t, err)
			}

			result := findGoModFilesWithModuleSDK(tt.modulePath, tt.minVersion)
			assert.Len(t, result, tt.expected, tt.description)
		})
	}
}

func TestHasModuleSDKDependencyEdgeCases(t *testing.T) {
	helper := NewTestHelper(t)
	tempDir := helper.CreateTempModule("test-module-sdk")

	tests := []struct {
		name        string
		goModPath   string
		minVersion  string
		setup       func() error
		expected    bool
		description string
	}{
		{
			name:        "go.mod file does not exist",
			goModPath:   filepath.Join(tempDir, "nonexistent", "go.mod"),
			minVersion:  "0.1.0",
			setup:       func() error { return nil },
			expected:    false,
			description: "Should return false when go.mod file does not exist",
		},
		{
			name:       "go.mod with invalid syntax",
			goModPath:  filepath.Join(tempDir, "invalid.go.mod"),
			minVersion: "0.1.0",
			setup: func() error {
				content := `invalid go.mod content`
				return os.WriteFile(filepath.Join(tempDir, "invalid.go.mod"), []byte(content), 0600)
			},
			expected:    false,
			description: "Should return false when go.mod has invalid syntax",
		},
		{
			name:       "go.mod with module-sdk but invalid version",
			goModPath:  filepath.Join(tempDir, "invalid-version.go.mod"),
			minVersion: "0.1.0",
			setup: func() error {
				content := `module test
require github.com/deckhouse/module-sdk invalid-version`
				return os.WriteFile(filepath.Join(tempDir, "invalid-version.go.mod"), []byte(content), 0600)
			},
			expected:    false,
			description: "Should return false when module-sdk has invalid version format",
		},
		{
			name:       "go.mod with module-sdk but empty version",
			goModPath:  filepath.Join(tempDir, "empty-version.go.mod"),
			minVersion: "0.1.0",
			setup: func() error {
				content := `module test
require github.com/deckhouse/module-sdk`
				return os.WriteFile(filepath.Join(tempDir, "empty-version.go.mod"), []byte(content), 0600)
			},
			expected:    false,
			description: "Should return false when module-sdk has empty version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			if tt.setup != nil {
				err := tt.setup()
				require.NoError(t, err)
			}

			result := hasModuleSDKDependency(tt.goModPath, tt.minVersion)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

func TestFindPatternInGoFilesEdgeCases(t *testing.T) {
	helper := NewTestHelper(t)
	tempDir := helper.CreateTempModule("pattern-edge-cases")

	tests := []struct {
		name        string
		dirs        []string
		pattern     *regexp.Regexp
		setup       func(string) error
		expected    bool
		description string
	}{
		{
			name:        "empty directories list",
			dirs:        []string{},
			pattern:     regexp.MustCompile(`app\.Run`),
			setup:       func(_ string) error { return nil },
			expected:    false,
			description: "Should return false when directories list is empty",
		},
		{
			name:        "directory does not exist",
			dirs:        []string{filepath.Join(tempDir, "nonexistent")},
			pattern:     regexp.MustCompile(`app\.Run`),
			setup:       func(_ string) error { return nil },
			expected:    false,
			description: "Should return false when directory does not exist",
		},
		{
			name:    "directory exists but no .go files",
			dirs:    []string{filepath.Join(tempDir, "no-go-files")},
			pattern: regexp.MustCompile(`app\.Run`),
			setup: func(base string) error {
				dir := filepath.Join(base, "no-go-files")
				if err := os.MkdirAll(dir, 0755); err != nil {
					return err
				}
				content := `package main`
				return os.WriteFile(filepath.Join(dir, "main.txt"), []byte(content), 0600)
			},
			expected:    false,
			description: "Should return false when directory exists but no .go files",
		},
		{
			name:    "go file exists but unreadable",
			dirs:    []string{filepath.Join(tempDir, "unreadable-go")},
			pattern: regexp.MustCompile(`app\.Run`),
			setup: func(base string) error {
				dir := filepath.Join(base, "unreadable-go")
				if err := os.MkdirAll(dir, 0755); err != nil {
					return err
				}
				content := `package main\nfunc main() { app.Run() }`
				goFile := filepath.Join(dir, "main.go")
				if err := os.WriteFile(goFile, []byte(content), 0600); err != nil {
					return err
				}
				return os.Chmod(goFile, 0000)
			},
			expected:    false,
			description: "Should return false when .go file exists but is unreadable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			if tt.setup != nil {
				err := tt.setup(tempDir)
				require.NoError(t, err)
			}

			result := findPatternInGoFiles(tt.dirs, tt.pattern)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

func TestGetDeckhouseModuleEdgeCases(t *testing.T) {
	helper := NewTestHelper(t)

	tests := []struct {
		name           string
		modulePath     string
		setup          func(string) error
		expectedErrors []string
		expectedModule *DeckhouseModule
		description    string
	}{
		{
			name:       "module.yaml exists but is not readable",
			modulePath: helper.CreateTempModule("unreadable-module"),
			setup: func(path string) error {
				content := `name: test-module\nnamespace: test`
				moduleFile := filepath.Join(path, ModuleConfigFilename)
				err := os.WriteFile(moduleFile, []byte(content), 0600)
				if err != nil {
					return err
				}
				// Make file unreadable
				return os.Chmod(moduleFile, 0000)
			},
			expectedErrors: []string{"Cannot read file \"module.yaml\""},
			expectedModule: nil,
			description:    "Should return error when module.yaml exists but is not readable",
		},
		{
			name:       "module.yaml exists but is invalid yaml",
			modulePath: helper.CreateTempModule("invalid-yaml"),
			setup: func(path string) error {
				content := `invalid: yaml: content: [`
				return os.WriteFile(filepath.Join(path, ModuleConfigFilename), []byte(content), 0600)
			},
			expectedErrors: []string{"Cannot parse file \"module.yaml\""},
			expectedModule: nil,
			description:    "Should return error when module.yaml has invalid yaml syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			if tt.setup != nil {
				err := tt.setup(tt.modulePath)
				require.NoError(t, err)
			}

			errorList := errors.NewLintRuleErrorsList()
			result, _ := getDeckhouseModule(tt.modulePath, errorList)

			helper.AssertErrors(errorList, tt.expectedErrors)

			assert.Equal(t, tt.expectedModule, result, tt.description)
		})
	}
}

func TestRequirementsRegistryEdgeCases(t *testing.T) {
	helper := NewTestHelper(t)

	tests := []struct {
		name           string
		modulePath     string
		module         *DeckhouseModule
		expectedErrors []string
		description    string
	}{
		{
			name:           "nil module with stage check",
			modulePath:     helper.CreateTempModule("nil-module"),
			module:         nil,
			expectedErrors: []string{"requirements [stage]: Stage usage requires minimum Deckhouse version, module is not defined"},
			description:    "Should return error when module is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			registry := NewRequirementsRegistry()
			errorList := errors.NewLintRuleErrorsList()

			// Create a custom check that will trigger with nil module
			customCheck := RequirementCheck{
				Name: "stage",
				Requirements: []ComponentRequirement{
					{
						ComponentType: ComponentDeckhouse,
						MinVersion:    MinimalDeckhouseVersionForStage,
						Description:   "Stage usage requires minimum Deckhouse version",
					},
				},
				Description: "Stage usage requires minimum Deckhouse version",
				Detector: func(_ string, _ *DeckhouseModule) bool {
					return true // Always trigger
				},
			}

			registry.RegisterCheck(customCheck)
			registry.RunAllChecks(tt.modulePath, tt.module, errorList)

			helper.AssertErrors(errorList, tt.expectedErrors)
		})
	}
}

func TestHasOptionalModules(t *testing.T) {
	tests := []struct {
		name        string
		module      *DeckhouseModule
		expected    bool
		description string
	}{
		{
			name:        "nil module",
			module:      nil,
			expected:    false,
			description: "Should return false when module is nil",
		},
		{
			name: "module without requirements",
			module: &DeckhouseModule{
				Name:      "test-module",
				Namespace: "test",
			},
			expected:    false,
			description: "Should return false when module has no requirements",
		},
		{
			name: "module with nil requirements",
			module: &DeckhouseModule{
				Name:         "test-module",
				Namespace:    "test",
				Requirements: nil,
			},
			expected:    false,
			description: "Should return false when requirements is nil",
		},
		{
			name: "module with empty ParentModules",
			module: &DeckhouseModule{
				Name:      "test-module",
				Namespace: "test",
				Requirements: &ModuleRequirements{
					ParentModules: map[string]string{},
				},
			},
			expected:    false,
			description: "Should return false when ParentModules is empty",
		},
		{
			name: "module with non-optional dependencies",
			module: &DeckhouseModule{
				Name:      "test-module",
				Namespace: "test",
				Requirements: &ModuleRequirements{
					ParentModules: map[string]string{
						"module1": ">= 1.0.0",
						"module2": "~1.2.0",
					},
				},
			},
			expected:    false,
			description: "Should return false when all dependencies are non-optional",
		},
		{
			name: "module with optional dependency",
			module: &DeckhouseModule{
				Name:      "test-module",
				Namespace: "test",
				Requirements: &ModuleRequirements{
					ParentModules: map[string]string{
						"module1": ">= 1.0.0 !optional",
					},
				},
			},
			expected:    true,
			description: "Should return true when module has optional dependency",
		},
		{
			name: "module with mixed dependencies",
			module: &DeckhouseModule{
				Name:      "test-module",
				Namespace: "test",
				Requirements: &ModuleRequirements{
					ParentModules: map[string]string{
						"module1": ">= 1.0.0",
						"module2": ">= 1.2.0 !optional",
						"module3": "~1.3.0",
					},
				},
			},
			expected:    true,
			description: "Should return true when module has at least one optional dependency",
		},
		{
			name: "module with multiple optional dependencies",
			module: &DeckhouseModule{
				Name:      "test-module",
				Namespace: "test",
				Requirements: &ModuleRequirements{
					ParentModules: map[string]string{
						"module1": ">= 1.0.0 !optional",
						"module2": ">= 1.2.0 !optional",
					},
				},
			},
			expected:    true,
			description: "Should return true when module has multiple optional dependencies",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasOptionalModules(tt.module)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

func TestOptionalModulesRequirementCheck(t *testing.T) {
	helper := NewTestHelper(t)

	tests := []struct {
		name           string
		module         *DeckhouseModule
		expectedErrors []string
		description    string
	}{
		{
			name: "optional modules with sufficient deckhouse version",
			module: &DeckhouseModule{
				Name:      "test-module",
				Namespace: "test",
				Requirements: &ModuleRequirements{
					ModulePlatformRequirements: ModulePlatformRequirements{
						Deckhouse: ">= 1.73.0",
					},
					ParentModules: map[string]string{
						"module1": ">= 1.0.0 !optional",
					},
				},
			},
			expectedErrors: []string{},
			description:    "Should pass when deckhouse version is >= 1.73.0 with optional modules",
		},
		{
			name: "optional modules with insufficient deckhouse version",
			module: &DeckhouseModule{
				Name:      "test-module",
				Namespace: "test",
				Requirements: &ModuleRequirements{
					ModulePlatformRequirements: ModulePlatformRequirements{
						Deckhouse: ">= 1.72.0",
					},
					ParentModules: map[string]string{
						"module1": ">= 1.0.0 !optional",
					},
				},
			},
			expectedErrors: []string{"requirements [optional_modules]: Optional modules usage requires minimum Deckhouse version, deckhouse version range should start no lower than 1.73.0"},
			description:    "Should fail when deckhouse version is < 1.73.0 with optional modules",
		},
		{
			name: "optional modules without deckhouse version specified",
			module: &DeckhouseModule{
				Name:      "test-module",
				Namespace: "test",
				Requirements: &ModuleRequirements{
					ParentModules: map[string]string{
						"module1": ">= 1.0.0 !optional",
					},
				},
			},
			expectedErrors: []string{"requirements [optional_modules]: Optional modules usage requires minimum Deckhouse version, deckhouse version range should start no lower than 1.73.0"},
			description:    "Should fail when deckhouse version is not specified with optional modules",
		},
		{
			name: "no optional modules with old deckhouse version",
			module: &DeckhouseModule{
				Name:      "test-module",
				Namespace: "test",
				Requirements: &ModuleRequirements{
					ModulePlatformRequirements: ModulePlatformRequirements{
						Deckhouse: ">= 1.70.0",
					},
					ParentModules: map[string]string{
						"module1": ">= 1.0.0",
					},
				},
			},
			expectedErrors: []string{},
			description:    "Should pass when no optional modules are used regardless of deckhouse version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRequirementsRegistry()
			errorList := errors.NewLintRuleErrorsList()
			registry.RunAllChecks("", tt.module, errorList)
			helper.AssertErrors(errorList, tt.expectedErrors)
		})
	}
}
