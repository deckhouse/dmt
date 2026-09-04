/*
Copyright 2026 Flant JSC

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

package moduleloader

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	moduleYamlFilename = "module.yaml"
)

// GetModulePaths returns all paths that contain a module (module.yaml).
// modulesDir can be a module directory or a directory that contains helm charts in subdirectories.
func GetModulePaths(modulesDir string) ([]string, error) {
	var chartDirs = make([]string, 0)

	err := filepath.Walk(modulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("file access '%s': %w", path, err)
		}

		if !info.IsDir() {
			return nil
		}

		// A module is identified by having module.yaml
		if isExistsOnFilesystem(path, moduleYamlFilename) {
			chartDirs = append(chartDirs, path)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return chartDirs, nil
}

func isExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}
