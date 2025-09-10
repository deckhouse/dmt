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
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/hooks/rules"
)

// Hooks linter
type Hooks struct {
	name, desc string
	cfg        *config.HooksSettings
	ErrorList  *errors.LintRuleErrorsList
	moduleCfg  *config.ModuleConfig
}

const ID = "hooks"

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Hooks {
	return &Hooks{
		name:      ID,
		desc:      "Lint hooks",
		cfg:       &cfg.LintersSettings.Hooks,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Hooks.Impact),
		moduleCfg: cfg,
	}
}

func (h *Hooks) GetRuleImpact(ruleName string) *pkg.Level {
	if h.moduleCfg != nil {
		return h.moduleCfg.LintersSettings.GetRuleImpact(ID, ruleName)
	}
	return h.cfg.Impact
}

func (h *Hooks) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := h.ErrorList.WithModule(m.GetName())
	r := rules.NewHookRule(h.cfg)

	// Get rule-specific impact for ingress rule
	ruleImpact := h.GetRuleImpact("ingress")
	if ruleImpact != nil {
		errorList = errorList.WithMaxLevel(ruleImpact)
	}

	for _, object := range m.GetStorage() {
		r.CheckIngressCopyCustomCertificateRule(m, object, errorList)
	}
}

func (h *Hooks) Name() string {
	return h.name
}

func (h *Hooks) Desc() string {
	return h.desc
}
