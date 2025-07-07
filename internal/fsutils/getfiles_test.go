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
)

func TestGetFiles(t *testing.T) {
	rootDir := t.TempDir()

	_ = os.Mkdir(filepath.Join(rootDir, "dir1"), 0755)
	_ = os.WriteFile(filepath.Join(rootDir, "dir1", "file1.txt"), []byte("test"), 0600)
	_ = os.WriteFile(filepath.Join(rootDir, "file2.txt"), []byte("test"), 0600)
	_ = os.Mkdir(filepath.Join(rootDir, ".git"), 0755)
	_ = os.WriteFile(filepath.Join(rootDir, ".git", "config"), []byte("test"), 0600)
	_ = os.Symlink(filepath.Join(rootDir, "file2.txt"), filepath.Join(rootDir, "symlink.txt"))

	files, err := GetFiles(rootDir, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedFiles := []string{
		filepath.Join(rootDir, "dir1", "file1.txt"),
		filepath.Join(rootDir, "file2.txt"),
		filepath.Join(rootDir, "symlink.txt"),
	}
	assertEqualFiles(t, files, expectedFiles)

	files, err = GetFiles(rootDir, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedFiles = []string{
		filepath.Join(rootDir, "dir1", "file1.txt"),
		filepath.Join(rootDir, "file2.txt"),
	}
	assertEqualFiles(t, files, expectedFiles)

	filter := func(_, path string) bool {
		return filepath.Ext(path) == ".txt"
	}
	files, err = GetFiles(rootDir, false, filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedFiles = []string{
		filepath.Join(rootDir, "dir1", "file1.txt"),
		filepath.Join(rootDir, "file2.txt"),
		filepath.Join(rootDir, "symlink.txt"),
	}
	assertEqualFiles(t, files, expectedFiles)

	nonExistentPath := filepath.Join(rootDir, "does_not_exist")
	files, err = GetFiles(nonExistentPath, false)
	if err == nil {
		t.Error("expected error for nonexistent path, got nil")
	}
	if len(files) != 0 {
		t.Errorf("expected no files for nonexistent path, got %d files", len(files))
	}
}

func TestGetFilesWithMultipleFilters(t *testing.T) {
	rootDir := t.TempDir()

	_ = os.Mkdir(filepath.Join(rootDir, "dir1"), 0755)
	_ = os.WriteFile(filepath.Join(rootDir, "dir1", "file1.txt"), []byte("test"), 0600)
	_ = os.WriteFile(filepath.Join(rootDir, "file2.txt"), []byte("test"), 0600)
	_ = os.WriteFile(filepath.Join(rootDir, "file3.yaml"), []byte("test"), 0600)

	// Test with multiple filters (logical AND)
	txtFilter := func(_, path string) bool {
		return filepath.Ext(path) == ".txt"
	}
	yamlFilter := func(_, path string) bool {
		return filepath.Ext(path) == ".yaml"
	}

	// Should return no files since no file has both .txt and .yaml extensions
	files, err := GetFiles(rootDir, false, txtFilter, yamlFilter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected no files with conflicting filters, got %d files", len(files))
	}

	// Test with single filter
	files, err = GetFiles(rootDir, false, txtFilter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedFiles := []string{
		filepath.Join(rootDir, "dir1", "file1.txt"),
		filepath.Join(rootDir, "file2.txt"),
	}
	assertEqualFiles(t, files, expectedFiles)
}

func assertEqualFiles(t *testing.T, actual, expected []string) {
	t.Helper()

	actualMap := make(map[string]bool)
	for _, file := range actual {
		actualMap[file] = true
	}

	for _, file := range expected {
		if !actualMap[file] {
			t.Errorf("expected file %s not found in result", file)
		}
	}

	if len(actual) != len(expected) {
		t.Errorf("expected %d files, but got %d", len(expected), len(actual))
	}
}
