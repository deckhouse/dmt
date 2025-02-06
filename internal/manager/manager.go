package manager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/sourcegraph/conc/pool"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters"
	"github.com/deckhouse/dmt/pkg/linters/conversions"
)

const (
	ChartConfigFilename = "Chart.yaml"
	ModuleYamlFilename  = "module.yaml"
	HooksDir            = "hooks"
	ImagesDir           = "images"
	OpenAPIDir          = "openapi"
)

type Manager struct {
	cfg     *config.RootConfig
	Linters linters.LinterList
	Modules []*module.Module

	errors *errors.LintRuleErrorsList
}

func NewManager(dirs []string, rootConfig *config.RootConfig) *Manager {
	m := &Manager{
		cfg: rootConfig,

		errors: errors.NewLintRuleErrorsList(),
	}

	// fill all linters
	m.Linters = []func(cfg *config.ModuleConfig, errList *errors.LintRuleErrorsList) linters.Linter{
		// openapi.New,
		// no_cyrillic.New,
		// license.New,
		// oss.New,
		// probes.New,
		// container.New,
		// rbacproxy.New,
		// vpa.New,
		// pdb.New,
		// crd.New,
		// images.New,
		// rbac.New,
		// monitoring.New,
		// ingress.New,
		// moduleLinter.New,
		conversions.New,
	}

	var paths []string

	for i := range dirs {
		dir, err := homedir.Expand(dirs[i])
		if err != nil {
			logger.ErrorF("Failed to expand home dir: %v", err)
			continue
		}
		result, err := getModulePaths(dir)
		if err != nil {
			logger.ErrorF("Error getting module paths: %v", err)
			continue
		}
		paths = append(paths, result...)
	}

	for i := range paths {
		moduleName := filepath.Base(paths[i])
		logger.DebugF("Found `%s` module", moduleName)
		mdl, err := module.NewModule(paths[i])
		if err != nil {
			m.errors.
				WithModule(moduleName).
				WithObjectID(paths[i]).
				WithValue(err.Error()).
				Criticalf("cannot create module `%s`", moduleName)
			continue
		}
		m.Modules = append(m.Modules, mdl)
	}

	logger.InfoF("Found %d modules", len(m.Modules))

	return m
}

func (m *Manager) Run() {
	var g = pool.New().WithMaxGoroutines(flags.LintersLimit)
	for _, module := range m.Modules {
		logger.InfoF("Run linters for `%s` module", module.GetName())

		for j := range m.Linters {
			linter := m.Linters[j](module.GetModuleConfig(), m.errors)

			g.Go(func() {
				logger.DebugF("Running linter `%s` on module `%s`", linter.Name(), module.GetName())

				linter.Run(module)
			})
		}
	}
	g.Wait()
}

func (m *Manager) PrintResult() {
	convertedError := m.errors.ConvertToError()
	if convertedError != nil {
		fmt.Printf("%s\n", convertedError)
	}
}

func (m *Manager) HasCriticalErrors() bool {
	return m.errors.ContainsCritical()
}

func isExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

// getModulePaths returns all paths with Chart.yaml
// modulesDir can be a module directory or a directory that contains helm in subdirectories.
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
		if isExistsOnFilesystem(path, ModuleYamlFilename) ||
			(isExistsOnFilesystem(path, ChartConfigFilename) &&
				(isExistsOnFilesystem(path, HooksDir) ||
					isExistsOnFilesystem(path, ImagesDir) ||
					isExistsOnFilesystem(path, OpenAPIDir))) {
			chartDirs = append(chartDirs, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return chartDirs, nil
}
