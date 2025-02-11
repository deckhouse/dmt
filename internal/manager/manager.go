package manager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/pkg/linters/conversions"
	"github.com/deckhouse/dmt/pkg/linters/ingress"
	k8s_resources "github.com/deckhouse/dmt/pkg/linters/k8s-resources"
	moduleLinter "github.com/deckhouse/dmt/pkg/linters/module"
	"github.com/deckhouse/dmt/pkg/linters/monitoring"

	"github.com/mitchellh/go-homedir"
	"github.com/sourcegraph/conc/pool"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/container"
	"github.com/deckhouse/dmt/pkg/linters/images"
	"github.com/deckhouse/dmt/pkg/linters/license"
	no_cyrillic "github.com/deckhouse/dmt/pkg/linters/no-cyrillic"
	"github.com/deckhouse/dmt/pkg/linters/openapi"
	"github.com/deckhouse/dmt/pkg/linters/probes"
	"github.com/deckhouse/dmt/pkg/linters/rbac"
)

const (
	ChartConfigFilename = "Chart.yaml"
	ModuleYamlFilename  = "module.yaml"
	HooksDir            = "hooks"
	ImagesDir           = "images"
)

type linterFn func(*module.Module)

type Manager struct {
	Modules []*module.Module
}

// fill all linters
var funcs = []linterFn{
	openapi.Run,
	no_cyrillic.Run,
	license.Run,
	probes.Run,
	container.Run,
	k8s_resources.Run,
	images.Run,
	rbac.Run,
	monitoring.Run,
	ingress.Run,
	moduleLinter.Run,
	conversions.New(&cfg.LintersSettings.Conversions),
}

func NewManager(dirs []string) *Manager {
	m := &Manager{}

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

	lintError := errors.NewError("manager")

	for i := range paths {
		moduleName := filepath.Base(paths[i])
		logger.DebugF("Found `%s` module", moduleName)
		mdl, err := module.NewModule(paths[i])
		if err != nil {
			lintError.
				WithModule(moduleName).
				WithObjectID(paths[i]).
				WithValue(err.Error()).
				Add("cannot create module `%s`", moduleName)
			continue
		}
		m.Modules = append(m.Modules, mdl)
	}

	logger.InfoF("Found %d modules", len(m.Modules))

	return m
}

func (m *Manager) Run() {
	go func() {
		var g = pool.New().WithMaxGoroutines(flags.LintersLimit)
		for _, module := range m.Modules {
			logger.InfoF("Run linters for `%s` module", module.GetName())
			for _, fn := range funcs {
				g.Go(func() {
					// logger.DebugF("Running linter `%s` on module `%s`", m.Linters[j].Name(), m.Modules[i].GetName())
					fn(module)
				})
			}
		}
		g.Wait()
	}()
	errors.Close()
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
				(isExistsOnFilesystem(path, HooksDir) || isExistsOnFilesystem(path, ImagesDir))) {
			chartDirs = append(chartDirs, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return chartDirs, nil
}
