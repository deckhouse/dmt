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
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"

	"dario.cat/mergo"
	"github.com/fatih/color"
	"github.com/go-openapi/spec"
	"github.com/kyokomi/emoji"
	"github.com/mitchellh/go-wordwrap"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/matrix"
	"github.com/deckhouse/dmt/internal/metrics"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/moduleloader"
	"github.com/deckhouse/dmt/internal/values"
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

type Linter interface {
	Run(m *module.Module)
	Name() string
}

type Manager struct {
	cfg     *config.RootConfig
	Modules []*module.Module

	errors *errors.LintRuleErrorsList

	// matrix enables rendering every value combination of each module
	// (see internal/matrix); matrixLimit caps the combinations per module.
	matrix      bool
	matrixLimit int
}

// Option customizes a Manager at construction time.
type Option func(*Manager)

// WithMatrix enables matrix mode with the given per-module combination limit.
// A non-positive limit falls back to the matrix package default. It is the
// programmatic equivalent of the --matrix / --matrix-limit flags and lets
// callers (e.g. tests) opt in without touching process-global flags.
func WithMatrix(enabled bool, limit int) Option {
	return func(m *Manager) {
		m.matrix = enabled
		if limit > 0 {
			m.matrixLimit = limit
		}
	}
}

func NewManager(dir string, rootConfig *config.RootConfig, opts ...Option) *Manager {
	managerLevel := pkg.Error
	m := &Manager{
		cfg: rootConfig,

		errors: errors.NewLintRuleErrorsList().WithMaxLevel(&managerLevel),

		// Default to the process-global flags so the CLI keeps working; options
		// below can override for programmatic callers.
		matrix:      flags.Matrix,
		matrixLimit: flags.MatrixLimit,
	}

	for _, opt := range opts {
		opt(m)
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

		m.Modules = append(m.Modules, m.loadModuleVariants(paths[i], moduleName, vals, globalValues, errorList)...)
	}

	log.Info("Found modules", slog.Int("count", len(m.Modules)))

	return m
}

// loadModuleVariants builds the module(s) to lint for a single path. In the
// default mode this is a single module rendered with the generated (plus
// --values-file) values. In --matrix mode it is one module per discovered value
// combination, so conditionally-rendered resources are reached too.
func (m *Manager) loadModuleVariants(
	path, moduleName string,
	baseVals chartutil.Values,
	globalValues *spec.Schema,
	errorList *errors.LintRuleErrorsList,
) []*module.Module {
	variants := []matrix.Variant{{}}

	if m.matrix {
		generated, err := matrix.Generate(path, "values.yaml", m.matrixLimit)
		if err != nil {
			errorList.WithFilePath(path).WithModule(moduleName).
				WithValue(err.Error()).
				Errorf("cannot expand matrix variants for module `%s`", moduleName)
		} else {
			variants = generated
			log.Info("Matrix variants for module",
				slog.String("module", moduleName), slog.Int("count", len(variants)))
		}
	}

	modules := make([]*module.Module, 0, len(variants))

	for idx := range variants {
		vals := mergeValues(baseVals, variants[idx].Overrides)

		mdl, err := module.NewModule(path, &vals, globalValues, m.cfg, errorList)
		if err != nil {
			errorList.
				WithFilePath(path).WithModule(moduleName).
				WithValue(err.Error()).
				Errorf("cannot create module `%s`", moduleName)

			continue
		}

		modules = append(modules, mdl)
	}

	return modules
}

// mergeValues returns a fresh value tree with override applied on top of base.
// base is treated as read-only.
func mergeValues(base, override chartutil.Values) chartutil.Values {
	out := chartutil.Values{}

	if len(base) > 0 {
		_ = mergo.Merge(&out, base, mergo.WithOverride)
	}

	if len(override) > 0 {
		_ = mergo.Merge(&out, override, mergo.WithOverride)
	}

	return out
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

			log.Info("Run linters for module", slog.String("module", module.GetName()))

			for _, linter := range getLintersForModule(module.GetModuleConfig(), m.errors) {
				if flags.LinterName != "" && linter.Name() != flags.LinterName {
					continue
				}

				log.Debug("Running linter", slog.String("linter", linter.Name()), slog.String("module", module.GetName()))

				linter.Run(module)
			}
		}()
	}

	wg.Wait()
}

func getLintersForModule(cfg *pkg.LintersSettings, errList *errors.LintRuleErrorsList) []Linter {
	return []Linter{
		openapi.New(&cfg.OpenAPI, errList),
		no_cyrillic.New(&cfg.NoCyrillic, errList),
		container.New(&cfg.Container, errList),
		templates.New(&cfg.Templates, errList),
		images.New(&cfg.Image, errList),
		rbac.New(&cfg.RBAC, errList),
		hooks.New(&cfg.Hooks, errList),
		moduleLinter.New(&cfg.Module, errList),
		docs.New(&cfg.Documentation, errList),
	}
}

func (m *Manager) PrintResult() {
	errs := m.errors.GetErrors()

	if m.matrix {
		errs = dedupeErrors(errs)
	}

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
	errs := m.errors.GetErrors()

	if m.matrix {
		return dedupeErrors(errs)
	}

	return errs
}

// dedupeErrors removes findings that are identical in every user-visible field.
// In --matrix mode the same resource is rendered and linted across many value
// combinations, so a finding that is not specific to one variant would
// otherwise be reported many times.
func dedupeErrors(errs []pkg.LinterError) []pkg.LinterError {
	seen := make(map[string]struct{}, len(errs))
	out := make([]pkg.LinterError, 0, len(errs))

	for i := range errs {
		e := errs[i]
		key := strings.Join([]string{
			e.LinterID, e.RuleID, e.ModuleID, e.ObjectID,
			e.Level.String(), e.FilePath, e.Text,
		}, "\x00")

		if _, dup := seen[key]; dup {
			continue
		}

		seen[key] = struct{}{}

		out = append(out, e)
	}

	return out
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
