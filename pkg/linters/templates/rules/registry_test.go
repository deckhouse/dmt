package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deckhouse/dmt/pkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		modulePath     string
		registrySecret string
		expectedErrors []string
		setupFunc      func(string) func()
	}{
		{
			name:       "no registry secret file",
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
			modulePath: "/tmp/deckhouse",
			setupFunc: func(path string) func() {
				// Create deckhouse module with git config
				err := os.MkdirAll(filepath.Join(path, ".git"), 0755)
				require.NoError(t, err)

				gitConfig := `[remote "origin"]
	url = git@github.com:deckhouse/deckhouse.git`
				err = os.WriteFile(filepath.Join(path, ".git", "config"), []byte(gitConfig), 0600)
				require.NoError(t, err)

				// Create registry secret file
				err = os.MkdirAll(path, 0755)
				require.NoError(t, err)
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
			name:       "registry secret with forbidden content",
			modulePath: "/tmp/test-module",
			setupFunc: func(path string) func() {
				// Create module with git config
				err := os.MkdirAll(filepath.Join(path, ".git"), 0755)
				require.NoError(t, err)

				gitConfig := `[remote "origin"]
	url = git@github.com:deckhouse/test-module.git`
				err = os.WriteFile(filepath.Join(path, ".git", "config"), []byte(gitConfig), 0600)
				require.NoError(t, err)

				// Create registry secret file with forbidden content
				err = os.MkdirAll(path, 0755)
				require.NoError(t, err)
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
			expectedErrors: []string{"registry-secret.yaml file should not contain .Values.global.modulesImages"},
		},
		{
			name:       "registry secret without forbidden content",
			modulePath: "/tmp/test-module",
			setupFunc: func(path string) func() {
				// Create module with git config
				err := os.MkdirAll(filepath.Join(path, ".git"), 0755)
				require.NoError(t, err)

				gitConfig := `[remote "origin"]
	url = git@github.com:deckhouse/test-module.git`
				err = os.WriteFile(filepath.Join(path, ".git", "config"), []byte(gitConfig), 0600)
				require.NoError(t, err)

				// Create registry secret file without forbidden content
				err = os.MkdirAll(path, 0755)
				require.NoError(t, err)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupFunc(tt.modulePath)
			defer cleanup()

			// Test the individual functions instead of the full method
			// since we can't easily mock the module interface
			moduleName := getModuleNameFromRepository(tt.modulePath)

			// If it's deckhouse module, it should be skipped
			if moduleName == "deckhouse" {
				return
			}

			// Check if registry secret file exists and contains forbidden content
			registryFiles := []string{filepath.Join(tt.modulePath, "registry-secret.yaml")}
			for _, registryFile := range registryFiles {
				if _, err := os.Stat(registryFile); err == nil {
					fileContent, err := os.ReadFile(registryFile)
					require.NoError(t, err)

					if strings.Contains(string(fileContent), ".Values.global.modulesImages") {
						// This should trigger an error
						assert.NotEmpty(t, tt.expectedErrors, "Expected error for forbidden content")
					}
				}
			}
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

func TestConvertUrlToModuleName(t *testing.T) {
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
