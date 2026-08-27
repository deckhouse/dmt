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

package images

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/dmt/internal/modules"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

// TestAllRuleNamesCoversRuleSet keeps AllRuleNames honest about what rules() actually
// builds. It is the other half of the table check in pkg/scopes: that one proves
// the scope asks for every rule AllRuleNames lists, this one proves AllRuleNames lists
// every rule there is. Without it, a rule added to rules() and mentioned in neither place
// would stop running with both tests still green.
//
// The assertion is a subset rather than an equality on purpose: rules() may leave out
// rules whose input the module does not carry, and the module here carries nothing.
func TestAllRuleNamesCoversRuleSet(t *testing.T) {
	all := AllRuleNames()
	linter := New(&pkg.ImageLinterConfig{}, all, &modules.Module{}, errors.NewLintRuleErrorsList())

	for _, rule := range linter.rules() {
		assert.True(t, all.Has(rule.GetName()),
			"rules() builds %q, which AllRuleNames does not list", rule.GetName())
	}
}
