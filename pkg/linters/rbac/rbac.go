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

package rbac

import (
	"context"

	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules"
)

const (
	ID = "rbac"
)

// Rbac linter
type Rbac struct {
	name, desc string
	cfg        *pkg.RBACLinterConfig
	module     pkg.Module
	ruleIDs    set.Set
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *pkg.RBACLinterConfig, ruleIDs set.Set, m pkg.Module, errorList *errors.LintRuleErrorsList) *Rbac {
	return &Rbac{
		name:      ID,
		desc:      "Lint rbac objects",
		cfg:       cfg,
		module:    m,
		ruleIDs:   ruleIDs,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.Impact),
	}
}

func (l *Rbac) Lint(ctx context.Context) {
	pkg.RunRules(ctx, l.ruleIDs, l.rules())
}

// rules builds this linter's rule set. Keeping the set as data — rather than a
// sequence of hand-written calls — is what lets rules be selected or grouped
// later without touching the rules themselves.
func (l *Rbac) rules() []pkg.Rule {
	m := l.module
	errorList := l.ErrorList.WithModule(m.GetName())

	// The four original rules have never applied per-rule impact levels
	// (pkg.RBACLinterConfig.Rules was populated from config but not consulted here), so
	// they keep the linter-level error list unchanged: wiring their levels up would alter
	// the severity of existing findings. The rules added for the module RBAC declaration
	// (coverage, contract, sync) do read their own level, so that they can start as
	// warnings in a tree that has not adopted the declaration yet.
	level := func(rule pkg.RuleConfig) *errors.LintRuleErrorsList {
		return errorList.WithMaxLevel(rule.GetLevel())
	}

	return []pkg.Rule{
		rules.NewUserAuthZRule(m, errorList),
		rules.NewBindingSubjectRule(l.cfg.ExcludeRules.BindingSubject.Get(), m, errorList),
		rules.NewPlacementRule(l.cfg.ExcludeRules.Placement.Get(), m, errorList),
		rules.NewWildcardsRule(l.cfg.ExcludeRules.Wildcards.Get(), m, errorList),
		rules.NewContractRule(l.cfg.ExcludeRules.Contract.Get(), m, level(l.cfg.Rules.ContractRule)),
		rules.NewCoverageRule(l.cfg.ExcludeRules.Coverage.Get(), m, level(l.cfg.Rules.CoverageRule)),
		rules.NewSyncRule(l.cfg.ExcludeRules.Sync.Get(), m, level(l.cfg.Rules.SyncRule)),
	}
}

func (l *Rbac) GetName() string {
	return l.name
}

func (l *Rbac) Desc() string {
	return l.desc
}
