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
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fatih/color"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

// The statistics summary is rendered as a single framed block that intentionally
// mirrors the look of the deckhouse-cli `mirror` pull/push summaries: the same
// framed box, the same cyan labels padded to a fixed column, the same semantic
// accent colours, and a trailing state + Elapsed line. Keeping the primitives in
// step with that summary is what makes the two tools feel like one family.
const (
	// frameWidth is the inner width of the summary box, in runes.
	frameWidth = 56
	// labelWidth aligns the category labels in the summary body.
	labelWidth = 11
	// nameWidth left-aligns per-linter names.
	nameWidth = 30
)

// Semantic accent colours for the summary. fatih/color disables them when stdout
// is not a TTY or NO_COLOR is set, so escape codes never reach pipes or files.
//
// Apply every colour AFTER width padding (padLabel, %-30s): the codes are
// zero-width on screen but count toward fmt's field widths and break columns.
var (
	cFrame = color.New(color.FgHiBlack).SprintFunc()          // box borders - recede
	cTitle = color.New(color.FgCyan, color.Bold).SprintFunc() // block title
	cLabel = color.New(color.FgCyan).SprintFunc()             // category labels (scan anchors)
	cCount = color.New(color.Bold).SprintFunc()               // primary numbers
	cDim   = color.New(color.FgHiBlack).SprintFunc()          // units and secondary text
	cGood  = color.New(color.FgGreen).SprintFunc()            // clean run
	cWarn  = color.New(color.FgYellow).SprintFunc()           // attention (warnings)
	cBad   = color.New(color.FgRed).SprintFunc()              // failure (errors)
)

// bar returns the coloured left border of a body line.
func bar() string { return cFrame("║") }

// writeTopBorder writes the framed-block top border with the given title.
func writeTopBorder(b *strings.Builder, title string) {
	prefix := "╔══ "
	suffix := " "
	used := utf8.RuneCountInString(prefix) + utf8.RuneCountInString(title) + utf8.RuneCountInString(suffix)

	pad := max(0, frameWidth-used)

	fmt.Fprintf(b, "%s%s%s%s\n", cFrame(prefix), cTitle(title), suffix, cFrame(strings.Repeat("═", pad)))
}

// padLabel formats a category label as a fixed-width "Name:" column.
func padLabel(name string) string {
	return fmt.Sprintf("%-*s", labelWidth, name+":")
}

// formatDuration renders an elapsed duration compactly, keeping millisecond
// precision for sub-second runs (so a fast run does not report "0s").
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}

	return d.Round(time.Second).String()
}

// linterStat is one linter's contribution to the findings.
type linterStat struct {
	name  string
	count int
}

// statistics is the end-of-lint accounting handed to the renderer.
type statistics struct {
	// modules is the number of modules that were linted.
	modules int
	// critical, errors, warnings and ignored are the severity breakdown of all
	// collected findings.
	critical int
	errors   int
	warnings int
	ignored  int
	// total is the sum of all findings across every severity.
	total int
	// byLinter holds the per-linter breakdown, most findings first.
	byLinter []linterStat
	// elapsed is the wall-clock duration of the run.
	elapsed time.Duration
}

// collectStatistics tallies every collected finding by severity and by linter.
// Counts are taken over all findings regardless of the --hide-warnings /
// --show-ignored display flags: the summary is meant to give the full picture.
func collectStatistics(errorList *errors.LintRuleErrorsList, modules int, elapsed time.Duration) statistics {
	s := statistics{
		modules: modules,
		elapsed: elapsed,
	}

	perLinter := make(map[string]int)

	errs := errorList.GetErrors()
	for idx := range errs {
		s.total++

		switch errs[idx].Level {
		case pkg.Critical:
			s.critical++
		case pkg.Error:
			s.errors++
		case pkg.Warn:
			s.warnings++
		case pkg.Ignored:
			s.ignored++
		}

		perLinter[errs[idx].LinterID]++
	}

	for name, count := range perLinter {
		s.byLinter = append(s.byLinter, linterStat{name: name, count: count})
	}

	slices.SortFunc(s.byLinter, func(a, b linterStat) int {
		return cmp.Or(
			cmp.Compare(b.count, a.count),
			cmp.Compare(a.name, b.name),
		)
	})

	return s
}

// PrintStatistics prints the end-of-lint statistics as a framed summary block
// styled identically to the deckhouse-cli mirror summaries. It is meant to be
// called after PrintResult, once all findings have been listed.
func (m *Manager) PrintStatistics() {
	fmt.Println(renderStatistics(collectStatistics(m.errors, m.moduleCount(), time.Since(m.startedAt))))
}

// renderStatistics formats the statistics as a single multi-line, framed block.
//
// Example output (colour stripped):
//
//	╔══ Lint summary ═══════════════════════════════════════
//	║ Modules:    3
//	║
//	║ Critical:   0
//	║ Errors:     5
//	║ Warnings:   12
//	║ Ignored:    2
//	║
//	║ By linter:
//	║   templates                      12
//	║   openapi                        3
//	║   no-cyrillic                    2
//	║
//	║ Total:      19 findings
//	║
//	║ Lint failed; 5 problem(s) must be fixed.
//	║ Elapsed: 1.2s
//	╚═══════════════════════════════════════════════════════
//
// The title switches to "Lint failed" and the state line turns red when there
// are error/critical findings; a clean run reports "No problems found." in green.
func renderStatistics(s statistics) string {
	var b strings.Builder

	b.WriteByte('\n')

	failed := s.critical+s.errors > 0

	title := "Lint summary"
	if failed {
		title = "Lint failed"
	}

	writeTopBorder(&b, title)

	// Modules scanned.
	fmt.Fprintf(&b, "%s %s %s\n", bar(), cLabel(padLabel("Modules")), cCount(fmt.Sprint(s.modules)))
	fmt.Fprintln(&b, bar())

	// Severity breakdown. A zero count is dimmed so a clean run reads calmly and
	// the eye is drawn only to the severities that actually fired.
	writeSeverity(&b, "Critical", s.critical, cBad)
	writeSeverity(&b, "Errors", s.errors, cBad)
	writeSeverity(&b, "Warnings", s.warnings, cWarn)
	writeSeverity(&b, "Ignored", s.ignored, cDim)

	// Per-linter breakdown, most findings first. Omitted entirely on a clean run.
	if len(s.byLinter) > 0 {
		fmt.Fprintln(&b, bar())
		fmt.Fprintf(&b, "%s %s\n", bar(), cLabel("By linter"))

		for _, ls := range s.byLinter {
			fmt.Fprintf(&b, "%s   %-*s %s\n", bar(), nameWidth, ls.name, cCount(fmt.Sprint(ls.count)))
		}
	}

	fmt.Fprintln(&b, bar())
	fmt.Fprintf(&b, "%s %s %s\n", bar(), cLabel(padLabel("Total")), cCount(fmt.Sprintf("%d findings", s.total)))

	fmt.Fprintln(&b, bar())

	switch {
	case failed:
		fmt.Fprintf(&b, "%s %s\n", bar(), cBad(fmt.Sprintf("Lint failed; %d problem(s) must be fixed.", s.critical+s.errors)))
	case s.warnings > 0:
		fmt.Fprintf(&b, "%s %s\n", bar(), cWarn(fmt.Sprintf("Completed with %d warning(s).", s.warnings)))
	default:
		fmt.Fprintf(&b, "%s %s\n", bar(), cGood("No problems found."))
	}

	fmt.Fprintf(&b, "%s %s\n", bar(), cDim("Elapsed: "+formatDuration(s.elapsed)))
	b.WriteString(cFrame("╚" + strings.Repeat("═", frameWidth-1)))

	return b.String()
}

// writeSeverity renders one severity line, e.g. `║ Errors:     5`. The count is
// dimmed when zero and coloured with activeColor otherwise.
func writeSeverity(b *strings.Builder, name string, count int, activeColor func(...any) string) {
	value := activeColor(fmt.Sprint(count))
	if count == 0 {
		value = cDim(fmt.Sprint(count))
	}

	fmt.Fprintf(b, "%s %s %s\n", bar(), cLabel(padLabel(name)), value)
}
