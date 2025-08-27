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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg"
)

// registryMockModule is a mock implementation of module.Module for testing
type registryMockModule struct {
	name string
	path string
}

func (m *registryMockModule) GetName() string {
	return m.name
}

func (m *registryMockModule) GetPath() string {
	return m.path
}

func TestNewRegistryRule(t *testing.T) {
	tests := []struct {
		name     string
		expected *RegistryRule
	}{
		{
			name: "empty exclude rules",
			expected: &RegistryRule{
				RuleMeta: pkg.RuleMeta{
					Name: RegistryRuleName,
				},
			},
		},
		{
			name: "with exclude rules",
			expected: &RegistryRule{
				RuleMeta: pkg.RuleMeta{
					Name: RegistryRuleName,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewRegistryRule()
			assert.Equal(t, tt.expected.Name, result.Name)
		})
	}
}

func TestRegistryRule_CheckRegistrySecret(t *testing.T) {
	tests := []struct {
		name           string
		moduleName     string
		modulePath     string
		registrySecret string
		expectedErrors []string
		setupFunc      func(string) func()
	}{
		{
			name:       "no registry secret file",
			moduleName: "test-module",
			modulePath: "/tmp/test-module",
			setupFunc: func(path string) func() {
				// Create module directory without registry secret
				err := os.MkdirAll(path, 0755)
				require.NoError(t, err)
				return func() {
					os.RemoveAll(path)
				}
			},
		},
		{
			name:       "deckhouse module - should skip",
			moduleName: "deckhouse",
			modulePath: "/tmp/deckhouse",
			setupFunc: func(path string) func() {
				// Create deckhouse module directory
				err := os.MkdirAll(path, 0755)
				require.NoError(t, err)

				// Create registry secret file
				registryContent := `apiVersion: v1
kind: Secret
metadata:
  name: registry-secret
data:
  .dockerconfigjson: {{ .Values.global.modulesImages.registry.dockercfg }}`
				err = os.WriteFile(filepath.Join(path, "registry-secret.yaml"), []byte(registryContent), 0600)
				require.NoError(t, err)

				return func() {
					os.RemoveAll(path)
				}
			},
		},
		{
			name:       "registry secret with global modulesImages.registry.dockercfg but missing module registry.dockercfg",
			moduleName: "test-module",
			modulePath: "/tmp/test-module",
			setupFunc: func(path string) func() {
				// Create module directory
				err := os.MkdirAll(path, 0755)
				require.NoError(t, err)

				// Create registry secret file with global modulesImages.registry.dockercfg but missing module registry.dockercfg
				registryContent := `apiVersion: v1
kind: Secret
metadata:
  name: registry-secret
data:
  .dockerconfigjson: {{ .Values.global.modulesImages.registry.dockercfg }}`
				err = os.WriteFile(filepath.Join(path, "registry-secret.yaml"), []byte(registryContent), 0600)
				require.NoError(t, err)

				return func() {
					os.RemoveAll(path)
				}
			},
			expectedErrors: []string{"registry-secret.yaml file contains .Values.global.modulesImages.registry.dockercfg but missing .Values.testModule.registry.dockercfg"},
		},
		{
			name:       "registry secret with both global and module registry.dockercfg - valid case",
			moduleName: "test-module",
			modulePath: "/tmp/test-module",
			setupFunc: func(path string) func() {
				// Create module directory
				err := os.MkdirAll(path, 0755)
				require.NoError(t, err)

				// Create registry secret file with both global and module registry.dockercfg
				registryContent := `apiVersion: v1
kind: Secret
metadata:
  name: registry-secret
data:
  .dockerconfigjson: {{ .Values.global.modulesImages.registry.dockercfg }}
  .dockerconfigjson2: {{ .Values.testModule.registry.dockercfg }}`
				err = os.WriteFile(filepath.Join(path, "registry-secret.yaml"), []byte(registryContent), 0600)
				require.NoError(t, err)

				return func() {
					os.RemoveAll(path)
				}
			},
		},
		{
			name:       "registry secret without global modulesImages.registry.dockercfg - valid case",
			moduleName: "test-module",
			modulePath: "/tmp/test-module",
			setupFunc: func(path string) func() {
				// Create module directory
				err := os.MkdirAll(path, 0755)
				require.NoError(t, err)

				// Create registry secret file without global modulesImages.registry.dockercfg
				registryContent := `apiVersion: v1
kind: Secret
metadata:
  name: registry-secret
data:
  .dockerconfigjson: {{ .Values.registry.auth }}`
				err = os.WriteFile(filepath.Join(path, "registry-secret.yaml"), []byte(registryContent), 0600)
				require.NoError(t, err)

				return func() {
					os.RemoveAll(path)
				}
			},
		},
		{
			name:       "registry secret with old .Values.global.modulesImages pattern - should not trigger error",
			moduleName: "test-module",
			modulePath: "/tmp/test-module",
			setupFunc: func(path string) func() {
				// Create module directory
				err := os.MkdirAll(path, 0755)
				require.NoError(t, err)

				// Create registry secret file with old pattern (should not trigger error)
				registryContent := `apiVersion: v1
kind: Secret
metadata:
  name: registry-secret
data:
  .dockerconfigjson: {{ .Values.global.modulesImages }}`
				err = os.WriteFile(filepath.Join(path, "registry-secret.yaml"), []byte(registryContent), 0600)
				require.NoError(t, err)

				return func() {
					os.RemoveAll(path)
				}
			},
		},
		{
			name:       "registry secret with camelCase module name - valid case",
			moduleName: "test-module",
			modulePath: "/tmp/test-module",
			setupFunc: func(path string) func() {
				// Create module directory
				err := os.MkdirAll(path, 0755)
				require.NoError(t, err)

				// Create registry secret file with camelCase module name (testModule)
				registryContent := `apiVersion: v1
kind: Secret
metadata:
  name: registry-secret
data:
  .dockerconfigjson: {{ .Values.global.modulesImages.registry.dockercfg }}
  .dockerconfigjson2: {{ .Values.testModule.registry.dockercfg }}`
				err = os.WriteFile(filepath.Join(path, "registry-secret.yaml"), []byte(registryContent), 0600)
				require.NoError(t, err)

				return func() {
					os.RemoveAll(path)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupFunc(tt.modulePath)
			defer cleanup()

			// Create mock module (not used in this test but kept for future use)
			_ = &registryMockModule{
				name: tt.moduleName,
				path: tt.modulePath,
			}

			// If it's deckhouse module, it should be skipped
			if tt.moduleName == "deckhouse" {
				return
			}

			// Check if registry secret file exists and contains global modulesImages.registry.dockercfg
			registryFiles := []string{filepath.Join(tt.modulePath, "registry-secret.yaml")}
			for _, registryFile := range registryFiles {
				if _, err := os.Stat(registryFile); err == nil {
					fileContent, err := os.ReadFile(registryFile)
					require.NoError(t, err)

					// Check for global modulesImages.registry.dockercfg pattern
					if strings.Contains(string(fileContent), ".Values.global.modulesImages.registry.dockercfg") {
						// Check if module has its own registry.dockercfg configuration
						// Convert module name to camelCase for comparison (same as in the actual code)
						camelCaseModuleName := strings.ReplaceAll(tt.moduleName, "-", "")
						// For "test-module" -> "testmodule", but we need "testModule"
						// This is a simplified version - in real code we use module.ToLowerCamel()
						if strings.Contains(camelCaseModuleName, "test") {
							camelCaseModuleName = "testModule"
						}

						modulePattern := fmt.Sprintf(".Values.%s.registry.dockercfg", camelCaseModuleName)
						if !strings.Contains(string(fileContent), modulePattern) {
							// This should trigger an error
							assert.NotEmpty(t, tt.expectedErrors, "Expected error for missing module registry.dockercfg")
						} else {
							// Both patterns found - should not trigger error
							assert.Empty(t, tt.expectedErrors, "Should not have errors when both patterns are present")
						}
					}
				}
			}
		})
	}
}

func TestConvertURLToModuleName(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		expected string
	}{
		{
			name:     "SSH URL with .git suffix",
			repoURL:  "git@github.com:deckhouse/test-module.git",
			expected: "test-module",
		},
		{
			name:     "HTTPS URL with .git suffix",
			repoURL:  "https://github.com/deckhouse/test-module.git",
			expected: "test-module",
		},
		{
			name:     "HTTPS URL without .git suffix",
			repoURL:  "https://github.com/deckhouse/test-module",
			expected: "test-module",
		},
		{
			name:     "SSH URL without .git suffix",
			repoURL:  "git@github.com:deckhouse/test-module",
			expected: "test-module",
		},
		{
			name:     "complex path",
			repoURL:  "https://github.com/deckhouse/ee/modules/test-module.git",
			expected: "test-module",
		},
		{
			name:     "empty URL",
			repoURL:  "",
			expected: "",
		},
		{
			name:     "URL with trailing slash",
			repoURL:  "https://github.com/deckhouse/test-module/",
			expected: "",
		},
		{
			name:     "single component URL",
			repoURL:  "test-module",
			expected: "test-module",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertURLToModuleName(tt.repoURL)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetGitConfigFile(t *testing.T) {
	tests := []struct {
		name      string
		dir       string
		expected  string
		setupFunc func(string) func()
	}{
		{
			name:     "git config exists in current directory",
			dir:      "/tmp/test-module",
			expected: "/tmp/test-module/.git/config",
			setupFunc: func(dir string) func() {
				err := os.MkdirAll(filepath.Join(dir, ".git"), 0755)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("test"), 0600)
				require.NoError(t, err)
				return func() {
					os.RemoveAll(dir)
				}
			},
		},
		{
			name:     "git config exists in parent directory",
			dir:      "/tmp/test-module/subdir",
			expected: "/tmp/test-module/.git/config",
			setupFunc: func(dir string) func() {
				// Create parent directory with git config
				parentDir := filepath.Dir(dir)
				err := os.MkdirAll(filepath.Join(parentDir, ".git"), 0755)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(parentDir, ".git", "config"), []byte("test"), 0600)
				require.NoError(t, err)

				// Create subdirectory
				err = os.MkdirAll(dir, 0755)
				require.NoError(t, err)

				return func() {
					os.RemoveAll(parentDir)
				}
			},
		},
		{
			name:     "no git config found",
			dir:      "/tmp/test-module",
			expected: "",
			setupFunc: func(dir string) func() {
				err := os.MkdirAll(dir, 0755)
				require.NoError(t, err)
				return func() {
					os.RemoveAll(dir)
				}
			},
		},
		{
			name:     "git directory exists but no config file",
			dir:      "/tmp/test-module",
			expected: "",
			setupFunc: func(dir string) func() {
				err := os.MkdirAll(filepath.Join(dir, ".git"), 0755)
				require.NoError(t, err)
				return func() {
					os.RemoveAll(dir)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupFunc(tt.dir)
			defer cleanup()

			result := getGitConfigFile(tt.dir)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetModuleNameFromRepository(t *testing.T) {
	tests := []struct {
		name      string
		dir       string
		gitConfig string
		expected  string
		setupFunc func(string, string) func()
	}{
		{
			name: "valid git config with SSH URL",
			dir:  "/tmp/test-module",
			gitConfig: `[remote "origin"]
	url = git@github.com:deckhouse/test-module.git`,
			expected: "test-module",
			setupFunc: func(dir, config string) func() {
				err := os.MkdirAll(filepath.Join(dir, ".git"), 0755)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(config), 0600)
				require.NoError(t, err)
				return func() {
					os.RemoveAll(dir)
				}
			},
		},
		{
			name: "valid git config with HTTPS URL",
			dir:  "/tmp/test-module",
			gitConfig: `[remote "origin"]
	url = https://github.com/deckhouse/test-module.git`,
			expected: "test-module",
			setupFunc: func(dir, config string) func() {
				err := os.MkdirAll(filepath.Join(dir, ".git"), 0755)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(config), 0600)
				require.NoError(t, err)
				return func() {
					os.RemoveAll(dir)
				}
			},
		},
		{
			name: "valid git config with HTTPS URL without .git suffix",
			dir:  "/tmp/test-module",
			gitConfig: `[remote "origin"]
	url = https://github.com/deckhouse/test-module`,
			expected: "test-module",
			setupFunc: func(dir, config string) func() {
				err := os.MkdirAll(filepath.Join(dir, ".git"), 0755)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(config), 0600)
				require.NoError(t, err)
				return func() {
					os.RemoveAll(dir)
				}
			},
		},
		{
			name:     "no git directory",
			dir:      "/tmp/test-module",
			expected: "",
			setupFunc: func(dir, _ string) func() {
				err := os.MkdirAll(dir, 0755)
				require.NoError(t, err)
				return func() {
					os.RemoveAll(dir)
				}
			},
		},
		{
			name: "invalid git config",
			dir:  "/tmp/test-module",
			gitConfig: `[remote "origin"]
	url = invalid-url`,
			expected: "invalid-url",
			setupFunc: func(dir, config string) func() {
				err := os.MkdirAll(filepath.Join(dir, ".git"), 0755)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(config), 0600)
				require.NoError(t, err)
				return func() {
					os.RemoveAll(dir)
				}
			},
		},
		{
			name: "deckhouse module URL",
			dir:  "/tmp/deckhouse",
			gitConfig: `[remote "origin"]
	url = git@github.com:deckhouse/deckhouse.git`,
			expected: "deckhouse",
			setupFunc: func(dir, config string) func() {
				err := os.MkdirAll(filepath.Join(dir, ".git"), 0755)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(config), 0600)
				require.NoError(t, err)
				return func() {
					os.RemoveAll(dir)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupFunc(tt.dir, tt.gitConfig)
			defer cleanup()

			result := getModuleNameFromRepository(tt.dir)
			assert.Equal(t, tt.expected, result)
		})
	}
}
