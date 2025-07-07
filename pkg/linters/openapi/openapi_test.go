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

package openapi

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilterCRDsfiles(t *testing.T) {
	tests := []struct {
		name     string
		rootPath string
		path     string
		expected bool
	}{
		// Valid CRD files
		{
			name:     "valid crd yaml file",
			rootPath: "/tmp/module",
			path:     "/tmp/module/crds/test.yaml",
			expected: true,
		},
		{
			name:     "valid crd yml file",
			rootPath: "/tmp/module",
			path:     "/tmp/module/crds/test.yml",
			expected: true,
		},
		{
			name:     "valid crd file in subdirectory",
			rootPath: "/tmp/module",
			path:     "/tmp/module/crds/subdir/test.yaml",
			expected: true,
		},
		{
			name:     "valid crd file with complex name",
			rootPath: "/tmp/module",
			path:     "/tmp/module/crds/my-custom-resource-definition.yaml",
			expected: true,
		},
		{
			name:     "valid crd file with numbers",
			rootPath: "/tmp/module",
			path:     "/tmp/module/crds/crd-v1beta1.yaml",
			expected: true,
		},

		// Invalid paths - not in crds directory
		{
			name:     "file not in crds directory",
			rootPath: "/tmp/module",
			path:     "/tmp/module/openapi/test.yaml",
			expected: false,
		},
		{
			name:     "file in root directory",
			rootPath: "/tmp/module",
			path:     "/tmp/module/test.yaml",
			expected: false,
		},
		{
			name:     "file in other subdirectory",
			rootPath: "/tmp/module",
			path:     "/tmp/module/templates/test.yaml",
			expected: false,
		},
		{
			name:     "file with crds in name but not in crds directory",
			rootPath: "/tmp/module",
			path:     "/tmp/module/templates/crds-test.yaml",
			expected: false,
		},

		// Invalid file extensions
		{
			name:     "file with .json extension",
			rootPath: "/tmp/module",
			path:     "/tmp/module/crds/test.json",
			expected: false,
		},
		{
			name:     "file with .txt extension",
			rootPath: "/tmp/module",
			path:     "/tmp/module/crds/test.txt",
			expected: false,
		},
		{
			name:     "file without extension",
			rootPath: "/tmp/module",
			path:     "/tmp/module/crds/test",
			expected: false,
		},

		// Excluded files
		{
			name:     "test file with -tests.yaml suffix",
			rootPath: "/tmp/module",
			path:     "/tmp/module/crds/test-tests.yaml",
			expected: false,
		},
		{
			name:     "test file with -tests.yml suffix",
			rootPath: "/tmp/module",
			path:     "/tmp/module/crds/test-tests.yml",
			expected: false,
		},
		{
			name:     "russian documentation file",
			rootPath: "/tmp/module",
			path:     "/tmp/module/crds/doc-ru-test.yaml",
			expected: false,
		},
		{
			name:     "russian documentation file with yml extension",
			rootPath: "/tmp/module",
			path:     "/tmp/module/crds/doc-ru-test.yml",
			expected: false,
		},

		// Edge cases
		{
			name:     "file with doc-ru in middle of name",
			rootPath: "/tmp/module",
			path:     "/tmp/module/crds/my-doc-ru-file.yaml",
			expected: true,
		},
		{
			name:     "file ending with tests but not -tests",
			rootPath: "/tmp/module",
			path:     "/tmp/module/crds/mytests.yaml",
			expected: true,
		},
		{
			name:     "file starting with doc but not doc-ru",
			rootPath: "/tmp/module",
			path:     "/tmp/module/crds/doc-en-test.yaml",
			expected: true,
		},

		// Different root paths
		{
			name:     "different root path",
			rootPath: "/home/user/project",
			path:     "/home/user/project/crds/test.yaml",
			expected: true,
		},
		{
			name:     "relative root path",
			rootPath: ".",
			path:     "./crds/test.yaml",
			expected: true,
		},
		{
			name:     "absolute path with relative root",
			rootPath: ".",
			path:     "/absolute/path/crds/test.yaml",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterCRDsfiles(tt.rootPath, tt.path)
			if result != tt.expected {
				t.Errorf("filterCRDsfiles(%q, %q) = %v, want %v", tt.rootPath, tt.path, result, tt.expected)
			}
		})
	}
}

func TestFilterCRDsfilesWithRealPaths(t *testing.T) {
	// Create temporary directory structure for testing
	tempDir := t.TempDir()

	// Create crds directory
	crdsDir := filepath.Join(tempDir, "crds")
	if err := os.MkdirAll(crdsDir, 0755); err != nil {
		t.Fatalf("Failed to create crds directory: %v", err)
	}

	// Create other directories for comparison
	openapiDir := filepath.Join(tempDir, "openapi")
	if err := os.MkdirAll(openapiDir, 0755); err != nil {
		t.Fatalf("Failed to create openapi directory: %v", err)
	}

	templatesDir := filepath.Join(tempDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates directory: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "valid crd file",
			path:     filepath.Join(crdsDir, "test.yaml"),
			expected: true,
		},
		{
			name:     "valid crd file with yml extension",
			path:     filepath.Join(crdsDir, "test.yml"),
			expected: true,
		},
		{
			name:     "test file excluded",
			path:     filepath.Join(crdsDir, "test-tests.yaml"),
			expected: false,
		},
		{
			name:     "russian doc file excluded",
			path:     filepath.Join(crdsDir, "doc-ru-test.yaml"),
			expected: false,
		},
		{
			name:     "file in openapi directory",
			path:     filepath.Join(openapiDir, "test.yaml"),
			expected: false,
		},
		{
			name:     "file in templates directory",
			path:     filepath.Join(templatesDir, "test.yaml"),
			expected: false,
		},
		{
			name:     "file in root directory",
			path:     filepath.Join(tempDir, "test.yaml"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterCRDsfiles(tempDir, tt.path)
			if result != tt.expected {
				t.Errorf("filterCRDsfiles(%q, %q) = %v, want %v", tempDir, tt.path, result, tt.expected)
			}
		})
	}
}

func TestFilterCRDsfilesRegexPattern(t *testing.T) {
	// Test that the regex pattern works correctly
	tests := []struct {
		path     string
		expected bool
	}{
		{"crds/test.yaml", true},
		{"crds/test.yml", true},
		{"crds/subdir/test.yaml", true},
		{"crds/subdir/deep/test.yml", true},
		{"openapi/test.yaml", false},
		{"templates/test.yaml", false},
		{"test.yaml", false},
		{"crds/test.json", false},
		{"crds/test.txt", false},
		{"crds/test", false},
		{"crds/", false},
		{"crds", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			// Simulate the regex check that happens in filterCRDsfiles
			// (without the exclusion checks)
			result := crdsYamlRegex.MatchString(tt.path)
			if result != tt.expected {
				t.Errorf("crdsYamlRegex.MatchString(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestFilterOpenAPIfiles(t *testing.T) {
	tests := []struct {
		name     string
		rootPath string
		path     string
		expected bool
	}{
		// Valid OpenAPI files
		{
			name:     "valid openapi yaml file",
			rootPath: "/tmp/module",
			path:     "/tmp/module/openapi/test.yaml",
			expected: true,
		},
		{
			name:     "valid openapi yml file",
			rootPath: "/tmp/module",
			path:     "/tmp/module/openapi/test.yml",
			expected: true,
		},
		{
			name:     "valid openapi file in subdirectory",
			rootPath: "/tmp/module",
			path:     "/tmp/module/openapi/subdir/test.yaml",
			expected: true,
		},

		// Invalid paths - not in openapi directory
		{
			name:     "file not in openapi directory",
			rootPath: "/tmp/module",
			path:     "/tmp/module/crds/test.yaml",
			expected: false,
		},
		{
			name:     "file in root directory",
			rootPath: "/tmp/module",
			path:     "/tmp/module/test.yaml",
			expected: false,
		},

		// Excluded files
		{
			name:     "test file with -tests.yaml suffix",
			rootPath: "/tmp/module",
			path:     "/tmp/module/openapi/test-tests.yaml",
			expected: false,
		},
		{
			name:     "test file with -tests.yml suffix",
			rootPath: "/tmp/module",
			path:     "/tmp/module/openapi/test-tests.yml",
			expected: false,
		},
		{
			name:     "russian documentation file",
			rootPath: "/tmp/module",
			path:     "/tmp/module/openapi/doc-ru-test.yaml",
			expected: false,
		},
		{
			name:     "russian documentation file with yml extension",
			rootPath: "/tmp/module",
			path:     "/tmp/module/openapi/doc-ru-test.yml",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterOpenAPIfiles(tt.rootPath, tt.path)
			if result != tt.expected {
				t.Errorf("filterOpenAPIfiles(%q, %q) = %v, want %v", tt.rootPath, tt.path, result, tt.expected)
			}
		})
	}
}

func TestFilterOpenAPIfilesRegexPattern(t *testing.T) {
	// Test that the regex pattern works correctly
	tests := []struct {
		path     string
		expected bool
	}{
		{"openapi/test.yaml", true},
		{"openapi/test.yml", true},
		{"openapi/subdir/test.yaml", true},
		{"openapi/subdir/deep/test.yml", true},
		{"crds/test.yaml", false},
		{"templates/test.yaml", false},
		{"test.yaml", false},
		{"openapi/test.json", false},
		{"openapi/test.txt", false},
		{"openapi/test", false},
		{"openapi/", false},
		{"openapi", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			// Simulate the regex check that happens in filterOpenAPIfiles
			// (without the exclusion checks)
			result := openapiYamlRegex.MatchString(tt.path)
			if result != tt.expected {
				t.Errorf("openapiYamlRegex.MatchString(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}
