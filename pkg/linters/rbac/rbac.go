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
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
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
	tracker    *exclusions.ExclusionTracker
}

func New(cfg *config.ModuleConfig, tracker *exclusions.ExclusionTracker, errorList *errors.LintRuleErrorsList) *Rbac {
	return &Rbac{
		name:      ID,
		desc:      "Lint rbac objects",
		cfg:       &cfg.LintersSettings.Rbac,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Rbac.Impact),
		tracker:   tracker,
	}
}

func (l *Rbac) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())
	l.run(m, m.GetName(), errorList)
}

func (l *Rbac) run(m *module.Module, moduleName string, errorList *errors.LintRuleErrorsList) {
	// Register rules without exclusions in tracker if available
	if l.tracker != nil {
		l.tracker.RegisterExclusionsForModule(ID, "user-authz", []string{}, moduleName)
	}

	rules.NewUzerAuthZRule().
		ObjectUserAuthzClusterRolePath(m, errorList)

	if l.tracker != nil {
		// With tracking
		exclusions.NewTrackedRule(
			rules.NewBindingSubjectRule(l.cfg.ExcludeRules.BindingSubject.Get()),
			exclusions.StringRuleKeys(l.cfg.ExcludeRules.BindingSubject.Get()),
			l.tracker,
			ID,
			"binding-subject",
			moduleName,
		).ObjectBindingSubjectServiceAccountCheck(m, errorList)

		exclusions.NewTrackedRule(
			rules.NewPlacementRule(l.cfg.ExcludeRules.Placement.Get()),
			exclusions.KindRuleKeys(l.cfg.ExcludeRules.Placement.Get()),
			l.tracker,
			ID,
			"placement",
			moduleName,
		).ObjectRBACPlacement(m, errorList)

		exclusions.NewTrackedRule(
			rules.NewWildcardsRule(l.cfg.ExcludeRules.Wildcards.Get()),
			exclusions.KindRuleKeys(l.cfg.ExcludeRules.Wildcards.Get()),
			l.tracker,
			ID,
			"wildcards",
			moduleName,
		).ObjectRolesWildcard(m, errorList)
	} else {
		// Without tracking
		rules.NewBindingSubjectRule(l.cfg.ExcludeRules.BindingSubject.Get()).
			ObjectBindingSubjectServiceAccountCheck(m, errorList)
		rules.NewPlacementRule(l.cfg.ExcludeRules.Placement.Get()).
			ObjectRBACPlacement(m, errorList)
		rules.NewWildcardsRule(l.cfg.ExcludeRules.Wildcards.Get()).
			ObjectRolesWildcard(m, errorList)
	}
}

func (l *Rbac) Name() string {
	return l.name
}

func (l *Rbac) Desc() string {
	return l.desc
}
