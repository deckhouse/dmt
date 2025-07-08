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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tempDir := t.TempDir()

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
			assert.Len(t, errs, len(tt.expectedErrors), "Expected %d errors, got %d", len(tt.expectedErrors), len(errs))

			for i, expectedError := range tt.expectedErrors {
				if i < len(errs) {
					assert.Contains(t, errs[i].Text, expectedError)
				}
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
