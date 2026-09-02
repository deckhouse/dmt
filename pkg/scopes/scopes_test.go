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

package scopes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/internal/modules"
	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// TestTablesCoverExactlyTheirLinters keeps every scope's table and its constructor
// list in step. The two failures it catches are the ones a hand-written table cannot
// survive: a set for a linter the scope does not build is dead weight, and a linter
// built with no entry in the table gets a nil set and runs nothing at all while
// reporting the module clean.
//
// Note what is deliberately *not* asserted: that a scope asks each linter for every
// rule it carries. A rule meant for a built image and not for the source tree — or
// the other way round — is exactly what scopes exist to express, so the table is the
// authority on membership and nothing checks it against a linter's full rule set.
func TestTablesCoverExactlyTheirLinters(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rules   map[string]set.Set
		linters func(*modules.Module, *errors.LintRuleErrorsList) []Linter
	}{
		{"static", staticRules, staticLinters},
		{"release", releaseRules, releaseLinters},
		{"bundle", bundleRules, bundleLinters},
	} {
		t.Run(tc.name, func(t *testing.T) {
			built := make([]string, 0, len(tc.rules))
			for _, l := range tc.linters(&modules.Module{}, errors.NewLintRuleErrorsList()) {
				built = append(built, l.GetName())
			}

			for id := range tc.rules {
				assert.Contains(t, built, id, "%sRules holds %q, which %sLinters does not build", tc.name, id, tc.name)
			}

			for _, id := range built {
				asked, ok := tc.rules[id]
				assert.True(t, ok, "%sLinters builds %q, which %sRules has no set for", tc.name, id, tc.name)
				assert.NotZero(t, asked.Size(), "%sRules asks %q for no rules, so it would run nothing", tc.name, id)
			}
		})
	}
}

// TestRemoteScopesRunOverAnUnpackedImage is the guard for the invariant the release
// and bundle tables carry: their module comes from modules.NewRemoteModule, which
// loads no chart and renders nothing, so a rule reaching for GetChart, GetObjectStore
// or GetValues panics on nil. Running both scopes over a directory is what catches a
// rule ID added to one of those tables that cannot survive there.
func TestRemoteScopesRunOverAnUnpackedImage(t *testing.T) {
	cfg, err := config.NewDefaultRootConfig(".")
	require.NoError(t, err)

	for _, tc := range []struct {
		scope   Scope
		files   []string
		dirs    []string
		wantErr bool
	}{
		{scope: Release, files: []string{"module.yaml", "version.json", "changelog.yaml"}},
		{scope: Release, wantErr: true},
		{
			scope: Bundle,
			// The root of a real published bundle — see layout_test.go for where it
			// comes from. Deriving it from bundleRules would test nothing.
			files: []string{".helmignore", "Chart.yaml", "images_digests.json", "module.yaml"},
			dirs:  []string{"charts", "docs", "openapi", "templates"},
		},
		{scope: Bundle, wantErr: true},
	} {
		name := string(tc.scope) + " complete"
		if tc.wantErr {
			name = string(tc.scope) + " empty"
		}

		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			for _, f := range tc.files {
				require.NoError(t, os.WriteFile(filepath.Join(root, f), []byte("x"), 0o600))
			}

			for _, d := range tc.dirs {
				require.NoError(t, os.Mkdir(filepath.Join(root, d), 0o755))
			}

			// The layout rules only want the paths to exist, but definition-file parses
			// module.yaml and the readme rule reads docs/README.md, so the complete
			// cases need real content in both.
			if len(tc.files) > 0 {
				require.NoError(t, os.WriteFile(filepath.Join(root, "module.yaml"),
					[]byte("name: test-module\nstage: General Availability\ndescriptions:\n  en: a module\n"), 0o600))
			}

			if len(tc.dirs) > 0 {
				require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "README.md"), []byte("x"), 0o600))
			}

			m := modules.NewRemoteModule(root, "test-module", tc.scope.Settings(cfg))

			errorList := errors.NewLintRuleErrorsList()
			for _, linter := range tc.scope.Linters(m, errorList) {
				linter.Lint(t.Context())
			}

			assert.Equal(t, tc.wantErr, errorList.ContainsErrors(), "findings: %v", errorList.GetErrors())
		})
	}
}

// TestScopeSettings pins the config layout the scopes read: `linters-settings` under
// `global` configures the source tree, `remote.bundle` and `remote.release` configure
// the two published images, and neither remote section inherits from the other two.
// The last part is what the test is really for — a scope silently falling back to the
// source-tree severities would look like a working config right up until someone
// relaxes a rule locally and finds the published images relaxed with it.
func TestScopeSettings(t *testing.T) {
	dir := t.TempDir()

	dmtlint := `
global:
  linters-settings:
    openapi:
      rules:
        bilingual:
          impact: error
    module:
      impact: ignored

linters-settings:
  openapi:
    impact: error

remote:
  release:
    openapi:
      rules:
        bilingual:
          impact: warn
  bundle:
    openapi:
      rules:
        bilingual:
          impact: ignored
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".dmtlint.yaml"), []byte(dmtlint), 0o600))

	cfg, err := config.NewDefaultRootConfig(dir)
	require.NoError(t, err)

	for _, tc := range []struct {
		scope     Scope
		bilingual string
	}{
		{Static, "error"},
		{Release, "warn"},
		{Bundle, "ignored"},
	} {
		t.Run(string(tc.scope), func(t *testing.T) {
			assert.Equal(t, tc.bilingual, tc.scope.Settings(cfg).OpenAPI.Rules.BilingualRule.Impact)
		})
	}

	// The module linter is configured for the source tree only, so a remote scope must
	// see it unset and fall back to the built-in severity rather than to `ignored`.
	assert.Empty(t, Release.Settings(cfg).Module.Impact)

	m := modules.NewRemoteModule(dir, "test-module", Release.Settings(cfg))
	assert.Equal(t, pkg.Warn, *m.GetModuleConfig().OpenAPI.Rules.BilingualRule.GetLevel())
	assert.Equal(t, pkg.Error, *m.GetModuleConfig().Module.Impact)
}
