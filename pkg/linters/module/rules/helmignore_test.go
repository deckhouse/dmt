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

	"github.com/deckhouse/dmt/pkg/errors"
)

func TestNewHelmignoreRule(t *testing.T) {
	tests := []struct {
		name     string
		disable  bool
		expected bool
	}{
		{
			name:     "enabled rule",
			disable:  false,
			expected: true,
		},
		{
			name:     "disabled rule",
			disable:  true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewHelmignoreRule(tt.disable)
			assert.Equal(t, HelmignoreRuleName, rule.GetName())
			assert.Equal(t, tt.expected, rule.Enabled())
		})
	}
}

func TestHelmignoreRule_CheckHelmignore(t *testing.T) {
	tests := []struct {
		name           string
		createFile     bool
		fileContent    string
		directories    []string // directories to create in temp dir
		files          []string // files to create in temp dir
		expectedErrors []string
	}{
		{
			name:       "missing .helmignore file",
			createFile: false,
			expectedErrors: []string{
				"File .helmignore is required in module root",
			},
		},
		{
			name:        "empty .helmignore file",
			createFile:  true,
			fileContent: "",
			expectedErrors: []string{
				"File .helmignore is empty or contains only comments",
			},
		},
		{
			name:        "only comments in .helmignore file",
			createFile:  true,
			fileContent: "# This is a comment\n# Another comment",
			expectedErrors: []string{
				"File .helmignore is empty or contains only comments",
			},
		},
		{
			name:           "valid .helmignore file",
			createFile:     true,
			fileContent:    "# Git\n.git/\n.gitignore\n# Documentation\nREADME.md\ndocs/\n# Development files\n*.md\n*.txt",
			directories:    []string{},
			expectedErrors: []string{},
		},
		{
			name:        "pattern with spaces without quotes",
			createFile:  true,
			fileContent: "file with spaces.txt",
			expectedErrors: []string{
				"Pattern contains spaces without quotes: \"file with spaces.txt\"",
			},
		},
		{
			name:        "too broad pattern",
			createFile:  true,
			fileContent: "*",
			expectedErrors: []string{
				"Pattern is too broad and will exclude everything: \"*\"",
			},
		},
		{
			name:        "pattern that might exclude templates",
			createFile:  true,
			fileContent: "templates/",
			expectedErrors: []string{
				"Pattern might exclude Helm templates: \"templates/\"",
			},
		},
		{
			name:        "pattern that might exclude Chart.yaml",
			createFile:  true,
			fileContent: "Chart.yaml",
			expectedErrors: []string{
				"Pattern might exclude Chart.yaml: \"Chart.yaml\"",
			},
		},
		{
			name:           "exclude templates with negation",
			createFile:     true,
			fileContent:    "!templates/",
			expectedErrors: []string{},
		},
		{
			name:           "exclude Chart.yaml with negation",
			createFile:     true,
			fileContent:    "!Chart.yaml",
			expectedErrors: []string{},
		},
		// --- Directory coverage tests ---
		{
			name:           "all directories covered",
			createFile:     true,
			fileContent:    "hooks/\nimages/\nopenapi/\ndocs/",
			directories:    []string{"hooks", "images", "openapi", "docs", "templates"},
			expectedErrors: []string{},
		},
		{
			name:        "missing directory in helmignore",
			createFile:  true,
			fileContent: "hooks/",
			directories: []string{"hooks", "images"},
			expectedErrors: []string{
				"Directory 'images/' is not listed in .helmignore",
			},
		},
		{
			name:        "multiple missing directories",
			createFile:  true,
			fileContent: "hooks/",
			directories: []string{"hooks", "images", "docs"},
			expectedErrors: []string{
				"Directory 'images/' is not listed in .helmignore",
				"Directory 'docs/' is not listed in .helmignore",
			},
		},
		{
			name:           "directory covered without trailing slash",
			createFile:     true,
			fileContent:    "hooks",
			directories:    []string{"hooks"},
			expectedErrors: []string{},
		},
		{
			name:        "directory covered with wildcard",
			createFile:  true,
			fileContent: "images/*",
			directories: []string{"images"},
			expectedErrors: []string{
				"Directory 'images/' is not listed in .helmignore",
			},
		},
		{
			name:           "wildcard file covered",
			createFile:     true,
			fileContent:    "*.md",
			directories:    []string{},
			files:          []string{"README.md", "CHANGELOG.md"},
			expectedErrors: []string{},
		},
		{
			name:        "file not covered",
			createFile:  true,
			fileContent: "*.md",
			directories: []string{},
			files:       []string{"README.md", "go.mod"},
			expectedErrors: []string{
				"File 'go.mod' is not listed in .helmignore",
			},
		},
		{
			name:        "double-wildcard is rejected by helm",
			createFile:  true,
			fileContent: "images/**",
			directories: []string{},
			expectedErrors: []string{
				"Cannot parse .helmignore: double-star (**) syntax is not supported",
			},
		},
		{
			name:        "negated pattern does not count as covered",
			createFile:  true,
			fileContent: "!hooks/",
			directories: []string{"hooks"},
			expectedErrors: []string{
				"Directory 'hooks/' is not listed in .helmignore",
			},
		},
		{
			name:           "templates directory is skipped",
			createFile:     true,
			fileContent:    "hooks/",
			directories:    []string{"hooks", "templates"},
			expectedErrors: []string{},
		},
		{
			name:           "charts directory is skipped",
			createFile:     true,
			fileContent:    "hooks/",
			directories:    []string{"hooks", "charts"},
			expectedErrors: []string{},
		},
		{
			name:           "empty module root only templates",
			createFile:     true,
			fileContent:    ".git/",
			directories:    []string{"templates"},
			expectedErrors: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tempDir := t.TempDir()

			// Create directories
			for _, dir := range tt.directories {
				err := os.MkdirAll(filepath.Join(tempDir, dir), 0750)
				require.NoError(t, err)
			}

			// Create files
			for _, f := range tt.files {
				err := os.WriteFile(filepath.Join(tempDir, f), []byte("test"), 0600)
				require.NoError(t, err)
			}

			// Create .helmignore file if needed
			if tt.createFile {
				helmignorePath := filepath.Join(tempDir, ".helmignore")
				err := os.WriteFile(helmignorePath, []byte(tt.fileContent), 0600)
				require.NoError(t, err)
			}

			// Create rule and error list
			rule := NewHelmignoreRule(false)
			errorList := errors.NewLintRuleErrorsList()

			// Run the check
			rule.CheckHelmignore(tempDir, errorList)

			// Check errors
			errs := errorList.GetErrors()

			// Collect error texts for comparison
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
		})
	}
}

func TestHelmignoreRule_validatePatterns(t *testing.T) {
	tests := []struct {
		name           string
		patterns       []string
		expectedErrors []string
	}{
		{
			name:           "empty patterns",
			patterns:       []string{},
			expectedErrors: []string{},
		},
		{
			name:           "valid patterns",
			patterns:       []string{"*.log", ".git/", "README.md"},
			expectedErrors: []string{},
		},
		{
			name:     "pattern with spaces",
			patterns: []string{"file with spaces.txt"},
			expectedErrors: []string{
				"Pattern contains spaces without quotes: \"file with spaces.txt\"",
			},
		},
		{
			name:     "too broad pattern",
			patterns: []string{"*"},
			expectedErrors: []string{
				"Pattern is too broad and will exclude everything: \"*\"",
			},
		},
		{
			name:     "pattern that might exclude templates",
			patterns: []string{"templates/"},
			expectedErrors: []string{
				"Pattern might exclude Helm templates: \"templates/\"",
			},
		},
		{
			name:     "pattern that might exclude Chart.yaml",
			patterns: []string{"Chart.yaml"},
			expectedErrors: []string{
				"Pattern might exclude Chart.yaml: \"Chart.yaml\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorList := errors.NewLintRuleErrorsList()
			validatePatterns(tt.patterns, errorList)

			errs := errorList.GetErrors()
			assert.Len(t, errs, len(tt.expectedErrors), "Expected %d errors, got %d", len(tt.expectedErrors), len(errs))

			for i, expectedError := range tt.expectedErrors {
				if i < len(errs) {
					assert.Contains(t, errs[i].Text, expectedError)
				}
			}
		})
	}
}
