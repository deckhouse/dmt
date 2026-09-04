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
	// Targets loads the modules to lint. Findings made while loading go into
	// errorList; a returned error is one the run could not fold into a finding, e.g.
	// a registry failure, and is reported after the findings are printed rather than
	// instead of them.
	Targets(ctx context.Context, cfg *config.RootConfig, errorList *errors.LintRuleErrorsList) ([]Target, error)
	// Close releases what the source allocated, e.g. image extraction directories.
	// It runs after the findings are printed, so a finding must never name a path
	// that only exists until Close.
	Close()
}

type Manager struct {
	cfg     *config.RootConfig
	source  Source
	targets []Target
	errors  *errors.LintRuleErrorsList
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
		startedAt: time.Now(),
	}
}

// Run loads the source's targets and lints them. The returned error is the source's:
// the findings collected before it are still on the Manager, so the caller prints
// them first and reports the error afterwards.
func (m *Manager) Run(ctx context.Context) error {
	targets, sourceErr := m.source.Targets(ctx, m.cfg, m.errors.WithLinterID("manager"))
	m.targets = targets

	log.Info("Found modules", slog.Int("count", m.moduleCount()))

	wg := new(sync.WaitGroup)
	// The send below happens before the goroutine that drains it, so an unbuffered
	// channel deadlocks. --parallel is a user-supplied number and every caller is not
	// a cobra command, so the floor lives here rather than at each entry point.
	processingCh := make(chan struct{}, max(flags.LintersLimit, 1))

	for _, target := range targets {
		processingCh <- struct{}{}

		wg.Add(1)

		go func() {
			defer func() {
				<-processingCh
				wg.Done()
			}()

			log.Info("Run linters for module",
				slog.String("module", target.Module.GetName()),
				slog.String("scope", string(target.Scope)),
			)

			lintModule(ctx, target.Scope, target.Module, m.errorsFor(target))
		}()
	}

	wg.Wait()

	return sourceErr
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
// images, and the summary must call that one module.
func (m *Manager) moduleCount() int {
	ids := set.New()
	for _, t := range m.targets {
		ids.Add(t.ModuleID)
	}

	return ids.Size()
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
	printResult(m.errors)
}

// printResult renders a finished error list.
func printResult(errorList *errors.LintRuleErrorsList) {
	errs := errorList.GetErrors()

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
