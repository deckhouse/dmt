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
)

type filterFn func(string, string) bool

func GetFiles(rootPath string, skipSymlink bool, filters ...filterFn) []string {
	var result []string
	// Check if rootPath exists
	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		return result
	}
	_ = filepath.Walk(rootPath, func(path string, info os.FileInfo, _ error) error {
		if skipSymlink && info.Mode()&os.ModeSymlink != 0 {
			// Correct symlink handling: skip symlink directory, just skip symlink file
			if info.IsDir() {
				// Skip symlink directory only
				return filepath.SkipDir
			}
			// Skip symlink file
			return nil
		}

		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}

			return nil
		}

		if filterPass(rootPath, path, filters...) {
			result = append(result, path)
		}

		return nil
	})

	return result
}

func filterPass(rootPath, path string, filters ...filterFn) bool {
	if len(filters) == 0 {
		return true
	}

	for _, filter := range filters {
		if filter(rootPath, path) {
			return true
		}
	}

	return false
}
