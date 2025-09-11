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

	"github.com/gojuno/minimock/v3"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestFilesRule_CheckFile_WithMock(t *testing.T) {
	mc := minimock.NewController(t)

	// Create a mock module
	mockModule := mocks.NewModuleMock(mc)

	// Create test directory structure
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	// Create a file with cyrillic content
	err := os.WriteFile(testFile, []byte("package test\n// Привет мир\n"), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Setup mock expectations
	mockModule.GetPathMock.Return(tempDir)

	// Create rule and error list
	rule := NewFilesRule(nil, nil)
	errorList := &errors.LintRuleErrorsList{}

	// Test the rule
	rule.CheckFile(mockModule, testFile, errorList)

	// Verify that cyrillic was detected
	errs := errorList.GetErrors()
	if len(errs) == 0 {
		t.Error("Expected cyrillic detection error, but got none")
	}
}

func TestFilesRule_CheckFile_SkipRussianFile(t *testing.T) {
	mc := minimock.NewController(t)

	// Create a mock module
	mockModule := mocks.NewModuleMock(mc)

	// Create test directory structure
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "README_RU.md")

	// Create a Russian file with cyrillic content (should be skipped)
	err := os.WriteFile(testFile, []byte("# Документация\nПривет мир\n"), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Setup mock expectations
	mockModule.GetPathMock.Return(tempDir)

	// Create rule and error list
	rule := NewFilesRule(nil, nil)
	errorList := &errors.LintRuleErrorsList{}

	// Test the rule
	rule.CheckFile(mockModule, testFile, errorList)

	// Verify that Russian file was skipped (no errors)
	errs := errorList.GetErrors()
	if len(errs) > 0 {
		t.Errorf("Expected Russian file to be skipped, but got %d errors", len(errs))
	}
}

func TestFilesRule_CheckFile_WithExcludeRules(t *testing.T) {
	mc := minimock.NewController(t)

	// Create a mock module
	mockModule := mocks.NewModuleMock(mc)

	// Create test directory structure
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "excluded.go")

	// Create a file with cyrillic content
	err := os.WriteFile(testFile, []byte("package test\n// Привет мир\n"), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Setup mock expectations
	mockModule.GetPathMock.Return(tempDir)

	// Create rule with exclude rules
	excludeRules := []pkg.StringRuleExclude{
		pkg.StringRuleExclude("excluded.go"),
	}
	rule := NewFilesRule(excludeRules, nil)
	errorList := &errors.LintRuleErrorsList{}

	// Test the rule
	rule.CheckFile(mockModule, testFile, errorList)

	// Verify that file was excluded (no errors)
	errs := errorList.GetErrors()
	if len(errs) > 0 {
		t.Errorf("Expected file to be excluded, but got %d errors", len(errs))
	}
}
