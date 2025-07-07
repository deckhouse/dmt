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
	"fmt"
	"os"
	"path/filepath"
)

// filterFn is a function type that filters files based on root path and file path.
// Returns true if the file should be included, false otherwise.
type filterFn func(string, string) bool

// GetFiles walks through the directory tree starting from rootPath and returns
// a slice of file paths that pass the provided filters.
//
// Parameters:
//   - rootPath: the root directory to start walking from
//   - skipSymlink: if true, symlinks will be skipped
//   - filters: optional filter functions to apply to files
//
// Returns:
//   - []string: slice of file paths that passed the filters
//   - error: any error encountered during the walk operation
func GetFiles(rootPath string, skipSymlink bool, filters ...filterFn) ([]string, error) {
	var result []string

	// Check if rootPath exists
	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		return result, fmt.Errorf("root path does not exist: %w", err)
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		// Handle errors from filepath.Walk
		if err != nil {
			return fmt.Errorf("error accessing path %s: %w", path, err)
		}

		// Skip if info is nil (should not happen with proper error handling above)
		if info == nil {
			return nil
		}

		// Skip symlinks if requested
		if skipSymlink && info.Mode()&os.ModeSymlink != 0 {
			// Correct symlink handling: skip symlink directory, just skip symlink file
			if info.IsDir() {
				// Skip symlink directory only
				return filepath.SkipDir
			}
			// Skip symlink file
			return nil
		}

		// Handle directories
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		// Apply filters to files
		if filterPass(rootPath, path, filters...) {
			result = append(result, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking directory %s: %w", rootPath, err)
	}

	return result, nil
}

// filterPass applies all provided filters to a file path.
// A file passes if ALL filters return true (logical AND).
// If no filters are provided, all files pass.
func filterPass(rootPath, path string, filters ...filterFn) bool {
	if len(filters) == 0 {
		return true
	}

	for _, filter := range filters {
		if !filter(rootPath, path) {
			return false
		}
	}

	return true
}
