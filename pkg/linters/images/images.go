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
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
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
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Images {
	imageCfg := convertToImageLinterConfig(&cfg.LintersSettings.Images)
	return &Images{
		name:      ID,
		desc:      "Lint docker images",
		cfg:       imageCfg,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(imageCfg.Impact),
	}
}

func convertToImageLinterConfig(oldCfg *config.ImageSettings) *pkg.ImageLinterConfig {
	newCfg := &pkg.ImageLinterConfig{}
	newCfg.Impact = oldCfg.Impact

	newCfg.Rules.DistrolessRule.SetLevel(oldCfg.Rules.DistrolessRule.Impact)
	newCfg.Rules.ImageRule.SetLevel(oldCfg.Rules.ImageRule.Impact)
	newCfg.Rules.PatchesRule.SetLevel(oldCfg.Rules.PatchesRule.Impact)
	newCfg.Rules.WerfRule.SetLevel(oldCfg.Rules.WerfRule.Impact)

	newCfg.Patches.Disable = oldCfg.Patches.Disable
	newCfg.Werf.Disable = oldCfg.Werf.Disable

	return newCfg
}

func (l *Images) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())

	oldCfg := l.convertToOldConfig()

	rules.NewImageRule(oldCfg).CheckImageNamesInDockerFiles(m.GetPath(), errorList.WithRule("image").WithMaxLevel(l.cfg.GetRuleImpact("image")))
	rules.NewDistrolessRule(oldCfg).CheckImageNamesInDockerFiles(m.GetPath(), errorList.WithRule("distroless").WithMaxLevel(l.cfg.GetRuleImpact("distroless")))
	rules.NewWerfRule(l.cfg.Werf.Disable).LintWerfFile(m.GetName(), m.GetWerfFile(), errorList.WithRule("werf").WithMaxLevel(l.cfg.GetRuleImpact("werf")))
	rules.NewPatchesRule(l.cfg.Patches.Disable).CheckPatches(m.GetPath(), errorList.WithRule("patches").WithMaxLevel(l.cfg.GetRuleImpact("patches")))
}

func (l *Images) convertToOldConfig() *config.ImageSettings {
	return &config.ImageSettings{
		ExcludeRules: config.ImageExcludeRules{},
		Rules: config.Rules{
			DistrolessRule: config.RuleConfig{Impact: l.cfg.Rules.DistrolessRule.GetLevel()},
			ImageRule:      config.RuleConfig{Impact: l.cfg.Rules.ImageRule.GetLevel()},
			PatchesRule:    config.RuleConfig{Impact: l.cfg.Rules.PatchesRule.GetLevel()},
			WerfRule:       config.RuleConfig{Impact: l.cfg.Rules.WerfRule.GetLevel()},
		},
		Patches: config.PatchesRuleSettings{Disable: l.cfg.Patches.Disable},
		Werf:    config.WerfRuleSettings{Disable: l.cfg.Werf.Disable},
		Impact:  l.cfg.Impact,
	}
}

func (l *Images) Name() string {
	return l.name
}

func (l *Images) Desc() string {
	return l.desc
}
