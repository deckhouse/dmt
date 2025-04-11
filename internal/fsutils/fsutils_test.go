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

package fsutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsDir(t *testing.T) {
	tempDir := t.TempDir()

	assert.True(t, IsDir(tempDir), "Expected IsDir to return true for a directory")
	assert.False(t, IsDir(filepath.Join(tempDir, "nonexistent")), "Expected IsDir to return false for a nonexistent path")
}

func TestIsFile(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "testfile.txt")

	err := os.WriteFile(tempFile, []byte("test"), 0600)
	require.NoError(t, err, "Failed to create test file")

	assert.True(t, IsFile(tempFile), "Expected IsFile to return true for a file")
	assert.False(t, IsFile(tempDir), "Expected IsFile to return false for a directory")
}

func TestGetwd(t *testing.T) {
	wd, err := Getwd()
	require.NoError(t, err, "Getwd returned an error")

	expectedWd, err := os.Getwd()
	require.NoError(t, err, "os.Getwd returned an error")
	assert.Equal(t, expectedWd, wd, "Getwd returned an unexpected working directory")
}

func TestExpandDir(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err, "Failed to get user home directory")

	expandedPath, err := ExpandDir("~/testdir")
	require.NoError(t, err, "ExpandDir returned an error")
	assert.Equal(t, filepath.Join(homeDir, "testdir"), expandedPath, "ExpandDir did not expand the path correctly")

	absPath, err := ExpandDir("/absolute/path")
	require.NoError(t, err, "ExpandDir returned an error for absolute path")
	assert.Equal(t, "/absolute/path", absPath, "ExpandDir modified an absolute path unexpectedly")
}

func TestFilterFileByExtensions(t *testing.T) {
	filter := FilterFileByExtensions(".txt", ".md")

	assert.True(t, filter("", "file.txt"), "FilterFileByExtensions did not match .txt file")
	assert.True(t, filter("", "file.md"), "FilterFileByExtensions did not match .md file")
	assert.False(t, filter("", "file.go"), "FilterFileByExtensions matched an unexpected file extension")
}
