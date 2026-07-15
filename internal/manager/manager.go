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
	cfg *config.RootConfig

	// variants is the flattened lint work list: one entry per module in the
	// default mode, or one per discovered value combination in --matrix mode.
	// Each entry is rendered into a module.Module lazily inside Run so that only
	// worker-count modules are resident in memory at a time — in --matrix mode
	// materializing every variant up front exhausts memory on large module sets.
	variants []moduleVariant

	errors *errors.LintRuleErrorsList

	// baseVals and globalValues are the render inputs captured at init and reused
	// when each variant is rendered on demand.
	baseVals     chartutil.Values
	globalValues *spec.Schema

	// matrix enables rendering every value combination of each module
	// (see internal/matrix); matrixLimit caps the combinations per module.
	matrix      bool
	matrixLimit int
}

// moduleVariant is one unit of lint work: a module path plus the value
// overrides that select which template branches to render. Rendering is
// deferred to Run, so this struct stays cheap and many can be held at once.
type moduleVariant struct {
	path       string
	moduleName string
	// label describes the matrix combination; empty for the base variant.
	label string
	// overrides is the matrix value override tree; nil for the base variant. A
	// nil overrides marks the one variant whose render failure is a genuine
	// module defect rather than an invalid value combination.
	overrides chartutil.Values
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

	m.baseVals = vals
	m.globalValues = globalValues

	errorList := m.errors.WithLinterID("manager")

	for i := range paths {
		moduleName := filepath.Base(paths[i])
		log.Debug("Found module", slog.String("module", moduleName))

		if err := m.validateModule(paths[i]); err != nil {
			// linting errors are already logged
			continue
		}

		m.variants = append(m.variants, m.expandModuleVariants(paths[i], moduleName, errorList)...)
	}

	log.Info("Found modules", slog.Int("count", len(m.variants)))

	return m
}

// expandModuleVariants enumerates the lint work items for a single module path.
// In the default mode this is a single item rendered with the generated (plus
// --values-file) values. In --matrix mode it is one item per discovered value
// combination, so conditionally-rendered resources are reached too. Only the
// (cheap) value overrides are computed here; the actual render is deferred to
// Run so that variants do not all occupy memory at once.
func (m *Manager) expandModuleVariants(
	path, moduleName string,
	errorList *errors.LintRuleErrorsList,
) []moduleVariant {
	generated := []matrix.Variant{{}}

	if m.matrix {
		variants, err := matrix.Generate(path, "values.yaml", m.matrixLimit)
		if err != nil {
			errorList.WithFilePath(path).WithModule(moduleName).
				WithValue(err.Error()).
				Errorf("cannot expand matrix variants for module `%s`", moduleName)
		} else {
			generated = variants
			log.Info("Matrix variants for module",
				slog.String("module", moduleName), slog.Int("count", len(generated)))
		}
	}

	out := make([]moduleVariant, 0, len(generated))

	for idx := range generated {
		out = append(out, moduleVariant{
			path:       path,
			moduleName: moduleName,
			label:      generated[idx].Label,
			overrides:  generated[idx].Overrides,
		})
	}

	return out
}

// renderVariant builds the module.Module for one work item. It returns nil when
// the variant cannot be rendered: a matrix variant (non-nil overrides) that
// fails is almost always an invalid value combination the chart rejects via
// `fail` (e.g. two mutually-exclusive parameters), so it is skipped quietly.
// Only the base variant's failure is reported as a genuine "module doesn't
// build" error.
func (m *Manager) renderVariant(v moduleVariant, errorList *errors.LintRuleErrorsList) *module.Module {
	vals := mergeValues(m.baseVals, v.overrides)

	mdl, err := module.NewModule(v.path, &vals, m.globalValues, m.cfg, errorList)
	if err == nil {
		return mdl
	}

	if v.overrides != nil {
		log.Debug("skipping matrix variant that failed to render",
			slog.String("module", v.moduleName),
			slog.String("variant", v.label),
			slog.String("error", err.Error()),
		)

		return nil
	}

	errorList.
		WithFilePath(v.path).WithModule(v.moduleName).
		WithValue(err.Error()).
		Errorf("cannot create module `%s`", v.moduleName)

	return nil
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

	// liveSlots bounds how many rendered modules exist at once. A slot is taken
	// before a module is rendered and released only after it has been linted and
	// freed, so peak memory stays ~worker-count modules — not every variant at
	// once, which is what exhausts memory on large --matrix runs.
	liveSlots := make(chan struct{}, flags.LintersLimit)

	errorList := m.errors.WithLinterID("manager")

	for idx := range m.variants {
		// Acquire a slot, then render in this (single) producer goroutine.
		// Rendering stays strictly sequential on purpose: nelm's chart loader and
		// template engine keep shared, non-synchronized state, so rendering
		// several charts concurrently both races (fatal "concurrent map writes")
		// and spikes memory. Concurrency is applied to linting instead, below.
		liveSlots <- struct{}{}

		mdl := m.renderVariant(m.variants[idx], errorList)
		if mdl == nil {
			<-liveSlots
			continue
		}

		wg.Add(1)

		go func(mdl *module.Module) {
			defer func() {
				// Release the rendered objects (returning the store to the pool)
				// as soon as linting finishes, then free the slot so the producer
				// can render the next variant.
				mdl.Release()
				<-liveSlots
				wg.Done()
			}()

			log.Info("Run linters for module", slog.String("module", mdl.GetName()))

			for _, linter := range getLintersForModule(mdl.GetModuleConfig(), m.errors) {
				if flags.LinterName != "" && linter.Name() != flags.LinterName {
					continue
				}

				log.Debug("Running linter", slog.String("linter", linter.Name()), slog.String("module", mdl.GetName()))

				linter.Run(mdl)
			}
		}(mdl)
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
