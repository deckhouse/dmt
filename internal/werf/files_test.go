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

package werf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupTestEnvironment(t *testing.T) (rootDir, moduleDir string, cleanup func()) {
	tempDir := t.TempDir()

	rootDirPath := filepath.Join(tempDir, "root")
	err := os.MkdirAll(rootDirPath, 0755)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("cannot create rootDir directory: %v", err)
	}

	directories := []string{
		filepath.Join(rootDirPath, "dir1"),
		filepath.Join(rootDirPath, "dir2"),
		filepath.Join(rootDirPath, "module"),
	}

	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			os.RemoveAll(tempDir)
			t.Fatalf("cannot create directory %s: %v", dir, err)
		}
	}

	testFiles := map[string]string{
		filepath.Join(rootDirPath, "test.txt"):                "test content",
		filepath.Join(rootDirPath, "dir1", "file1.txt"):       "file1 content",
		filepath.Join(rootDirPath, "dir2", "file2.txt"):       "file2 content",
		filepath.Join(rootDirPath, "werf.yaml"):               "root module yaml",
		filepath.Join(rootDirPath, "module", "werf.inc.yaml"): "submodule yaml",
	}

	for path, content := range testFiles {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			os.RemoveAll(tempDir)
			t.Fatalf("cannot create file %s: %v", path, err)
		}
	}

	cleanup = func() {
		os.RemoveAll(tempDir)
	}

	return rootDirPath, filepath.Join(rootDirPath, "module"), cleanup
}

func TestNewFiles(t *testing.T) {
	rootDir, moduleDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	f := NewFiles(rootDir, moduleDir)

	absModuleDir, _ := filepath.Abs(moduleDir)
	if f.moduleDir != absModuleDir {
		t.Errorf("moduleDir not matches: expected %s, got %s", absModuleDir, f.moduleDir)
	}
}

func TestGet(t *testing.T) {
	rootDir, moduleDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	f := NewFiles(rootDir, moduleDir)

	content := f.Get("test.txt")
	if content != "test content" {
		t.Errorf("file content not matches: expected 'test content', got '%s'", content)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Get not called panic when reading non-existent file")
		}
	}()
	_ = f.Get("non-existent.txt")
}

func TestDoGlob(t *testing.T) {
	rootDir, moduleDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	f := NewFiles(rootDir, moduleDir)

	result, err := f.doGlob("**/*.txt")
	if err != nil {
		t.Fatalf("doGlob returned error: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("incorrect number of found files: expected 3, got %d", len(result))
	}

	expectedPaths := []string{"test.txt", "dir1/file1.txt", "dir2/file2.txt"}
	for _, path := range expectedPaths {
		if _, ok := result[path]; !ok {
			t.Errorf("file %s not found in results", path)
		}
	}

	result, err = f.doGlob("modules/**/werf.inc.yaml")
	if err != nil {
		t.Fatalf("doGlob returned error: %v", err)
	}

	found := false
	for _, content := range result {
		if content == "submodule yaml" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("file werf.inc.yaml from submodule not found in results")
	}
}

func TestGlob(t *testing.T) {
	rootDir, moduleDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	f := NewFiles(rootDir, moduleDir)

	result := f.Glob("**/*.txt")
	if len(result) != 3 {
		t.Errorf("incorrect number of found files: expected 3, got %d", len(result))
	}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("glob did not call panic with incorrect pattern")
		}
	}()
	_ = f.Glob("[")
}

func TestGlobWithWerfIncYaml(t *testing.T) {
	rootDir, moduleDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	f := NewFiles(rootDir, moduleDir)

	result := f.Glob("modules/**/werf.inc.yaml")

	require.Equal(t, len(result), 1)
	t.Logf("%v", result)
	require.Equal(t, result["module/werf.inc.yaml"], "submodule yaml")

	result = f.Glob("werf.yaml")
	require.Equal(t, len(result), 1)
	require.Equal(t, result["werf.yaml"], "root module yaml")
}
