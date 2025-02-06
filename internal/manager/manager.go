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
	"github.com/deckhouse/dmt/pkg/linters/container"
	"github.com/deckhouse/dmt/pkg/linters/conversions"
	"github.com/deckhouse/dmt/pkg/linters/crd-resources"
	"github.com/deckhouse/dmt/pkg/linters/images"
	"github.com/deckhouse/dmt/pkg/linters/ingress"
	rbacproxy "github.com/deckhouse/dmt/pkg/linters/kube-rbac-proxy"
	"github.com/deckhouse/dmt/pkg/linters/license"
	moduleLinter "github.com/deckhouse/dmt/pkg/linters/module"
	"github.com/deckhouse/dmt/pkg/linters/monitoring"
	no_cyrillic "github.com/deckhouse/dmt/pkg/linters/no-cyrillic"
	"github.com/deckhouse/dmt/pkg/linters/openapi"
	"github.com/deckhouse/dmt/pkg/linters/oss"
	"github.com/deckhouse/dmt/pkg/linters/pdb-resources"
	"github.com/deckhouse/dmt/pkg/linters/probes"
	"github.com/deckhouse/dmt/pkg/linters/rbac"
	"github.com/deckhouse/dmt/pkg/linters/vpa-resources"
)

const (
	ChartConfigFilename = "Chart.yaml"
	ModuleYamlFilename  = "module.yaml"
	HooksDir            = "hooks"
	ImagesDir           = "images"
	OpenAPIDir          = "openapi"
)

type Linter interface {
	Run(m *module.Module) *errors.LintRuleErrorsList
	Name() string
}

type LinterList []func() Linter

type Manager struct {
	cfg     *config.RootConfig
	Linters LinterList
	Modules []*module.Module

	errors *errors.LintRuleErrorsList
}

func NewManager(dirs []string, rootConfig *config.RootConfig) *Manager {
	m := &Manager{
		cfg: rootConfig,
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
		for _, module := range m.Modules {
			logger.InfoF("Run linters for `%s` module", module.GetName())

			for j := range getLintersForModule(module.GetModuleConfig()) {
				linter := m.Linters[j]()

				g.Go(func() {
					logger.DebugF("Running linter `%s` on module `%s`", linter.Name(), module.GetName())

					errs := linter.Run(module)
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

func getLintersForModule(cfg *config.ModuleConfig) []Linter {
	return []Linter{
		openapi.New(cfg),
		no_cyrillic.New(cfg),
		license.New(cfg),
		oss.New(cfg),
		probes.New(cfg),
		container.New(cfg),
		rbacproxy.New(cfg),
		vpa.New(cfg),
		pdb.New(cfg),
		crd.New(cfg),
		images.New(cfg),
		rbac.New(cfg),
		monitoring.New(cfg),
		ingress.New(cfg),
		moduleLinter.New(cfg),
		conversions.New(cfg),
	}
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
