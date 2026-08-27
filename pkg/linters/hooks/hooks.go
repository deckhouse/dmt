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

package hooks

import (
	"context"

	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/hooks/rules"
)

// Hooks linter
type Hooks struct {
	name, desc string
	cfg        *pkg.HooksLinterConfig
	module     pkg.Module
	ruleIDs    set.Set
	ErrorList  *errors.LintRuleErrorsList
}

const ID = "hooks"

func New(cfg *pkg.HooksLinterConfig, ruleIDs set.Set, m pkg.Module, errorList *errors.LintRuleErrorsList) *Hooks {
	return &Hooks{
		name:      ID,
		desc:      "Lint hooks",
		cfg:       cfg,
		module:    m,
		ruleIDs:   ruleIDs,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.Impact),
	}
}

func (h *Hooks) Lint(ctx context.Context) {
	pkg.RunRules(ctx, h.ruleIDs, h.rules())
}

// rules builds this linter's rule set. Keeping the set as data — rather than a
// sequence of hand-written calls — is what lets rules be selected or grouped
// later without touching the rules themselves.
func (h *Hooks) rules() []pkg.Rule {
	m := h.module

	// The rule gets the linter-level error list unchanged: cfg.Rules.HooksRule is
	// fed from the linter impact (mapSimpleLinterRules in internal/modules), which
	// New already applied, so scoping by it here would be a no-op.
	return []pkg.Rule{
		rules.NewHookRule(h.cfg, m, h.ErrorList.WithModule(m.GetName())),
	}
}

// AllRuleNames returns the IDs of every rule this linter has. It is not knowledge about
// scopes: the linter only states honestly what it carries. Checking the list against a
// scope's table is done in pkg/scopes, not here.
func AllRuleNames() set.Set {
	return set.New(
		rules.IngressRuleName,
	)
}

func (h *Hooks) GetName() string {
	return h.name
}

func (h *Hooks) Desc() string {
	return h.desc
}
