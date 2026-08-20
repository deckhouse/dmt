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
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/stretchr/testify/require"
)

// withNoColor disables ANSI colour for the duration of the test so the rendered
// summary is asserted on its plain text, then restores the previous setting.
func withNoColor(t *testing.T) {
	t.Helper()

	orig := color.NoColor
	color.NoColor = true

	t.Cleanup(func() { color.NoColor = orig })
}

func TestRenderStatistics_Failed(t *testing.T) {
	withNoColor(t)

	out := renderStatistics(statistics{
		modules:  2,
		errors:   5,
		warnings: 3,
		ignored:  1,
		total:    9,
		byLinter: []linterStat{
			{name: "templates", count: 5},
			{name: "openapi", count: 3},
			{name: "module", count: 1},
		},
		elapsed: 250 * time.Millisecond,
	})

	// Failed state: title and the red state line reflect the error count.
	require.Contains(t, out, "Lint failed")
	require.Contains(t, out, "Lint failed; 5 problem(s) must be fixed.")

	// Severity breakdown.
	require.Contains(t, out, "Modules:    2")
	require.Contains(t, out, "Errors:     5")
	require.Contains(t, out, "Warnings:   3")
	require.Contains(t, out, "Ignored:    1")

	// Per-linter breakdown, most findings first.
	require.Contains(t, out, "By linter")
	require.Contains(t, out, "templates")
	require.Contains(t, out, "Total:      9 findings")
	require.Contains(t, out, "Elapsed: 250ms")
}

func TestRenderStatistics_WarningsOnly(t *testing.T) {
	withNoColor(t)

	out := renderStatistics(statistics{
		modules:  1,
		warnings: 4,
		total:    4,
		byLinter: []linterStat{{name: "container", count: 4}},
		elapsed:  500 * time.Millisecond,
	})

	// No errors: it is a summary, not a failure, and the state line is the
	// warnings note rather than a failure or a clean-run message.
	require.Contains(t, out, "Lint summary")
	require.NotContains(t, out, "Lint failed")
	require.Contains(t, out, "Completed with 4 warning(s).")
	require.NotContains(t, out, "No problems found.")
}

func TestRenderStatistics_Clean(t *testing.T) {
	withNoColor(t)

	out := renderStatistics(statistics{
		modules: 3,
		total:   0,
		elapsed: 42 * time.Millisecond,
	})

	require.Contains(t, out, "Lint summary")
	require.Contains(t, out, "No problems found.")
	require.Contains(t, out, "Total:      0 findings")

	// A clean run omits the per-linter breakdown entirely.
	require.NotContains(t, out, "By linter")
}

// TestRenderStatistics_FrameGeometry pins the framed box to a fixed inner width,
// matching the mirror summary: the top and bottom borders are exactly frameWidth
// runes wide, so the two tools' summaries line up.
func TestRenderStatistics_FrameGeometry(t *testing.T) {
	withNoColor(t)

	out := renderStatistics(statistics{modules: 1, errors: 1, total: 1, elapsed: time.Second})

	lines := strings.Split(strings.Trim(out, "\n"), "\n")
	require.NotEmpty(t, lines)

	top := lines[0]
	bottom := lines[len(lines)-1]

	require.True(t, strings.HasPrefix(top, "╔══ "), "top border starts the frame")
	require.True(t, strings.HasPrefix(bottom, "╚"), "bottom border closes the frame")
	require.Equal(t, frameWidth, utf8.RuneCountInString(top), "top border width")
	require.Equal(t, frameWidth, utf8.RuneCountInString(bottom), "bottom border width")
}
