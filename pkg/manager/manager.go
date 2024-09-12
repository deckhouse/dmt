package manager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/deckhouse/d8-lint/pkg/errors"
	"github.com/deckhouse/d8-lint/pkg/linters/openapi"
	"github.com/deckhouse/d8-lint/pkg/module"
)

const (
	ChartConfigFilename = "Chart.yaml"
)

type Manager struct {
	Linters LinterList
	Modules module.ModuleList
}

func NewManager(dirs []string) *Manager {
	m := &Manager{}

	m.Linters = []Linter{
		openapi.New(),
	}

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
		mdl := module.NewModule(paths[i])
		if mdl.Chart == nil {
			continue
		}
		m.Modules = append(m.Modules, mdl)
	}

	return m
}

func (m *Manager) Run() errors.LintRuleErrorsList {
	result := errors.LintRuleErrorsList{}

	for i := range m.Linters {
		for j := range m.Modules {
			errs, err := m.Linters[i].Run(context.Background(), m.Modules[j])
			if err != nil {
				continue
			}
			result.Merge(errs)
		}
	}

	return result
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

		// root path can be module dir, if we run one module for local testing
		// usually, root dir contains another modules and should not be ignored
		if path == modulesDir {
			return nil
		}

		// Check if first level subdirectory has a helm chart configuration file
		if isExistsOnFilesystem(path, ChartConfigFilename) {
			chartDirs = append(chartDirs, path)
		}

		return filepath.SkipDir
	})

	if err != nil {
		return nil, err
	}

	return chartDirs, nil
}
