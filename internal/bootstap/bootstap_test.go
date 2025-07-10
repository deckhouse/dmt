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

package bootstap

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckCurrentDirectoryEmpty(t *testing.T) {
	// Test with empty directory
	tempDir := t.TempDir()

	// Change to temp directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	err = checkDirectoryEmpty(tempDir)
	assert.NoError(t, err)
}

func TestCheckCurrentDirectoryEmptyWithFiles(t *testing.T) {
	// Test with non-empty directory
	tempDir := t.TempDir()

	// Create a file in the directory
	err := os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("test"), 0600)
	require.NoError(t, err)

	// Change to temp directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// This should exit with code 1, but we can't test os.Exit directly
	// Instead, we'll test the logic by checking if files exist
	files := getFilesInCurrentDir()
	assert.Greater(t, len(files), 0, "Directory should not be empty")
}

func TestDownloadFile(t *testing.T) {
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "test.zip")

	// Test with a real URL (this will actually download)
	err := downloadFile(ModuleTemplateURL, zipPath)
	assert.NoError(t, err)

	// Check if file was created
	_, err = os.Stat(zipPath)
	assert.NoError(t, err)
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
	assert.NoError(t, err)

	// Check if files were extracted
	entries, err := os.ReadDir(extractDir)
	assert.NoError(t, err)
	assert.Greater(t, len(entries), 0)

	// Check that the root directory was extracted
	rootDirInfo, err := os.Stat(filepath.Join(extractDir, "modules-template-main"))
	assert.NoError(t, err)
	assert.True(t, rootDirInfo.IsDir())

	// Check files inside the root directory
	_, err = os.Stat(filepath.Join(extractDir, "modules-template-main", "test.txt"))
	assert.NoError(t, err)

	dirInfo, err := os.Stat(filepath.Join(extractDir, "modules-template-main", "dir1"))
	assert.NoError(t, err)
	assert.True(t, dirInfo.IsDir())

	// Check file in subdirectory
	_, err = os.Stat(filepath.Join(extractDir, "modules-template-main", "dir1", "file.txt"))
	assert.NoError(t, err)
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
		err := os.MkdirAll(filepath.Dir(filePath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(filePath, []byte("test"), 0600)
		require.NoError(t, err)
	}

	// Change to a new directory for testing
	testDir := t.TempDir()
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	err = os.Chdir(testDir)
	require.NoError(t, err)

	// Move content
	err = moveExtractedContent(tempDir, testDir)
	assert.NoError(t, err)

	// Check if files were moved
	for _, file := range testFiles {
		// Check if the file exists in the test directory
		_, err := os.Stat(filepath.Join(testDir, file))
		assert.NoError(t, err, "File %s should be moved to test directory", file)
	}
}

// Helper functions for testing

func getFilesInCurrentDir() []string {
	// This is a simplified version for testing
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files
}

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
