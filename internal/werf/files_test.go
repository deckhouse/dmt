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

func setupTestEnvironment(t *testing.T) (string, string, func()) {
	tempDir := t.TempDir()

	rootDirPath := filepath.Join(tempDir, "root")
	err := os.MkdirAll(rootDirPath, 0755)
	require.NoError(t, err, "cannot create rootDir directory: %v", err)

	directories := []string{
		filepath.Join(rootDirPath, "dir1"),
		filepath.Join(rootDirPath, "dir2"),
		filepath.Join(rootDirPath, "modules"),
		filepath.Join(rootDirPath, "modules", "module"),
	}

	for _, dir := range directories {
		if err = os.MkdirAll(dir, 0755); err != nil {
			_ = os.RemoveAll(tempDir)
			t.Fatalf("cannot create directory %s: %v", dir, err)
		}
	}

	testFiles := map[string]string{
		filepath.Join(rootDirPath, "test.txt"):                           "test content",
		filepath.Join(rootDirPath, "dir1", "file1.txt"):                  "file1 content",
		filepath.Join(rootDirPath, "dir2", "file2.txt"):                  "file2 content",
		filepath.Join(rootDirPath, "werf.yaml"):                          "root module yaml",
		filepath.Join(rootDirPath, "modules", "module", "werf.inc.yaml"): "module yaml",
	}

	for path, content := range testFiles {
		if err = os.WriteFile(path, []byte(content), 0600); err != nil {
			_ = os.RemoveAll(tempDir)
			t.Fatalf("cannot create file %s: %v", path, err)
		}
	}

	// Create symlink
	if err = os.Symlink(filepath.Join(rootDirPath, "test.txt"), filepath.Join(rootDirPath, "test_symlink.txt")); err != nil {
		_ = os.RemoveAll(tempDir)
		t.Fatalf("cannot create symlink %s: %v", filepath.Join(rootDirPath, "test_symlink.txt"), err)
	}

	// Create directory
	if err = os.Mkdir(filepath.Join(rootDirPath, "dir3"), 0700); err != nil {
		_ = os.RemoveAll(tempDir)
		t.Fatalf("cannot create directory %s: %v", filepath.Join(rootDirPath, "dir3"), err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	return rootDirPath, filepath.Join(rootDirPath, "modules", "module"), cleanup
}

func TestNewFiles(t *testing.T) {
	rootDir, moduleDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	f := NewFiles(rootDir, moduleDir)

	absModuleDir, _ := filepath.Abs(moduleDir)
	require.Equal(t, f.moduleDir, absModuleDir, "moduleDir not matches: expected %s, got %s", absModuleDir, f.moduleDir)
}

func TestGet(t *testing.T) {
	rootDir, moduleDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	f := NewFiles(rootDir, moduleDir)

	content := f.Get("test.txt")
	require.Equal(t, "test content", content, "file content not matches: expected 'test content', got '%s'", content)

	content = f.Get("base_images.yml")
	require.Empty(t, content, "file content not matches: expected '', got '%s'", content)

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
	require.NoError(t, err, "doGlob returned error: %v", err)
	require.Len(t, result, 4)

	expectedPaths := []string{"test.txt", "dir1/file1.txt", "dir2/file2.txt", "test_symlink.txt"}
	require.Len(t, result, len(expectedPaths))
	for _, path := range expectedPaths {
		if _, ok := result[path]; !ok {
			t.Errorf("file %s not found in results", path)
		}
	}

	result, err = f.doGlob("modules/*/werf.inc.yaml")
	require.NoError(t, err, "doGlob returned error: %v", err)
	require.Len(t, result, 1)
	require.Equal(t, "module yaml", result["modules/module/werf.inc.yaml"], "file werf.inc.yaml from submodule not found in results")

	result, err = f.doGlob("*")
	require.NoError(t, err, "doGlob returned error: %v", err)
	require.Len(t, result, 3)
	t.Logf("%v", result)
}

func TestGlob(t *testing.T) {
	rootDir, moduleDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	f := NewFiles(rootDir, moduleDir)

	result := f.Glob("**/*.txt")
	require.Len(t, result, 4, "incorrect number of found files: expected 4, got %d", len(result))

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

	result := f.Glob("modules/*/werf.inc.yaml")

	require.Len(t, result, 1)
	require.Equal(t, "module yaml", result["modules/module/werf.inc.yaml"])

	result = f.Glob("werf.yaml")
	require.Len(t, result, 1)
	require.Equal(t, "root module yaml", result["werf.yaml"])
}
