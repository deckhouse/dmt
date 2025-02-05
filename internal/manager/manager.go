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
	vpa_resources "github.com/deckhouse/dmt/pkg/linters/vpa-resources"
	"github.com/deckhouse/dmt/pkg/linters/oss"

	"github.com/mitchellh/go-homedir"
	"github.com/sourcegraph/conc/pool"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
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
	OpenAPIDir          = "openapi"
)

type Manager struct {
	cfg     *config.Config
	Linters LinterList
	Modules []*module.Module

	errors *errors.LintRuleErrorsList
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
		oss.New(&cfg.LintersSettings.OSS),
		probes.New(&cfg.LintersSettings.Probes),
		container.New(&cfg.LintersSettings.Container),
		k8s_resources.New(&cfg.LintersSettings.K8SResources),
		vpa_resources.New(&cfg.LintersSettings.VPAResources),
		images.New(&cfg.LintersSettings.Images),
		rbac.New(&cfg.LintersSettings.Rbac),
		monitoring.New(&cfg.LintersSettings.Monitoring),
		ingress.New(&cfg.LintersSettings.Ingress),
		moduleLinter.New(&cfg.LintersSettings.Module),
		conversions.New(&cfg.LintersSettings.Conversions),
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

	errorsList := errors.NewLinterRuleList("manager")

	for i := range paths {
		moduleName := filepath.Base(paths[i])
		logger.DebugF("Found `%s` module", moduleName)
		mdl, err := module.NewModule(paths[i])
		if err != nil {
			errorsList.
				WithModule(moduleName).
				WithObjectID(paths[i]).
				WithValue(err.Error()).
				Add("cannot create module `%s`", moduleName)
			continue
		}
		m.Modules = append(m.Modules, mdl)
	}

	m.errors = errorsList

	logger.InfoF("Found %d modules", len(m.Modules))

	return m
}

func (m *Manager) Run() *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList("manager")

	var ch = make(chan *errors.LintRuleErrorsList)
	go func() {
		var g = pool.New().WithMaxGoroutines(flags.LintersLimit)
		for i := range m.Modules {
			logger.InfoF("Run linters for `%s` module", m.Modules[i].GetName())
			for j := range m.Linters {
				g.Go(func() {
					logger.DebugF("Running linter `%s` on module `%s`", m.Linters[j].Name(), m.Modules[i].GetName())
					errs := m.Linters[j].Run(m.Modules[i])
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

	result.Merge(m.errors)

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
