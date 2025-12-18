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
	"strings"

	"github.com/bmatcuk/doublestar"

	"github.com/deckhouse/dmt/internal/fsutils"
)

type files struct {
	rootDir   string
	moduleDir string
}

func NewFiles(rootDir, moduleDir string) files {
	moduleDir, _ = filepath.Abs(moduleDir)
	// If rootDir is not a directory, fallback to using its parent directory.
	// This ensures that rootDir always points to a valid directory.
	if !fsutils.IsDir(rootDir) {
		rootDir = filepath.Dir(rootDir)
	}
	return files{
		rootDir:   rootDir,
		moduleDir: moduleDir,
	}
}

func (f files) Get(relPath string) string {
	var res []byte
	if relPath == "base_images.yml" || relPath == "base_images.yaml" {
		// Special case for base_images.yaml, which is a file in the root directory
		// and should not be looked for in the module directory.
		return string(res)
	}

	res, err := os.ReadFile(filepath.Join(f.rootDir, relPath))
	if err != nil {
		panic(err.Error())
	}

	return string(res)
}

func (f files) doGlob(pattern string) (map[string]any, error) {
	res := make(map[string]any)
	dir := f.rootDir
	// Check if we are looking for werf.inc.yaml in the module directory
	// If so, we need to change the directory to the module directory
	// and remove the modules/* prefix from the pattern
	// This is needed because the module directory is not a direct child of the root directory
	// and the pattern should be relative to the root directory
	// Specific for Deckhouse project
	if strings.Contains(pattern, "werf.inc.yaml") {
		dir = f.moduleDir
		pattern = strings.TrimPrefix(pattern, "modules/*")
	}
	matches, err := doublestar.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return nil, err
	}
	for _, path := range matches {
		if !fsutils.IsFile(path) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		rel, _ := filepath.Rel(f.rootDir, path)
		res[rel] = string(data)
	}

	return res, nil
}

func (f files) Glob(pattern string) map[string]any {
	if res, err := f.doGlob(pattern); err != nil {
		panic(err.Error())
	} else {
		return res
	}
}

// Exists returns true if the named file exists in the Files object.
//
// This is designed to be called from a template.
//
//	{{ if .Files.Exists "foo" }}
//	{{ .Files.Get "foo" }}
//	{{ end }}
func (f files) Exists(relPath string) bool {
	fullPath := filepath.Join(f.rootDir, relPath)
	return fsutils.IsFile(fullPath)
}
