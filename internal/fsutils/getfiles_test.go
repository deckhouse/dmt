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
	_ = os.WriteFile(filepath.Join(rootDir, "dir1", "file1.txt"), []byte("test"), 0644)
	_ = os.WriteFile(filepath.Join(rootDir, "file2.txt"), []byte("test"), 0644)
	_ = os.Mkdir(filepath.Join(rootDir, ".git"), 0755)
	_ = os.WriteFile(filepath.Join(rootDir, ".git", "config"), []byte("test"), 0644)
	_ = os.Symlink(filepath.Join(rootDir, "file2.txt"), filepath.Join(rootDir, "symlink.txt"))

	files := GetFiles(rootDir, false)
	expectedFiles := []string{
		filepath.Join(rootDir, "dir1", "file1.txt"),
		filepath.Join(rootDir, "file2.txt"),
		filepath.Join(rootDir, "symlink.txt"),
	}
	assertEqualFiles(t, files, expectedFiles)

	files = GetFiles(rootDir, true)
	expectedFiles = []string{
		filepath.Join(rootDir, "dir1", "file1.txt"),
		filepath.Join(rootDir, "file2.txt"),
	}
	assertEqualFiles(t, files, expectedFiles)

	filter := func(_, path string) bool {
		return filepath.Ext(path) == ".txt"
	}
	files = GetFiles(rootDir, false, filter)
	expectedFiles = []string{
		filepath.Join(rootDir, "dir1", "file1.txt"),
		filepath.Join(rootDir, "file2.txt"),
		filepath.Join(rootDir, "symlink.txt"),
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
