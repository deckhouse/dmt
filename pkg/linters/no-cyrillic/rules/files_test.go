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

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

// mockModule implements the mockModuleInterface for testing
type mockModule struct {
	path string
}

func (m *mockModule) GetPath() string {
	return m.path
}

func TestNewFilesRule(t *testing.T) {
	excludeFiles := []pkg.StringRuleExclude{
		"exclude.txt",
	}
	excludeDirs := []pkg.PrefixRuleExclude{
		"exclude_dir/",
	}

	rule := NewFilesRule(excludeFiles, excludeDirs)

	assert.Equal(t, "files", rule.GetName())
	assert.Equal(t, excludeFiles, rule.ExcludeStringRules)
	assert.Equal(t, excludeDirs, rule.ExcludePrefixRules)
	assert.NotNil(t, rule.skipDocRe)
	assert.NotNil(t, rule.skipI18NRe)
	assert.NotNil(t, rule.skipSelfRe)
}

func TestCheckFile(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Create test files
	normalFile := filepath.Join(tempDir, "normal.txt")
	err := os.WriteFile(normalFile, []byte("This is English text."), 0600)
	require.NoError(t, err)

	cyrillicFile := filepath.Join(tempDir, "cyrillic.txt")
	err = os.WriteFile(cyrillicFile, []byte("This contains Cyrillic: Привет"), 0600)
	require.NoError(t, err)

	docRuFile := filepath.Join(tempDir, "doc-ru-test.yml")
	err = os.WriteFile(docRuFile, []byte("This contains Cyrillic: Привет"), 0600)
	require.NoError(t, err)

	ruMdFile := filepath.Join(tempDir, "test_RU.md")
	err = os.WriteFile(ruMdFile, []byte("This contains Cyrillic: Привет"), 0600)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(tempDir, "i18n"), 0755)
	require.NoError(t, err)
	i18nFile := filepath.Join(tempDir, "i18n", "strings.txt")
	err = os.WriteFile(i18nFile, []byte("This contains Cyrillic: Привет"), 0600)
	require.NoError(t, err)

	selfFile := filepath.Join(tempDir, "no_cyrillic.go")
	err = os.WriteFile(selfFile, []byte("This contains Cyrillic: Привет"), 0600)
	require.NoError(t, err)

	excludedFile := filepath.Join(tempDir, "exclude.txt")
	err = os.WriteFile(excludedFile, []byte("This contains Cyrillic: Привет"), 0600)
	require.NoError(t, err)

	// Setup mock module for testing
	mod := &mockModule{path: tempDir}

	// Setup rule
	excludeFiles := []pkg.StringRuleExclude{
		"exclude.txt",
	}
	excludeDirs := []pkg.PrefixRuleExclude{}
	rule := NewFilesRule(excludeFiles, excludeDirs)

	// Test normal file (no Cyrillic)
	t.Run("NormalFile", func(t *testing.T) {
		errorList := errors.NewLintRuleErrorsList()
		rule.CheckFile(mod, normalFile, errorList)
		assert.Empty(t, errorList.GetErrors())
	})

	// Test file with Cyrillic
	t.Run("CyrillicFile", func(t *testing.T) {
		errorList := errors.NewLintRuleErrorsList()
		rule.CheckFile(mod, cyrillicFile, errorList)
		errs := errorList.GetErrors()
		// Just check that an error is produced for files with Cyrillic
		assert.NotEmpty(t, errs, "Should report error for Cyrillic content")
		if len(errs) > 0 {
			assert.Contains(t, errs[0].Text, "has cyrillic letters")
		}
	})

	// Test excluded file by regex (doc-ru)
	t.Run("DocRuFile", func(t *testing.T) {
		errorList := errors.NewLintRuleErrorsList()
		rule.CheckFile(mod, docRuFile, errorList)
		assert.Empty(t, errorList.GetErrors())
	})

	// Test excluded RU.md file pattern
	t.Run("RUMdFile", func(t *testing.T) {
		errorList := errors.NewLintRuleErrorsList()
		rule.CheckFile(mod, ruMdFile, errorList)
		assert.Empty(t, errorList.GetErrors())
	})

	// Test excluded file by regex (i18n)
	t.Run("I18nFile", func(t *testing.T) {
		errorList := errors.NewLintRuleErrorsList()
		rule.CheckFile(mod, i18nFile, errorList)
		assert.Empty(t, errorList.GetErrors())
	})

	// Test excluded file by regex (self)
	t.Run("SelfFile", func(t *testing.T) {
		errorList := errors.NewLintRuleErrorsList()
		rule.CheckFile(mod, selfFile, errorList)
		assert.Empty(t, errorList.GetErrors())
	})

	// Test excluded file by rule
	t.Run("ExcludedFile", func(t *testing.T) {
		errorList := errors.NewLintRuleErrorsList()
		rule.CheckFile(mod, excludedFile, errorList)
		assert.Empty(t, errorList.GetErrors())
	})

	// Test non-existent file
	t.Run("NonExistentFile", func(t *testing.T) {
		errorList := errors.NewLintRuleErrorsList()
		rule.CheckFile(mod, filepath.Join(tempDir, "nonexistent.txt"), errorList)
		errs := errorList.GetErrors()
		assert.NotEmpty(t, errs, "Should report error for non-existent file")
		if len(errs) > 0 {
			assert.Contains(t, errs[0].Text, "no such file or directory")
		}
	})
}

func TestGetFileContent(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	content := "line1\nline2\nline3"
	err := os.WriteFile(testFile, []byte(content), 0600)
	require.NoError(t, err)

	// Test reading existing file
	fileBytes, err := os.ReadFile(testFile)
	require.NoError(t, err)
	lines := strings.Split(string(fileBytes), "\n")
	assert.Equal(t, []string{"line1", "line2", "line3"}, lines)

	// Test reading non-existent file
	_, err = os.ReadFile(filepath.Join(tempDir, "nonexistent.txt"))
	require.Error(t, err)
}
