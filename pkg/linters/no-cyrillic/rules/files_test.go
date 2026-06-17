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

// cyrillicYAML is a snippet of valid YAML that contains Cyrillic characters, so
// it would always be reported by the linter unless the file is skipped.
const cyrillicYAML = "greeting: Привет, мир\n"

// TestFilesRule_CheckFile_SkipPatterns exercises every filename / path pattern
// that the files rule is expected to skip (or not skip). Each case writes a file
// containing Cyrillic and asserts whether the rule reported it.
func TestFilesRule_CheckFile_SkipPatterns(t *testing.T) {
	tests := []struct {
		name        string
		relPath     string
		content     string
		wantSkipped bool
	}{
		// doc-ru-*.y[a]ml
		{name: "doc-ru yaml", relPath: "doc-ru-resources.yaml", wantSkipped: true},
		{name: "doc-ru yml", relPath: "doc-ru-config.yml", wantSkipped: true},
		{name: "doc-ru nested", relPath: "openapi/doc-ru-values.yaml", wantSkipped: true},

		// *.ru.{yaml,yml,json,md,html}
		{name: "ru yaml suffix", relPath: "values.ru.yaml", wantSkipped: true},
		{name: "ru yml suffix", relPath: "CHANGELOG/v0.3.21.ru.yml", wantSkipped: true},
		{name: "ru json suffix", relPath: "messages.ru.json", wantSkipped: true},
		{name: "ru md suffix", relPath: "guide.ru.md", wantSkipped: true},
		{name: "ru html suffix", relPath: "page.ru.html", wantSkipped: true},

		// *_RU.md and *_ru.html
		{name: "RU md suffix", relPath: "README_RU.md", wantSkipped: true},
		{name: "ru html underscore", relPath: "index_ru.html", wantSkipped: true},

		// docs/site and docs/documentation underscore-prefixed includes
		{name: "docs site include", relPath: "docs/site/_header.yaml", wantSkipped: true},
		{name: "docs documentation include", relPath: "docs/documentation/_nav.yaml", wantSkipped: true},

		// tools/spelling and openapi/conversions
		{name: "tools spelling", relPath: "tools/spelling/wordlist.yaml", wantSkipped: true},
		{name: "openapi conversions", relPath: "openapi/conversions/v1.yaml", wantSkipped: true},

		// module.yaml (module definition carries localized descriptions)
		{name: "module yaml", relPath: "module.yaml", wantSkipped: true},

		// ru.* prefix (e.g. ru.meta.deckhouse.io/description annotation files)
		{name: "ru meta prefix", relPath: "ru.meta.deckhouse.io.yaml", wantSkipped: true},
		{name: "ru dot prefix nested", relPath: "config/ru.description.yaml", wantSkipped: true},

		// i18n translation directory
		{name: "i18n dir", relPath: "i18n/messages.yaml", wantSkipped: true},

		// the linter's own source
		{name: "linter self", relPath: "no_cyrillic.go", content: "package nocyrillic\n// Привет\n", wantSkipped: true},
		{name: "linter self test", relPath: "no_cyrillic_test.go", content: "package nocyrillic\n// Привет\n", wantSkipped: true},

		// Files that must NOT be skipped and therefore must be reported.
		{name: "plain yaml", relPath: "config.yaml", wantSkipped: false},
		{name: "plain json", relPath: "data.json", content: "{\"greeting\": \"Привет\"}\n", wantSkipped: false},
		{name: "plain go", relPath: "main.go", content: "package main\n// Привет\n", wantSkipped: false},
		{name: "nested template", relPath: "templates/configmap.yaml", wantSkipped: false},
		// English doc-ru lookalike that does not match the doc-ru- prefix.
		{name: "ru in middle of name", relPath: "instructions.yaml", wantSkipped: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := minimock.NewController(t)
			mockModule := mocks.NewModuleMock(mc)

			tempDir := t.TempDir()
			mockModule.GetPathMock.Return(tempDir)

			content := tt.content
			if content == "" {
				content = cyrillicYAML
			}

			testFile := filepath.Join(tempDir, filepath.FromSlash(tt.relPath))
			if err := os.MkdirAll(filepath.Dir(testFile), 0700); err != nil {
				t.Fatalf("failed to create dir: %v", err)
			}

			if err := os.WriteFile(testFile, []byte(content), 0600); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			rule := NewFilesRule(nil, nil)
			errorList := &errors.LintRuleErrorsList{}

			rule.CheckFile(mockModule, testFile, errorList)

			errs := errorList.GetErrors()
			if tt.wantSkipped && len(errs) > 0 {
				t.Errorf("expected %q to be skipped, but got %d error(s)", tt.relPath, len(errs))
			}

			if !tt.wantSkipped && len(errs) == 0 {
				t.Errorf("expected %q to be reported, but got no errors", tt.relPath)
			}
		})
	}
}

// TestFilesRule_CheckFile_NoCyrillicNotReported makes sure that files without
// Cyrillic produce no findings regardless of their (non-skipped) name.
func TestFilesRule_CheckFile_NoCyrillicNotReported(t *testing.T) {
	for _, relPath := range []string{"config.yaml", "data.json", "main.go"} {
		t.Run(relPath, func(t *testing.T) {
			mc := minimock.NewController(t)
			mockModule := mocks.NewModuleMock(mc)

			tempDir := t.TempDir()
			mockModule.GetPathMock.Return(tempDir)

			testFile := filepath.Join(tempDir, relPath)
			if err := os.WriteFile(testFile, []byte("greeting: hello world\n"), 0600); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			rule := NewFilesRule(nil, nil)
			errorList := &errors.LintRuleErrorsList{}

			rule.CheckFile(mockModule, testFile, errorList)

			if errs := errorList.GetErrors(); len(errs) > 0 {
				t.Errorf("expected no errors for cyrillic-free file %q, got %d", relPath, len(errs))
			}
		})
	}
}

// TestFilesRule_CheckFile_ExcludeDirectories verifies that directory exclude
// rules suppress findings for files beneath the excluded prefix.
func TestFilesRule_CheckFile_ExcludeDirectories(t *testing.T) {
	mc := minimock.NewController(t)
	mockModule := mocks.NewModuleMock(mc)

	tempDir := t.TempDir()
	mockModule.GetPathMock.Return(tempDir)

	testFile := filepath.Join(tempDir, "vendor", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(testFile), 0700); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	if err := os.WriteFile(testFile, []byte(cyrillicYAML), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	excludeDirs := []pkg.DirectoryRuleExclude{
		pkg.DirectoryRuleExclude("vendor"),
	}
	rule := NewFilesRule(nil, excludeDirs)
	errorList := &errors.LintRuleErrorsList{}

	rule.CheckFile(mockModule, testFile, errorList)

	if errs := errorList.GetErrors(); len(errs) > 0 {
		t.Errorf("expected file under excluded directory to be skipped, got %d errors", len(errs))
	}
}

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

func TestFilesRule_CheckFile_SkipRussianYAMLFile(t *testing.T) {
	mc := minimock.NewController(t)

	// Create a mock module
	mockModule := mocks.NewModuleMock(mc)

	// Create test directory structure
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "v0.3.21.ru.yml")

	// Create a Russian YAML file with cyrillic content (should be skipped)
	err := os.WriteFile(testFile, []byte("changes:\n  - Изменения в CI\n"), 0600)
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

	// Verify that Russian YAML file was skipped (no errors)
	errs := errorList.GetErrors()
	if len(errs) > 0 {
		t.Errorf("Expected Russian YAML file to be skipped, but got %d errors", len(errs))
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

// TestFilesRule_CheckFile_DirectoryExcludeNotPrefix verified that directory excludes
// do not incorrectly match similarly-named sibling directories via prefix matching.
func TestFilesRule_CheckFile_DirectoryExcludeNotPrefix(t *testing.T) {
	tests := []struct {
		name        string
		relPath     string
		wantSkipped bool
	}{
		{
			name:        "exact dir match",
			relPath:     "images/stronghold/config.yaml",
			wantSkipped: true,
		},
		{
			name:        "subdirectory match",
			relPath:     "images/stronghold/subdir/config.yaml",
			wantSkipped: true,
		},
		{
			name:        "sibling dir with dash suffix not matched",
			relPath:     "images/stronghold-automatic/config.yaml",
			wantSkipped: false,
		},
		{
			name:        "sibling dir with suffix not matched",
			relPath:     "images/stronghold-for-dmt-abuse/config.yaml",
			wantSkipped: false,
		},
		{
			name:        "parent dir not matched",
			relPath:     "images/config.yaml",
			wantSkipped: false,
		},
		{
			name:        "unrelated dir not matched",
			relPath:     "hooks/config.yaml",
			wantSkipped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := minimock.NewController(t)
			mockModule := mocks.NewModuleMock(mc)

			tempDir := t.TempDir()
			mockModule.GetPathMock.Return(tempDir)

			testFile := filepath.Join(tempDir, filepath.FromSlash(tt.relPath))
			if err := os.MkdirAll(filepath.Dir(testFile), 0700); err != nil {
				t.Fatalf("failed to create dir: %v", err)
			}

			if err := os.WriteFile(testFile, []byte(cyrillicYAML), 0600); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			excludeDirs := []pkg.DirectoryRuleExclude{
				pkg.DirectoryRuleExclude("images/stronghold"),
			}
			rule := NewFilesRule(nil, excludeDirs)
			errorList := &errors.LintRuleErrorsList{}

			rule.CheckFile(mockModule, testFile, errorList)

			errs := errorList.GetErrors()
			if tt.wantSkipped && len(errs) > 0 {
				t.Errorf("expected %q to be skipped, but got %d error(s)", tt.relPath, len(errs))
			}
			if !tt.wantSkipped && len(errs) == 0 {
				t.Errorf("expected %q to be reported, but got no errors", tt.relPath)
			}
		})
	}
}

// TestFilesRule_CheckFile_directory_exclude_trailing_slash verifies that
// a trailing slash in the exclude directory name is handled correctly
// (same behavior as without trailing slash).
func TestFilesRule_CheckFile_directory_exclude_trailing_slash(t *testing.T) {
	mc := minimock.NewController(t)
	mockModule := mocks.NewModuleMock(mc)

	tempDir := t.TempDir()
	mockModule.GetPathMock.Return(tempDir)

	testFile := filepath.Join(tempDir, "vendor", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(testFile), 0700); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(testFile, []byte(cyrillicYAML), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	excludeDirs := []pkg.DirectoryRuleExclude{
		pkg.DirectoryRuleExclude("vendor/"),
	}
	rule := NewFilesRule(nil, excludeDirs)
	errorList := &errors.LintRuleErrorsList{}

	rule.CheckFile(mockModule, testFile, errorList)

	if errs := errorList.GetErrors(); len(errs) > 0 {
		t.Errorf("expected file under excluded directory (with trailing slash) to be skipped, got %d errors", len(errs))
	}
}
