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
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/modules"
	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/config/global"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/scopes"
)

// fakeSource hands the Manager a fixed target list, so a test can describe the shape
// of a run — how many modules, in how many scopes — without a registry or a tree.
type fakeSource struct {
	scopes  []scopes.Scope
	targets []Target
	err     error
	closed  bool
}

func (s *fakeSource) ConfigDir() string      { return "." }
func (s *fakeSource) Scopes() []scopes.Scope { return s.scopes }
func (s *fakeSource) Close()                 { s.closed = true }

func (s *fakeSource) Targets(
	_ context.Context,
	_ *config.RootConfig,
	_ *errors.LintRuleErrorsList,
	yield func(Target) bool,
) error {
	for _, t := range s.targets {
		if !yield(t) {
			break
		}
	}

	return s.err
}

// TestModuleCountCountsModulesNotTargets pins the number the summary reports. A
// remote run reads one module from two images and lints each in its own scope, so it
// has two targets and one module — the literal 1 the remote path used to pass in. The
// same-name case is the reason the count keys on the source's ModuleID and not on the
// module name: a tree can hold two directories that declare the same name.
func TestModuleCountCountsModulesNotTargets(t *testing.T) {
	cfg, err := config.NewDefaultRootConfig(t.TempDir())
	require.NoError(t, err)

	mdl := func(name string) *modules.Module {
		return modules.NewRemoteModule(t.TempDir(), name, scopes.Bundle.Settings(cfg))
	}

	for name, tc := range map[string]struct {
		targets []Target
		want    int
	}{
		"one module in two scopes": {
			targets: []Target{
				{Module: mdl("mod"), Scope: scopes.Bundle, ModuleID: "mod"},
				{Module: mdl("mod"), Scope: scopes.Release, ModuleID: "mod"},
			},
			want: 1,
		},
		"two modules in one scope": {
			targets: []Target{
				{Module: mdl("first"), Scope: scopes.Bundle, ModuleID: "a/first"},
				{Module: mdl("second"), Scope: scopes.Bundle, ModuleID: "a/second"},
			},
			want: 2,
		},
		"two directories declaring the same name": {
			targets: []Target{
				{Module: mdl("same"), Scope: scopes.Bundle, ModuleID: "a/same"},
				{Module: mdl("same"), Scope: scopes.Bundle, ModuleID: "b/same"},
			},
			want: 2,
		},
		"nothing found": {want: 0},
	} {
		t.Run(name, func(t *testing.T) {
			m := New(cfg, &fakeSource{targets: tc.targets})
			require.NoError(t, m.Run(t.Context()))

			assert.Equal(t, tc.want, m.moduleCount())
		})
	}
}

// TestMetricsSectionsFollowTheSourceScopes pins which config sections a run reports
// against: the source tree is configured by `linters-settings` and the two published
// images by `remote.bundle` and `remote.release`, and neither may be flushed under
// the other's severities.
func TestMetricsSectionsFollowTheSourceScopes(t *testing.T) {
	cfg, err := config.NewDefaultRootConfig(t.TempDir())
	require.NoError(t, err)

	static := New(cfg, &fakeSource{scopes: []scopes.Scope{scopes.Static}})
	assert.Equal(t, []*global.Linters{&cfg.GlobalSettings.Linters}, static.MetricsSections())

	remote := New(cfg, &fakeSource{scopes: []scopes.Scope{scopes.Bundle, scopes.Release}})
	assert.Equal(t, []*global.Linters{&cfg.Remote.Bundle, &cfg.Remote.Release}, remote.MetricsSections())
}

// TestRunReportsTheSourceError covers the ordering the remote path depends on: a
// registry failure comes back from Run, but the findings collected before it are
// still on the Manager for the caller to print.
func TestRunReportsTheSourceError(t *testing.T) {
	cfg, err := config.NewDefaultRootConfig(t.TempDir())
	require.NoError(t, err)

	src := &fakeSource{
		err: assert.AnError,
		targets: []Target{
			{
				Module:   modules.NewRemoteModule(t.TempDir(), "mod", scopes.Bundle.Settings(cfg)),
				Scope:    scopes.Bundle,
				ModuleID: "mod",
			},
		},
	}

	m := New(cfg, src)
	require.ErrorIs(t, m.Run(t.Context()), assert.AnError)
	assert.NotEmpty(t, m.GetErrors(), "findings collected before the failure must survive it")

	m.Close()
	assert.True(t, src.closed)
}

// TestLintModuleHonoursTheLinterFilter is the guard for the reason lintModule exists: the
// remote-lint path used to run this loop itself and silently ignored --linter, so
// `dmt lint remote --linter=<name>` reported a module clean without having checked it.
// Both paths go through this function now, so one test covers both.
//
// The bundle scope over an empty directory is the fixture: none of the files that scope
// looks for are there, so both of its linters report — module via bundle-layout,
// documentation via readme. The unfiltered case is what makes the filtered one mean
// something; without it the filter would look correct even if nothing reported at all.
func TestLintModuleHonoursTheLinterFilter(t *testing.T) {
	cfg, err := config.NewDefaultRootConfig(t.TempDir())
	require.NoError(t, err)

	// lint returns the linters that reported, which is the only thing the filter changes.
	lint := func(t *testing.T, linterName string) []string {
		t.Helper()

		// flags.LinterName is process-global, so this test must not run in parallel.
		flags.LinterName = linterName

		t.Cleanup(func() { flags.LinterName = "" })

		m := modules.NewRemoteModule(t.TempDir(), "test-module", scopes.Bundle.Settings(cfg))

		errorList := errors.NewLintRuleErrorsList()
		lintModule(t.Context(), scopes.Bundle, m, errorList)

		reported := set.New()
		for _, e := range errorList.GetErrors() {
			reported.Add(e.LinterID)
		}

		return reported.Slice()
	}

	t.Run("unfiltered", func(t *testing.T) {
		assert.ElementsMatch(t, []string{"module", "documentation"}, lint(t, ""))
	})

	t.Run("filtered", func(t *testing.T) {
		assert.Equal(t, []string{"module"}, lint(t, "module"))
	})
}

// variantSource pushes variant targets and records, for each one, how many findings
// the run had collected by the time yield handed control back.
type variantSource struct {
	targets []Target
	// atReturn[i] is the finding count observed right after yield returned for
	// target i.
	atReturn []int
}

func (s *variantSource) ConfigDir() string      { return "." }
func (s *variantSource) Scopes() []scopes.Scope { return []scopes.Scope{scopes.Bundle} }
func (s *variantSource) Close()                 {}

func (s *variantSource) Targets(
	_ context.Context,
	_ *config.RootConfig,
	errorList *errors.LintRuleErrorsList,
	yield func(Target) bool,
) error {
	for _, t := range s.targets {
		if !yield(t) {
			break
		}

		s.atReturn = append(s.atReturn, len(errorList.GetErrors()))
	}

	return nil
}

// TestVariantTargetsAreLintedBeforeTheSourceContinues pins the invariant a --matrix
// run depends on: every render of a module writes a helper template into that
// module's templates/ and removes it again, so the source must never render the next
// variant while a linter is still walking the directory. The manager enforces it by
// linting a variant target to completion before returning from yield — without which
// a matrix run crashes outright on a path that vanishes mid-walk.
//
// The finding count is the observation: the bundle scope reports on every one of
// these modules, so a target that has been linted has necessarily added findings by
// the time the source is let go.
func TestVariantTargetsAreLintedBeforeTheSourceContinues(t *testing.T) {
	cfg, err := config.NewDefaultRootConfig(t.TempDir())
	require.NoError(t, err)

	targets := make([]Target, 0, 3)
	for i := range 3 {
		targets = append(targets, Target{
			Module:   modules.NewRemoteModule(t.TempDir(), "mod", scopes.Bundle.Settings(cfg)),
			Scope:    scopes.Bundle,
			ModuleID: "mod",
			ObjectID: fmt.Sprintf("variant-%d", i),
			Variant:  true,
		})
	}

	src := &variantSource{targets: targets}

	m := New(cfg, src)
	require.NoError(t, m.Run(t.Context()))

	require.Len(t, src.atReturn, len(targets))

	for i, count := range src.atReturn {
		assert.Positive(t, count,
			"variant %d was still being linted when the source was let go to render the next one", i)

		if i > 0 {
			assert.Greater(t, count, src.atReturn[i-1],
				"variant %d added no findings before the source continued", i)
		}
	}
}

// TestPlainTargetsDoNotBlockTheSource is the other half: a run that renders each
// module once has no directory to protect, so its targets must keep linting in the
// background instead of paying the variant barrier's serialization.
func TestPlainTargetsDoNotBlockTheSource(t *testing.T) {
	cfg, err := config.NewDefaultRootConfig(t.TempDir())
	require.NoError(t, err)

	// One slot only, so a target that is linted synchronously would be finished by
	// the time yield returns, exactly as the variant case asserts above.
	flags.LintersLimit = 1

	t.Cleanup(func() { flags.LintersLimit = 0 })

	src := &variantSource{targets: []Target{{
		Module:   modules.NewRemoteModule(t.TempDir(), "mod", scopes.Bundle.Settings(cfg)),
		Scope:    scopes.Bundle,
		ModuleID: "mod",
	}}}

	m := New(cfg, src)
	require.NoError(t, m.Run(t.Context()))

	assert.NotEmpty(t, m.GetErrors(), "the target still has to be linted by the time Run returns")
}
