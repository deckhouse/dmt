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
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules"
)

const (
	ID = "rbac"
)

// Rbac linter
type Rbac struct {
	name, desc string
	cfg        *config.RbacSettings
	ErrorList  *errors.LintRuleErrorsList
	moduleCfg  *config.ModuleConfig
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Rbac {
	return &Rbac{
		name:      ID,
		desc:      "Lint rbac objects",
		cfg:       &cfg.LintersSettings.Rbac,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Rbac.Impact),
		moduleCfg: cfg,
	}
}

func (l *Rbac) GetRuleImpact(ruleName string) *pkg.Level {
	if l.moduleCfg != nil {
		return l.moduleCfg.LintersSettings.GetRuleImpact(ID, ruleName)
	}
	return l.cfg.Impact
}

func (l *Rbac) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())

	// Apply rule-specific impact for each rule
	userAuthzRuleImpact := l.GetRuleImpact("user-authz")
	if userAuthzRuleImpact != nil {
		userAuthzErrorList := errorList.WithMaxLevel(userAuthzRuleImpact)
		rules.NewUzerAuthZRule().ObjectUserAuthzClusterRolePath(m, userAuthzErrorList)
	} else {
		rules.NewUzerAuthZRule().ObjectUserAuthzClusterRolePath(m, errorList)
	}

	bindingSubjectRuleImpact := l.GetRuleImpact("binding-subject")
	if bindingSubjectRuleImpact != nil {
		bindingSubjectErrorList := errorList.WithMaxLevel(bindingSubjectRuleImpact)
		rules.NewBindingSubjectRule(l.cfg.ExcludeRules.BindingSubject.Get()).ObjectBindingSubjectServiceAccountCheck(m, bindingSubjectErrorList)
	} else {
		rules.NewBindingSubjectRule(l.cfg.ExcludeRules.BindingSubject.Get()).ObjectBindingSubjectServiceAccountCheck(m, errorList)
	}

	placementRuleImpact := l.GetRuleImpact("placement")
	if placementRuleImpact != nil {
		placementErrorList := errorList.WithMaxLevel(placementRuleImpact)
		rules.NewPlacementRule(l.cfg.ExcludeRules.Placement.Get()).ObjectRBACPlacement(m, placementErrorList)
	} else {
		rules.NewPlacementRule(l.cfg.ExcludeRules.Placement.Get()).ObjectRBACPlacement(m, errorList)
	}

	wildcardsRuleImpact := l.GetRuleImpact("wildcards")
	if wildcardsRuleImpact != nil {
		wildcardsErrorList := errorList.WithMaxLevel(wildcardsRuleImpact)
		rules.NewWildcardsRule(l.cfg.ExcludeRules.Wildcards.Get()).ObjectRolesWildcard(m, wildcardsErrorList)
	} else {
		rules.NewWildcardsRule(l.cfg.ExcludeRules.Wildcards.Get()).ObjectRolesWildcard(m, errorList)
	}
}

func (l *Rbac) Name() string {
	return l.name
}

func (l *Rbac) Desc() string {
	return l.desc
}
