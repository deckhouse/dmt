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
	"slices"
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

	nonExistentPath := filepath.Join(rootDir, "does_not_exist")

	files = GetFiles(nonExistentPath, false)
	if len(files) != 0 {
		t.Errorf("expected no files for nonexistent path, got %d files", len(files))
	}
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

// TestGetFilesSurvivesUnstatablePath is the regression guard for a crash that took
// down whole lint runs: filepath.Walk hands the callback a nil FileInfo for a path
// it could not stat, and the callback used to dereference it. dmt itself creates
// such paths — a render injects a helper template into the module's templates/ and
// removes it again — so a linter walking that directory could hit an entry that had
// just vanished and panic the process.
//
// A directory that is readable but not traversable reproduces it deterministically:
// its children are listed, and the lstat of each one then fails.
func TestGetFilesSurvivesUnstatablePath(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the permission bits this test relies on")
	}

	root := t.TempDir()

	readable := filepath.Join(root, "readable.yaml")
	if err := os.WriteFile(readable, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	blocked := filepath.Join(root, "blocked")
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(blocked, "hidden.yaml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Readable (r) but not traversable (no x): the entry is listed, its lstat fails.
	if err := os.Chmod(blocked, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	files := GetFiles(root, false)

	if !slices.Contains(files, readable) {
		t.Errorf("GetFiles dropped the readable file: %v", files)
	}
}
