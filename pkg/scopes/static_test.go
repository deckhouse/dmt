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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/dmt/internal/modules"
	"github.com/deckhouse/dmt/pkg/errors"
)

// TestStaticTableCoversExactlyItsLinters keeps the table and the constructor list in step.
// The two failures it catches are the ones the table cannot survive: a set for a linter
// static does not build is dead weight, and a linter built with no entry in the table gets
// a nil set and runs nothing at all while reporting the module clean.
//
// Note what is deliberately *not* asserted: that static asks each linter for every rule it
// carries. That would only hold while static is the only scope. A rule meant for a built
// image and not for the source tree is exactly what scopes exist to express, so the table
// is the authority on membership and nothing checks it against a linter's full rule set.
func TestStaticTableCoversExactlyItsLinters(t *testing.T) {
	built := make([]string, 0, len(staticRules))
	for _, l := range staticLinters(&modules.Module{}, errors.NewLintRuleErrorsList()) {
		built = append(built, l.GetName())
	}

	for id := range staticRules {
		assert.Contains(t, built, id, "staticRules holds %q, which staticLinters does not build", id)
	}

	for _, id := range built {
		asked, ok := staticRules[id]
		assert.True(t, ok, "staticLinters builds %q, which staticRules has no set for", id)
		assert.NotZero(t, asked.Size(), "staticRules asks %q for no rules, so it would run nothing", id)
	}
}
