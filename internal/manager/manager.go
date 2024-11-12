package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	k8s_resources "github.com/deckhouse/d8-lint/pkg/linters/k8s-resources"
	"github.com/deckhouse/d8-lint/pkg/linters/monitoring"

	"github.com/mitchellh/go-homedir"
	"github.com/sourcegraph/conc/pool"

	"github.com/deckhouse/d8-lint/internal/flags"
	"github.com/deckhouse/d8-lint/internal/logger"
	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/errors"
	"github.com/deckhouse/d8-lint/pkg/linters/container"
	"github.com/deckhouse/d8-lint/pkg/linters/helm"
	"github.com/deckhouse/d8-lint/pkg/linters/license"
	no_cyrillic "github.com/deckhouse/d8-lint/pkg/linters/no-cyrillic"
	"github.com/deckhouse/d8-lint/pkg/linters/openapi"
	"github.com/deckhouse/d8-lint/pkg/linters/probes"
	"github.com/deckhouse/d8-lint/pkg/linters/rbac"
)

const (
	ChartConfigFilename = "Chart.yaml"
	ModuleYamlFilename  = "module.yaml"
	HooksDir            = "hooks"
	ImagesDir           = "images"
)

type Manager struct {
	cfg     *config.Config
	Linters LinterList
	Modules []*module.Module

	lintersMap map[string]Linter
}

func NewManager(dirs []string, cfg *config.Config) *Manager {
	m := &Manager{
		cfg: cfg,
	}

	// fill all linters
	m.Linters = []Linter{
		openapi.New(&cfg.LintersSettings.OpenAPI),
		no_cyrillic.New(&cfg.LintersSettings.NoCyrillic),
		license.New(&cfg.LintersSettings.License),
		probes.New(&cfg.LintersSettings.Probes),
		container.New(&cfg.LintersSettings.Container),
		k8s_resources.New(&cfg.LintersSettings.Object),
		helm.New(&cfg.LintersSettings.Helm),
		rbac.New(&cfg.LintersSettings.Rbac),
		monitoring.New(&cfg.LintersSettings.Monitoring),
	}

	m.lintersMap = make(map[string]Linter)
	for _, linter := range m.Linters {
		m.lintersMap[strings.ToLower(linter.Name())] = linter
	}

	m.Linters = make(LinterList, 0)
	for _, linter := range m.lintersMap {
		m.Linters = append(m.Linters, linter)
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
			logger.ErrorF("Chart fill not success for module `%s`: %v", moduleName, err)
			continue
		}
		m.Modules = append(m.Modules, mdl)
	}

	logger.InfoF("Found %d modules", len(m.Modules))

	return m
}

func (m *Manager) Run() errors.LintRuleErrorsList {
	result := errors.LintRuleErrorsList{}

	var ch = make(chan errors.LintRuleErrorsList)
	go func() {
		var g = pool.New().WithMaxGoroutines(flags.LintersLimit)
		for i := range m.Modules {
			logger.InfoF("Run linters for `%s` module", m.Modules[i].GetName())
			for j := range m.Linters {
				g.Go(func() {
					logger.DebugF("Running linter `%s` on module `%s`", m.Linters[j].Name(), m.Modules[i].GetName())
					errs, err := m.Linters[j].Run(m.Modules[i])
					if err != nil {
						logger.ErrorF("Error running linter `%s`: %s\n", m.Linters[j].Name(), err)
						return
					}
					if errs.ConvertToError() != nil {
						ch <- errs
					}
				})
			}
		}
		g.Wait()
		close(ch)
	}()

	for er := range ch {
		result.Merge(er)
	}

	return result
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
