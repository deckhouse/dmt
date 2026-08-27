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
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/internal/modules"
	"github.com/deckhouse/dmt/internal/set"
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

// allLinterRules pairs each linter with the rules it carries, as the linter itself
// reports them. It is the reference the scope tables are checked against.
var allLinterRules = map[string]set.Set{
	container.ID:    container.AllRuleNames(),
	docs.ID:         docs.AllRuleNames(),
	hooks.ID:        hooks.AllRuleNames(),
	images.ID:       images.AllRuleNames(),
	moduleLinter.ID: moduleLinter.AllRuleNames(),
	no_cyrillic.ID:  no_cyrillic.AllRuleNames(),
	openapi.ID:      openapi.AllRuleNames(),
	rbac.ID:         rbac.AllRuleNames(),
	templates.ID:    templates.AllRuleNames(),
}

// TestStaticAsksForEveryRule is the guard that makes an explicit allowlist safe. static
// lints the full source tree, so it asks every linter for all of its rules; a rule added
// to a linter and forgotten in staticRules would otherwise stop running silently, which
// reads as a clean module rather than as a missing check.
func TestStaticAsksForEveryRule(t *testing.T) {
	for id, all := range allLinterRules {
		t.Run(id, func(t *testing.T) {
			asked, ok := staticRules[id]
			require.True(t, ok, "linter %q has no rule set in staticRules", id)

			assert.Empty(t, missingFrom(all, asked),
				"rules the %s linter has that static never asks for", id)
			assert.Empty(t, missingFrom(asked, all),
				"rule IDs static asks %s for that no rule of it carries", id)
		})
	}
}

// TestStaticTableCoversExactlyItsLinters keeps the table and the constructor list in step:
// a set for a linter static does not build would never be read, and a linter built with a
// set the table does not hold would run nothing at all.
func TestStaticTableCoversExactlyItsLinters(t *testing.T) {
	built := make([]string, 0, len(allLinterRules))
	for _, l := range staticLinters(&modules.Module{}, errors.NewLintRuleErrorsList()) {
		built = append(built, l.GetName())
	}

	for id := range staticRules {
		assert.Contains(t, built, id, "staticRules holds %q, which staticLinters does not build", id)
	}

	for _, id := range built {
		_, ok := staticRules[id]
		assert.True(t, ok, "staticLinters builds %q, which staticRules has no set for", id)
	}
}

// missingFrom returns the members of want that have are absent from got, sorted so the
// failure message is stable.
func missingFrom(want, got set.Set) []string {
	var missing []string

	for name := range want {
		if !got.Has(name) {
			missing = append(missing, name)
		}
	}

	slices.Sort(missing)

	return missing
}
