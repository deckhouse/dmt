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

package manager

import (
	"bytes"
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/go-openapi/spec"
	"github.com/kyokomi/emoji"
	"github.com/mitchellh/go-wordwrap"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/metrics"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/values"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/container"
	"github.com/deckhouse/dmt/pkg/linters/hooks"
	"github.com/deckhouse/dmt/pkg/linters/images"
	moduleLinter "github.com/deckhouse/dmt/pkg/linters/module"
	no_cyrillic "github.com/deckhouse/dmt/pkg/linters/no-cyrillic"
	"github.com/deckhouse/dmt/pkg/linters/openapi"
	"github.com/deckhouse/dmt/pkg/linters/rbac"
	"github.com/deckhouse/dmt/pkg/linters/templates"
)

const (
	ChartConfigFilename = "Chart.yaml"
	ModuleYamlFilename  = "module.yaml"
	HooksDir            = "hooks"
	ImagesDir           = "images"
	OpenAPIDir          = "openapi"
)

type Linter interface {
	Run(m *module.Module)
	Name() string
}

type Manager struct {
	cfg     *config.RootConfig
	Modules []*module.Module

	errors *errors.LintRuleErrorsList
}

func NewManager(dir string, rootConfig *config.RootConfig) *Manager {
	managerLevel := pkg.Error
	m := &Manager{
		cfg: rootConfig,

		errors: errors.NewLintRuleErrorsList().WithMaxLevel(&managerLevel),
	}

	paths, err := getModulePaths(dir)
	if err != nil {
		logger.ErrorF("Error getting module paths: %v", err)
		return m
	}

	vals, err := decodeValuesFile(flags.ValuesFile)
	if err != nil {
		logger.ErrorF("Failed to decode values file: %v", err)
	}

	globalValues, err := values.GetGlobalValues(getRootDirectory(dir))
	var globalSchema spec.Schema
	if err == nil && globalValues != nil {
		globalSchema = *globalValues
	}
	globalSchema.Default = make(map[string]any)

	for i := range paths {
		moduleName := filepath.Base(paths[i])
		logger.DebugF("Found `%s` module", moduleName)
		mdl, err := module.NewModule(paths[i], &vals, globalValues)
		if err != nil {
			m.errors.
				WithLinterID("!manager").
				WithModule(moduleName).
				WithFilePath(paths[i]).
				WithValue(err.Error()).
				Errorf("cannot create module `%s`", moduleName)
			continue
		}

		mdl.MergeRootConfig(rootConfig)

		m.Modules = append(m.Modules, mdl)
	}

	logger.InfoF("Found %d modules", len(m.Modules))

	return m
}

func decodeValuesFile(path string) (chartutil.Values, error) {
	if path == "" {
		return nil, nil
	}

	valuesFile, err := fsutils.ExpandDir(path)
	if err != nil {
		return nil, err
	}

	return chartutil.ReadValuesFile(valuesFile)
}

func (m *Manager) Run() {
	wg := new(sync.WaitGroup)
	processingCh := make(chan struct{}, flags.LintersLimit)

	for _, module := range m.Modules {
		processingCh <- struct{}{}
		wg.Add(1)

		go func() {
			defer func() {
				<-processingCh
				wg.Done()
			}()

			logger.InfoF("Run linters for `%s` module", module.GetName())

			for _, linter := range getLintersForModule(module.GetModuleConfig(), m.errors) {
				if flags.LinterName != "" && linter.Name() != flags.LinterName {
					continue
				}

				logger.DebugF("Running linter `%s` on module `%s`", linter.Name(), module.GetName())

				linter.Run(module)
			}
		}()
	}

	wg.Wait()
}

func getLintersForModule(cfg *config.ModuleConfig, errList *errors.LintRuleErrorsList) []Linter {
	return []Linter{
		openapi.New(cfg, errList),
		no_cyrillic.New(cfg, errList),
		container.New(cfg, errList),
		templates.New(cfg, errList),
		images.New(cfg, errList),
		rbac.New(cfg, errList),
		hooks.New(cfg, errList),
		moduleLinter.New(cfg, errList),
	}
}

func (m *Manager) PrintResult() {
	errs := m.errors.GetErrors()

	if len(errs) == 0 {
		return
	}

	slices.SortFunc(errs, func(a, b pkg.LinterError) int {
		return cmp.Or(
			cmp.Compare(a.ModuleID, b.ModuleID),
			cmp.Compare(a.LinterID, b.LinterID),
			cmp.Compare(a.RuleID, b.RuleID),
		)
	})

	w := new(tabwriter.Writer)

	const minWidth = 5

	buf := bytes.NewBuffer([]byte{})
	w.Init(buf, minWidth, 0, 0, ' ', 0)

	for idx := range errs {
		err := errs[idx]

		msgColor := color.FgRed

		if err.Level == pkg.Warn {
			msgColor = color.FgHiYellow
			metrics.IncLinterWarning(err.LinterID, err.RuleID)
		}

		// header
		fmt.Fprint(w, emoji.Sprintf(":monkey:"))
		fmt.Fprint(w, color.New(color.FgHiBlue).SprintFunc()("["))

		if err.RuleID != "" {
			fmt.Fprint(w, color.New(color.FgHiBlue).SprintFunc()(err.RuleID+" "))
		}

		fmt.Fprintf(w, "%s\n", color.New(color.FgHiBlue).SprintfFunc()("(#%s)]", err.LinterID))

		// body
		fmt.Fprintf(w, "\t%s\t\t%s\n", "Message:", color.New(msgColor).SprintfFunc()(prepareString(err.Text)))

		fmt.Fprintf(w, "\t%s\t\t%s\n", "Module:", err.ModuleID)

		if err.ObjectID != "" && err.ObjectID != err.ModuleID {
			fmt.Fprintf(w, "\t%s\t\t%s\n", "Object:", err.ObjectID)
		}

		if err.ObjectValue != nil {
			value := fmt.Sprintf("%v", err.ObjectValue)

			fmt.Fprintf(w, "\t%s\t\t%s\n", "Value:", prepareString(value))
		}

		if err.FilePath != "" {
			fmt.Fprintf(w, "\t%s\t\t%s\n", "FilePath:", strings.TrimSpace(err.FilePath))
		}

		if err.LineNumber != 0 {
			fmt.Fprintf(w, "\t%s\t\t%d\n", "LineNumber:", err.LineNumber)
		}

		fmt.Fprintln(w)

		w.Flush()
	}

	fmt.Println(buf.String())
}

func (m *Manager) HasCriticalErrors() bool {
	return m.errors.ContainsErrors()
}

func (m *Manager) GetLinterWarningsCountLabels() map[string]map[string]struct{} {
	result := make(map[string]map[string]struct{})
	for i := range m.errors.GetErrors() {
		err := m.errors.GetErrors()[i]
		if err.Level != pkg.Warn {
			continue
		}
		if _, ok := result[err.LinterID]; !ok {
			result[err.LinterID] = make(map[string]struct{})
		}
		result[err.LinterID][err.RuleID] = struct{}{}
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

// prepareString handle ussual string and prepare it for tablewriter
func prepareString(input string) string {
	// magic wrap const
	const wrapLen = 100

	w := &strings.Builder{}

	// split wraps for tablewrite
	split := strings.Split(wordwrap.WrapString(input, wrapLen), "\n")

	// first string must be pure for correct handling
	fmt.Fprint(w, strings.TrimSpace(split[0]))

	for i := 1; i < len(split); i++ {
		fmt.Fprintf(w, "\n\t\t\t%s", strings.TrimSpace(split[i]))
	}

	return w.String()
}

func getRootDirectory(dir string) string {
	for {
		if fsutils.IsDir(filepath.Join(dir, "global-hooks", "openapi")) &&
			fsutils.IsDir(filepath.Join(dir, "modules")) &&
			fsutils.IsFile(filepath.Join(dir, "global-hooks", "openapi", "config-values.yaml")) &&
			fsutils.IsFile(filepath.Join(dir, "global-hooks", "openapi", "values.yaml")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if dir == parent || parent == "" {
			break
		}

		dir = parent
	}

	return ""
}
