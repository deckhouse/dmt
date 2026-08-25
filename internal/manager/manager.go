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
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/kyokomi/emoji"
	"github.com/mitchellh/go-wordwrap"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/metrics"
	"github.com/deckhouse/dmt/internal/moduleloader"
	"github.com/deckhouse/dmt/internal/modules"
	"github.com/deckhouse/dmt/internal/modules/values"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/container"
	"github.com/deckhouse/dmt/pkg/linters/docs"
	"github.com/deckhouse/dmt/pkg/linters/hooks"
	"github.com/deckhouse/dmt/pkg/linters/images"
	moduleLinter "github.com/deckhouse/dmt/pkg/linters/module"
	no_cyrillic "github.com/deckhouse/dmt/pkg/linters/no-cyrillic"
	"github.com/deckhouse/dmt/pkg/linters/openapi"
	"github.com/deckhouse/dmt/pkg/linters/rbac"
	"github.com/deckhouse/dmt/pkg/linters/templates"
)

const (
	baseRepoURL = "https://github.com/deckhouse/dmt/tree/main"
)

func generateDocumentationURL(linterID, ruleID string) string {
	if linterID == "" || ruleID == "" {
		return "Not ready"
	}

	return fmt.Sprintf("%s/pkg/linters/%s#%s", baseRepoURL, linterID, ruleID)
}

// Linter is the common interface implemented by all lint passes. Everything a
// linter needs — its config, the module it inspects and the error list it
// reports into — is supplied to its constructor, so Lint takes only a context.
type Linter interface {
	GetName() string
	Lint(ctx context.Context)
}

type Manager struct {
	cfg     *config.RootConfig
	Modules []*modules.Module

	errors *errors.LintRuleErrorsList

	// startedAt marks the beginning of the run; PrintStatistics reports the
	// wall-clock time elapsed since it, matching the mirror summary's Elapsed line.
	startedAt time.Time
}

func NewManager(dir string, rootConfig *config.RootConfig) *Manager {
	managerLevel := pkg.Error
	m := &Manager{
		cfg: rootConfig,

		errors:    errors.NewLintRuleErrorsList().WithMaxLevel(&managerLevel),
		startedAt: time.Now(),
	}

	return m.initManager(dir)
}

func (m *Manager) initManager(dir string) *Manager {
	paths, err := moduleloader.GetModulePaths(dir)
	if err != nil {
		log.Error("Error getting module paths", log.Err(err))
		return m
	}

	vals, err := decodeValuesFile(flags.ValuesFile)
	if err != nil {
		log.Error("Failed to decode values file", log.Err(err))
	}

	globalValues, err := values.GetGlobalValues(getRootDirectory(dir))
	if err != nil {
		log.Error("Failed to get global values", log.Err(err))
		return m
	}

	errorList := m.errors.WithLinterID("manager")

	for i := range paths {
		moduleName := filepath.Base(paths[i])
		log.Debug("Found module", slog.String("module", moduleName))

		if err := m.validateModule(paths[i]); err != nil {
			// linting errors are already logged
			continue
		}

		mdl, err := modules.NewModule(paths[i], &vals, globalValues, m.cfg, errorList)
		if err != nil {
			errorList.
				WithFilePath(paths[i]).WithModule(moduleName).
				WithValue(err.Error()).
				Errorf("cannot create module `%s`", moduleName)

			continue
		}

		m.Modules = append(m.Modules, mdl)
	}

	log.Info("Found modules", slog.Int("count", len(m.Modules)))

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

func (m *Manager) Run(ctx context.Context) {
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

			log.Info("Run linters for module", slog.String("module", module.GetName()))

			for _, linter := range getLintersForModule(module, m.errors) {
				if flags.LinterName != "" && linter.GetName() != flags.LinterName {
					continue
				}

				log.Debug("Running linter", slog.String("linter", linter.GetName()), slog.String("module", module.GetName()))

				linter.Lint(ctx)
			}
		}()
	}

	wg.Wait()
}

func getLintersForModule(module *modules.Module, errList *errors.LintRuleErrorsList) []Linter {
	cfg := module.GetModuleConfig()

	return []Linter{
		openapi.New(&cfg.OpenAPI, module, errList),
		no_cyrillic.New(&cfg.NoCyrillic, module, errList),
		container.New(&cfg.Container, module, errList),
		templates.New(&cfg.Templates, module, errList),
		images.New(&cfg.Image, module, errList),
		rbac.New(&cfg.RBAC, module, errList),
		hooks.New(&cfg.Hooks, module, errList),
		moduleLinter.New(&cfg.Module, module, errList),
		docs.New(&cfg.Documentation, module, errList),
	}
}

func (m *Manager) PrintResult() {
	errs := m.errors.GetErrors()

	if len(errs) == 0 {
		return
	}

	slices.SortFunc(errs, func(a, b pkg.LinterError) int {
		return cmp.Or(
			cmp.Compare(a.Level, b.Level),
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

		metrics.IncDmtLinterErrorsCount(err.LinterID, err.RuleID, err.Level.String())

		if err.Level == pkg.Ignored {
			// TODO: make it not global
			if !flags.ShowIgnored {
				continue
			}

			msgColor = color.FgWhite
		}

		if err.Level == pkg.Warn {
			// TODO: make it not global
			if flags.HideWarnings {
				continue
			}

			msgColor = color.FgHiYellow
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

		if err.FixError != nil {
			fmt.Fprintf(w, "\t%s\t\t%s\n", "AutofixError:", color.New(color.FgHiYellow).Sprint(err.FixError.Error()))
		}

		if flags.ShowDocumentation {
			docURL := generateDocumentationURL(err.LinterID, err.RuleID)
			if docURL != "" {
				fmt.Fprintf(w, "\t%s\t\t%s\n", "Documentation:", docURL)
			}
		}

		fmt.Fprintln(w)

		w.Flush()
	}

	fmt.Println(buf.String())
}

func (m *Manager) HasCriticalErrors() bool {
	return m.errors.ContainsErrors()
}

// ApplyFixes is the single entry point for the --fix flag. It runs every fix
// attached to a collected finding. Findings whose fix succeeds are marked Fixed
// and subsequently dropped by GetErrors; findings whose fix fails are kept, and
// PrintResult reports the failure via the finding's FixError.
func (m *Manager) ApplyFixes() {
	for _, fix := range m.errors.GetFixes() {
		fix()
	}
}

// GetErrors returns all findings collected during the run.
// It is primarily intended for tests (e.g. the e2e framework) that need to
// assert on the structured findings produced by the linters.
func (m *Manager) GetErrors() []pkg.LinterError {
	return m.errors.GetErrors()
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
