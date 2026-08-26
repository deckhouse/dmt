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
	"slices"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/kyokomi/emoji"
	"github.com/mitchellh/go-wordwrap"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/metrics"
	"github.com/deckhouse/dmt/internal/modules"
	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/config/global"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/scopes"
)

const (
	baseRepoURL = "https://github.com/deckhouse/dmt/tree/main"

	// logProgressEvery throttles the per-target "Run linters" line when a source
	// pushes many renders of one module, as --matrix does: the first render and
	// every Nth one afterwards are logged.
	logProgressEvery = 50
)

func generateDocumentationURL(linterID, ruleID string) string {
	if linterID == "" || ruleID == "" {
		return "Not ready"
	}

	return fmt.Sprintf("%s/pkg/linters/%s#%s", baseRepoURL, linterID, ruleID)
}

// Target is one unit of work: a module and the scope it is linted in. The unit is a
// pair rather than a module because a remote run reads one module from two images and
// lints each in its own scope.
type Target struct {
	Module *modules.Module
	Scope  scopes.Scope
	// ModuleID is what the source calls one module. Targets sharing it count as one
	// module in the summary, which is how a remote run reports the two images it read
	// as the single module they are. It is not the module's name: a tree can hold two
	// directories declaring the same name, and those are two modules.
	ModuleID string
	// ObjectID tags this target's findings. The remote source sets the scope name so
	// bundle and release findings stay apart in the output; empty means no tag.
	ObjectID string
	// Variant marks this target as one of several renders of the same module, which
	// is what a --matrix run produces. It turns on deduplication for the whole run:
	// a finding that does not depend on the value combination being rendered is
	// produced once per combination, and only one of those is worth printing.
	Variant bool
}

// Source supplies the modules a run lints. It is the only thing that differs between
// linting a source tree and linting the images a release published: everything after
// the modules exist — running the linters, printing, statistics, metrics — is the
// Manager's, and identical for both.
type Source interface {
	// ConfigDir is the directory .dmtlint.yaml is looked up from and the metrics
	// labels are derived from. It is read before Targets, to load the config Targets
	// is then handed.
	ConfigDir() string
	// Scopes are the scopes this source produces targets in. Answered without loading
	// anything, so a run that finds no modules still reports the sections it would
	// have linted with.
	Scopes() []scopes.Scope
	// Targets loads the modules to lint, handing each to yield as soon as it is
	// built. Targets are pushed rather than returned as a slice because a --matrix
	// run expands one module into many renders: yield blocks while the worker pool
	// is full, so the source builds the next render only once an earlier one has
	// been linted and freed, and a module that expands to thousands of renders never
	// has more than the worker count of them resident. A false from yield means the
	// run is over and the source must stop.
	//
	// Findings made while loading go into errorList; a returned error is one the run
	// could not fold into a finding, e.g. a registry failure, and is reported after
	// the findings are printed rather than instead of them.
	Targets(
		ctx context.Context,
		cfg *config.RootConfig,
		errorList *errors.LintRuleErrorsList,
		yield func(Target) bool,
	) error
	// Close releases what the source allocated, e.g. image extraction directories.
	// It runs after the findings are printed, so a finding must never name a path
	// that only exists until Close.
	Close()
}

type Manager struct {
	cfg    *config.RootConfig
	source Source
	errors *errors.LintRuleErrorsList
	// moduleIDs holds the ModuleID of every target the source pushed. Targets are
	// streamed and not kept — a --matrix run pushes far more of them than a summary
	// would want to hold — so the module count is accumulated as they arrive.
	moduleIDs set.Set
	// dedupe is set by the first variant target, and collapses findings that repeat
	// across the renders of one module. See Target.Variant.
	dedupe bool
	// startedAt marks the beginning of the run; PrintStatistics reports the
	// wall-clock time elapsed since it, matching the mirror summary's Elapsed line.
	// It is taken before the source loads anything, so for a remote run the pulls
	// are inside the number the caller waited for.
	startedAt time.Time
}

func New(cfg *config.RootConfig, src Source) *Manager {
	managerLevel := pkg.Error

	return &Manager{
		cfg:       cfg,
		source:    src,
		errors:    errors.NewLintRuleErrorsList().WithMaxLevel(&managerLevel),
		moduleIDs: set.New(),
		startedAt: time.Now(),
	}
}

// Run lints the targets the source pushes. The returned error is the source's: the
// findings collected before it are still on the Manager, so the caller prints them
// first and reports the error afterwards.
func (m *Manager) Run(ctx context.Context) error {
	wg := new(sync.WaitGroup)
	// The send below happens before the goroutine that drains it, so an unbuffered
	// channel deadlocks. --parallel is a user-supplied number and every caller is not
	// a cobra command, so the floor lives here rather than at each entry point.
	//
	// The channel doubles as the bound on how many rendered modules are alive at
	// once: a slot is taken before the source is let go to build the next target and
	// released only once this one has been linted and freed. That is what keeps a
	// --matrix run, which renders a module under thousands of value combinations,
	// from holding more than the worker count of them.
	processingCh := make(chan struct{}, max(flags.LintersLimit, 1))

	var prog progress

	sourceErr := m.source.Targets(ctx, m.cfg, m.errors.WithLinterID("manager"), func(target Target) bool {
		processingCh <- struct{}{}

		m.moduleIDs.Add(target.ModuleID)
		m.dedupe = m.dedupe || target.Variant

		prog.start(target)

		wg.Add(1)

		go func() {
			defer func() {
				// Hand the rendered objects back to the pool as soon as this target
				// is linted, then free the slot: the source is waiting on it to
				// render the next one.
				target.Module.Release()
				<-processingCh
				wg.Done()
			}()

			lintModule(ctx, target.Scope, target.Module, m.errorsFor(target))
		}()

		return true
	})

	prog.done()

	wg.Wait()

	return sourceErr
}

// progress throttles the per-target "Run linters" line. A --matrix run renders one
// module under many value combinations, each of which would otherwise log the same
// line; the renders of a module arrive consecutively, so counting the current run of
// them is enough to log the first, every logProgressEvery-th, and the last. Only the
// goroutine the source pushes targets from touches it, so it needs no locking.
type progress struct {
	target Target
	count  int
	// logged is the count the last line reported, so done can tell whether the final
	// render of a module has already been announced.
	logged int
}

// start announces that target is about to be linted, unless the throttle swallows
// the line.
func (p *progress) start(t Target) {
	// A module linted in two scopes — the two images of a remote run — is two
	// streams, not a repeated render, and each is announced.
	if t.ModuleID != p.target.ModuleID || t.Scope != p.target.Scope {
		p.done()

		p.count, p.logged = 0, 0
	}

	p.target = t
	p.count++

	if p.count == 1 || p.count%logProgressEvery == 0 {
		p.log()
	}
}

// done reports where the stream the throttle was counting finished, unless its last
// render was already logged. Call it once the source has pushed everything.
func (p *progress) done() {
	if p.count > 1 && p.logged != p.count {
		p.log()
	}
}

func (p *progress) log() {
	p.logged = p.count

	attrs := []any{
		slog.String("module", p.target.Module.GetName()),
		slog.String("scope", string(p.target.Scope)),
	}

	// Only a stream of several renders needs a counter; a plain run has one per
	// module and the field would be noise.
	if p.count > 1 {
		attrs = append(attrs, slog.Int("render", p.count))
	}

	log.Info("Run linters for module", attrs...)
}

// Close releases the source's resources. It must be called after the findings are
// printed: a source may be holding the directory the run linted.
func (m *Manager) Close() {
	m.source.Close()
}

// errorsFor decorates the shared error list for one target.
func (m *Manager) errorsFor(t Target) *errors.LintRuleErrorsList {
	if t.ObjectID == "" {
		return m.errors
	}

	return m.errors.WithObjectID(t.ObjectID)
}

// moduleCount counts modules, not targets: a remote run reads one module from two
// images and a --matrix run renders one module many times, and the summary must call
// either of those one module.
func (m *Manager) moduleCount() int {
	return m.moduleIDs.Size()
}

// MetricsSections returns the config sections this run linted with, in the form
// metrics.Flush takes. They follow from the source's scopes — `linters-settings` for
// a source tree, `remote.bundle` and `remote.release` for the published images — so
// the caller does not have to know which run it started.
func (m *Manager) MetricsSections() []*global.Linters {
	sc := m.source.Scopes()
	sections := make([]*global.Linters, 0, len(sc))

	for _, s := range sc {
		sections = append(sections, s.Settings(m.cfg))
	}

	return sections
}

// lintModule runs the scope's linters over one module.
func lintModule(ctx context.Context, sc scopes.Scope, m *modules.Module, errorList *errors.LintRuleErrorsList) {
	for _, linter := range sc.Linters(m, errorList) {
		if flags.LinterName != "" && linter.GetName() != flags.LinterName {
			continue
		}

		log.Debug("Running linter",
			slog.String("linter", linter.GetName()),
			slog.String("module", m.GetName()),
		)

		linter.Lint(ctx)
	}
}

func (m *Manager) PrintResult() {
	printResult(m.GetErrors())
}

// printResult renders a finished set of findings.
func printResult(errs []pkg.LinterError) {
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
			// Render a VSCode-clickable "path:line" reference when the line is known,
			// so the separate LineNumber line is not needed.
			filePath := strings.TrimSpace(err.FilePath)
			if err.LineNumber != 0 {
				filePath = fmt.Sprintf("%s:%d", filePath, err.LineNumber)
			}

			fmt.Fprintf(w, "\t%s\t\t%s\n", "FilePath:", filePath)
		} else if err.LineNumber != 0 {
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

	if m.dedupe {
		return dedupeErrors(errs)
	}

	return errs
}

// dedupeErrors removes findings that are identical in every user-visible field. It
// runs only for a run that rendered some module more than once (see Target.Variant):
// the same resource is linted under many value combinations, so a finding that is
// not specific to one of them would otherwise be reported once per combination.
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
