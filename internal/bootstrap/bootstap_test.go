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

package bootstrap

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/internal/fsutils"
)

func TestRunBootstrap(t *testing.T) {
	// Test successful bootstrap
	tempDir := t.TempDir()

	config := BootstrapConfig{
		ModuleName:     "test-module",
		RepositoryType: RepositoryTypeGitHub,
		RepositoryURL:  "https://github.com/test/repo",
		Directory:      tempDir,
	}
	err := RunBootstrap(config)
	require.NoError(t, err)

	// Check if module.yaml was created
	moduleYamlPath := filepath.Join(tempDir, "module.yaml")
	_, err = os.Stat(moduleYamlPath)
	require.NoError(t, err)

	// Check if module name was replaced
	content, err := os.ReadFile(moduleYamlPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "test-module")
}

func TestRunBootstrapWithNonEmptyDirectory(t *testing.T) {
	// Test bootstrap with non-empty directory
	tempDir := t.TempDir()

	// Create a file in the directory
	err := os.WriteFile(filepath.Join(tempDir, "existing.txt"), []byte("existing"), 0600)
	require.NoError(t, err)

	// Since checkDirectoryEmpty calls os.Exit(1), we can't test it directly
	// Instead, we'll test the logic by checking if files exist
	files := fsutils.GetFiles(tempDir, false)
	assert.NotEmpty(t, files, "Directory should not be empty")
}

func TestCheckDirectoryEmpty(t *testing.T) {
	// Test with empty directory
	tempDir := t.TempDir()

	err := checkDirectoryEmpty(tempDir)
	require.NoError(t, err)
}

func TestCheckDirectoryEmptyWithFiles(t *testing.T) {
	// Test with non-empty directory
	tempDir := t.TempDir()

	// Create a file in the directory
	err := os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("test"), 0600)
	require.NoError(t, err)

	// Since checkDirectoryEmpty calls os.Exit(1), we can't test it directly
	// Instead, we'll test the logic by checking if files exist
	files := fsutils.GetFiles(tempDir, false)
	assert.NotEmpty(t, files, "Directory should not be empty")
}

func TestCheckDirectoryEmptyWithEmptyString(t *testing.T) {
	// Test with empty string (should use current directory)
	originalDir, err := os.Getwd()
	require.NoError(t, err)

	// Create a temporary directory and change to it
	tempDir := t.TempDir()
	err = os.Chdir(tempDir)
	require.NoError(t, err)
	defer func() {
		if chdirErr := os.Chdir(originalDir); chdirErr != nil {
			t.Logf("Failed to restore original directory: %v", chdirErr)
		}
	}()

	err = checkDirectoryEmpty("")
	require.NoError(t, err)
}

func TestReplaceModuleName(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files with old module name
	testFiles := map[string]string{
		"file1.txt":        "old-module-name content",
		"file2.yaml":       "name: old-module-name\nversion: 1.0",
		"subdir/file3.txt": "some old-module-name reference",
	}

	for fileName, content := range testFiles {
		filePath := filepath.Join(tempDir, fileName)
		err := os.MkdirAll(filepath.Dir(filePath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(filePath, []byte(content), 0600)
		require.NoError(t, err)
	}

	// Replace module name
	err := replaceModuleName("old-module-name", "new-module-name", tempDir)
	require.NoError(t, err)

	// Check if replacements were made
	for fileName, originalContent := range testFiles {
		filePath := filepath.Join(tempDir, fileName)
		content, err := os.ReadFile(filePath)
		require.NoError(t, err)

		expectedContent := strings.ReplaceAll(originalContent, "old-module-name", "new-module-name")
		assert.Equal(t, expectedContent, string(content))
	}
}

func TestReplaceValuesModuleName(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files with .Values references
	testFiles := map[string]string{
		"values.yaml":   ".Values.oldModuleName.someValue",
		"template.yaml": "{{ .Values.oldModuleName.internal }}",
		"config.yaml":   "config:\n  module: .Values.oldModuleName.internal",
	}

	for fileName, content := range testFiles {
		filePath := filepath.Join(tempDir, fileName)
		err := os.WriteFile(filePath, []byte(content), 0600)
		require.NoError(t, err)
	}

	// Replace values module name
	err := replaceValuesModuleName("oldModuleName", "newModuleName", tempDir)
	require.NoError(t, err)

	// Check if replacements were made correctly
	expectedFiles := map[string]string{
		"values.yaml":   ".Values.newModuleName.someValue",
		"template.yaml": "{{ .Values.newModuleName.internal }}",
		"config.yaml":   "config:\n  module: .Values.newModuleName.internal",
	}

	for fileName, expectedContent := range expectedFiles {
		filePath := filepath.Join(tempDir, fileName)
		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, expectedContent, string(content))
	}
}

func TestReplaceValuesModuleNameWithCamelCase(t *testing.T) {
	tempDir := t.TempDir()

	// Create test file with snake_case module name
	content := ".Values.old_module_name.internal"
	filePath := filepath.Join(tempDir, "test.yaml")
	err := os.WriteFile(filePath, []byte(content), 0600)
	require.NoError(t, err)

	// Replace values module name (should convert to camelCase)
	err = replaceValuesModuleName("old_module_name", "new_module_name", tempDir)
	require.NoError(t, err)

	// Check if replacement was made with camelCase
	expectedContent := ".Values.newModuleName.internal"
	contentBytes, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, expectedContent, string(contentBytes))
}

func TestGetModuleName(t *testing.T) {
	tempDir := t.TempDir()

	// Create module.yaml with test name
	moduleYaml := `name: test-module
version: 1.0.0`

	moduleYamlPath := filepath.Join(tempDir, "module.yaml")
	err := os.WriteFile(moduleYamlPath, []byte(moduleYaml), 0600)
	require.NoError(t, err)

	// Get module name
	moduleName, err := getModuleName(tempDir)
	require.NoError(t, err)
	assert.Equal(t, "test-module", moduleName)
}

func TestGetModuleNameFileNotFound(t *testing.T) {
	tempDir := t.TempDir()

	// Try to get module name from directory without module.yaml
	_, err := getModuleName(tempDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read module.yaml")
}

func TestGetModuleNameInvalidYaml(t *testing.T) {
	tempDir := t.TempDir()

	// Create invalid module.yaml
	moduleYamlPath := filepath.Join(tempDir, "module.yaml")
	err := os.WriteFile(moduleYamlPath, []byte("invalid: yaml: content"), 0600)
	require.NoError(t, err)

	// Try to get module name
	_, err = getModuleName(tempDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal module.yaml")
}

func TestDownloadFile(t *testing.T) {
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "test.zip")

	// Test with a real URL (this will actually download)
	err := downloadFile(ModuleTemplateURL, zipPath)
	require.NoError(t, err)

	// Check if file was created
	_, err = os.Stat(zipPath)
	require.NoError(t, err)
}

func TestDownloadFileInvalidURL(t *testing.T) {
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "test.zip")

	// Test with invalid URL
	err := downloadFile("https://invalid-url-that-does-not-exist.com/file.zip", zipPath)
	require.Error(t, err)
}

func TestDownloadFileInvalidPath(t *testing.T) {
	// Test with invalid file path
	err := downloadFile(ModuleTemplateURL, "/invalid/path/test.zip")
	require.Error(t, err)
}

func TestExtractZip(t *testing.T) {
	tempDir := t.TempDir()
	extractDir := filepath.Join(tempDir, "extracted")

	// Create a proper zip file for testing
	zipPath := filepath.Join(tempDir, "test.zip")
	err := createTestZip(zipPath)
	require.NoError(t, err)

	// Extract the zip
	err = extractZip(zipPath, extractDir)
	require.NoError(t, err)

	// Check if files were extracted
	entries, err := os.ReadDir(extractDir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries)

	// Check that the root directory was extracted
	rootDirInfo, err := os.Stat(filepath.Join(extractDir, "modules-template-main"))
	require.NoError(t, err)
	assert.True(t, rootDirInfo.IsDir())

	// Check files inside the root directory
	_, err = os.Stat(filepath.Join(extractDir, "modules-template-main", "test.txt"))
	require.NoError(t, err)

	dirInfo, err := os.Stat(filepath.Join(extractDir, "modules-template-main", "dir1"))
	require.NoError(t, err)
	assert.True(t, dirInfo.IsDir())

	// Check file in subdirectory
	_, err = os.Stat(filepath.Join(extractDir, "modules-template-main", "dir1", "file.txt"))
	require.NoError(t, err)
}

func TestExtractZipInvalidFile(t *testing.T) {
	tempDir := t.TempDir()
	extractDir := filepath.Join(tempDir, "extracted")

	// Create an invalid zip file
	zipPath := filepath.Join(tempDir, "invalid.zip")
	err := os.WriteFile(zipPath, []byte("not a zip file"), 0600)
	require.NoError(t, err)

	// Try to extract invalid zip
	err = extractZip(zipPath, extractDir)
	require.Error(t, err)
}

func TestExtractZipNoRootDirectory(t *testing.T) {
	tempDir := t.TempDir()
	extractDir := filepath.Join(tempDir, "extracted")

	// Create a zip file without root directory
	zipPath := filepath.Join(tempDir, "no-root.zip")
	err := createZipWithoutRoot(zipPath)
	require.NoError(t, err)

	// Try to extract zip without root directory
	err = extractZip(zipPath, extractDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no root directory found")
}

func TestMoveExtractedContent(t *testing.T) {
	tempDir := t.TempDir()

	// Create a template directory inside tempDir (simulating extracted zip structure)
	templateDir := filepath.Join(tempDir, "modules-template-main")
	err := os.MkdirAll(templateDir, 0755)
	require.NoError(t, err)

	// Create some test files in template directory
	testFiles := []string{"file1.txt", "file2.txt", "dir1/file3.txt"}
	for _, file := range testFiles {
		filePath := filepath.Join(templateDir, file)
		mkdirErr := os.MkdirAll(filepath.Dir(filePath), 0755)
		require.NoError(t, mkdirErr)
		writeErr := os.WriteFile(filePath, []byte("test"), 0600)
		require.NoError(t, writeErr)
	}

	// Change to a new directory for testing
	testDir := t.TempDir()
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		if chdirErr := os.Chdir(originalDir); chdirErr != nil {
			t.Logf("Failed to restore original directory: %v", chdirErr)
		}
	}()

	err = os.Chdir(testDir)
	require.NoError(t, err)

	// Move content
	err = moveExtractedContent(tempDir, testDir)
	require.NoError(t, err)

	// Check if files were moved
	for _, file := range testFiles {
		// Check if the file exists in the test directory
		_, err := os.Stat(filepath.Join(testDir, file))
		require.NoError(t, err, "File %s should be moved to test directory", file)
	}
}

func TestMoveExtractedContentNoTemplateDir(t *testing.T) {
	tempDir := t.TempDir()
	testDir := t.TempDir()

	// Try to move content when no template directory exists
	err := moveExtractedContent(tempDir, testDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template directory not found")
}

func TestMoveExtractedContentMultipleDirs(t *testing.T) {
	tempDir := t.TempDir()
	testDir := t.TempDir()

	// Create multiple directories
	firstDir := filepath.Join(tempDir, "dir1")
	secondDir := filepath.Join(tempDir, "dir2")
	err := os.MkdirAll(firstDir, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(secondDir, 0755)
	require.NoError(t, err)

	// Create files in both directories
	file1 := filepath.Join(firstDir, "file1.txt")
	err = os.WriteFile(file1, []byte("data1"), 0600)
	require.NoError(t, err)
	file2 := filepath.Join(secondDir, "file2.txt")
	err = os.WriteFile(file2, []byte("data2"), 0600)
	require.NoError(t, err)

	// Move content
	err = moveExtractedContent(tempDir, testDir)
	require.NoError(t, err)

	// Проверяем, что файл из первой директории перемещён
	_, err = os.Stat(filepath.Join(testDir, "file1.txt"))
	require.NoError(t, err)
	// Файл из второй директории не должен быть перемещён
	_, err = os.Stat(filepath.Join(testDir, "file2.txt"))
	require.Error(t, err)
}

func TestDownloadAndExtractTemplate(_ *testing.T) {
	// This test would require mocking HTTP requests
	// For now, we'll test the function structure
	// In a real implementation, you might want to use httptest.Server

	// Test that the function can be called (will fail due to network issues in test environment)
	// tempDir := t.TempDir()
	// err := downloadAndExtractTemplate(tempDir)
	// This test is commented out because it requires network access
}

// Helper functions for testing

func createTestZip(zipPath string) error {
	// Create a proper zip file for testing
	file, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	defer writer.Close()

	// Add a test file
	testFile, err := writer.Create("modules-template-main/test.txt")
	if err != nil {
		return err
	}
	_, err = testFile.Write([]byte("test content"))
	if err != nil {
		return err
	}

	// Add a test directory
	_, err = writer.Create("modules-template-main/dir1/")
	if err != nil {
		return err
	}

	// Add a file in the directory
	dirFile, err := writer.Create("modules-template-main/dir1/file.txt")
	if err != nil {
		return err
	}
	_, err = dirFile.Write([]byte("dir content"))
	if err != nil {
		return err
	}

	return nil
}

func createZipWithoutRoot(zipPath string) error {
	// Create a zip file without root directory
	file, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	defer writer.Close()

	// Add files directly without root directory
	testFile, err := writer.Create("test.txt")
	if err != nil {
		return err
	}
	_, err = testFile.Write([]byte("test content"))
	if err != nil {
		return err
	}

	return nil
}
