package manager

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	ChartConfigFilename = "Chart.yaml"
)

type Manager struct {
	Modules []*Module
}

func NewManager() *Manager {
	return &Manager{}
}

func (m Manager) LoadModules(dirs []string) []*Module {
	var paths []string

	for i := range dirs {
		dir, err := filepath.Abs(dirs[i])
		if err != nil {
			continue
		}
		result, err := getModulePaths(dir)
		if err != nil {
			continue
		}
		paths = append(paths, result...)
	}

	for i := range paths {
		module := NewModule(paths[i])
		if module.Chart == nil {
			continue
		}
		m.Modules = append(m.Modules, module)
	}

	return m.Modules
}

func isExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

// getModulePaths returns all paths with Chart.yaml
// modulesDir can be a module directory or a directory that contains modules in subdirectories.
func getModulePaths(modulesDir string) ([]string, error) {
	var chartDirs = make([]string, 0)

	// Here we find all dirs and check for Chart.yaml in them.
	err := filepath.Walk(modulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("file access '%s': %w", path, err)
		}

		// Ignore non-dirs
		if !info.IsDir() {
			return nil
		}

		// Check if first level subdirectory has a helm chart configuration file
		if isExistsOnFilesystem(path, ChartConfigFilename) {
			chartDirs = append(chartDirs, path)
		}

		// root path can be module dir, if we run one module for local testing
		// usually, root dir contains another modules and should not be ignored
		if path == modulesDir {
			return nil
		}

		return filepath.SkipDir
	})

	if err != nil {
		return nil, err
	}

	return chartDirs, nil
}
