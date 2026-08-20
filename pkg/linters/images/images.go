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
	"context"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/images/rules"
)

const (
	ID = "images"
)

// Images linter
type Images struct {
	name, desc string
	cfg        *pkg.ImageLinterConfig
	module     pkg.Module
	ErrorList  *errors.LintRuleErrorsList
}

func New(imageCfg *pkg.ImageLinterConfig, m pkg.Module, errorList *errors.LintRuleErrorsList) *Images {
	return &Images{
		name:      ID,
		desc:      "Lint docker images",
		cfg:       imageCfg,
		module:    m,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(imageCfg.Impact),
	}
}

func (l *Images) Lint(ctx context.Context) {
	if l.module == nil {
		return
	}

	for _, rule := range l.rules() {
		rule.Check(ctx)
	}
}

// rules builds this linter's rule set. Keeping the set as data — rather than a
// sequence of hand-written calls — is what lets rules be selected or grouped
// later without touching the rules themselves.
//
// Unlike hooks/rbac/no-cyrillic, the per-rule impact levels here are real:
// mapImageRules reads global per-rule impacts and only falls back to the
// linter's, so every rule must be scoped by its own level.
func (l *Images) rules() []pkg.Rule {
	m := l.module
	cfg := l.cfg
	errorList := l.ErrorList.WithModule(m.GetName())

	// level scopes errorList to the configured impact of a single rule.
	level := func(rule pkg.RuleConfig) *errors.LintRuleErrorsList {
		return errorList.WithMaxLevel(rule.GetLevel())
	}

	return []pkg.Rule{
		rules.NewImageRule(cfg, m, level(cfg.Rules.ImageRule)),
		rules.NewDistrolessRule(cfg, m, level(cfg.Rules.DistrolessRule)),
		rules.NewWerfRule(cfg.Werf.Disable, m, level(cfg.Rules.WerfRule)),
		rules.NewPatchesRule(cfg.Patches.Disable, m, level(cfg.Rules.PatchesRule)),
	}
}

func (l *Images) Name() string {
	return l.name
}

func (l *Images) Desc() string {
	return l.desc
}
