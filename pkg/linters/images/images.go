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
	cfg        *config.ImageSettings
	ErrorList  *errors.LintRuleErrorsList
	moduleCfg  *config.ModuleConfig
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Images {
	return &Images{
		name:      ID,
		desc:      "Lint docker images",
		cfg:       &cfg.LintersSettings.Images,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Images.Impact),
		moduleCfg: cfg,
	}
}

func (l *Images) GetRuleImpact(ruleName string) *pkg.Level {
	if l.moduleCfg != nil {
		return l.moduleCfg.LintersSettings.GetRuleImpact(ID, ruleName)
	}
	return l.cfg.Impact
}

func (l *Images) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())

	// Apply rule-specific impact for each rule
	imageRuleImpact := l.GetRuleImpact("dockerfile")
	if imageRuleImpact != nil {
		imageErrorList := errorList.WithMaxLevel(imageRuleImpact)
		rules.NewImageRule(l.cfg).CheckImageNamesInDockerFiles(m.GetPath(), imageErrorList)
	} else {
		rules.NewImageRule(l.cfg).CheckImageNamesInDockerFiles(m.GetPath(), errorList)
	}

	distrolessRuleImpact := l.GetRuleImpact("distroless")
	if distrolessRuleImpact != nil {
		distrolessErrorList := errorList.WithMaxLevel(distrolessRuleImpact)
		rules.NewDistrolessRule(l.cfg).CheckImageNamesInDockerFiles(m.GetPath(), distrolessErrorList)
	} else {
		rules.NewDistrolessRule(l.cfg).CheckImageNamesInDockerFiles(m.GetPath(), errorList)
	}

	werfRuleImpact := l.GetRuleImpact("werf")
	if werfRuleImpact != nil {
		werfErrorList := errorList.WithMaxLevel(werfRuleImpact)
		rules.NewWerfRule(l.cfg.Werf.Disable).LintWerfFile(m.GetName(), m.GetWerfFile(), werfErrorList)
	} else {
		rules.NewWerfRule(l.cfg.Werf.Disable).LintWerfFile(m.GetName(), m.GetWerfFile(), errorList)
	}

	patchesRuleImpact := l.GetRuleImpact("patches")
	if patchesRuleImpact != nil {
		patchesErrorList := errorList.WithMaxLevel(patchesRuleImpact)
		rules.NewPatchesRule(l.cfg.Patches.Disable).CheckPatches(m.GetPath(), patchesErrorList)
	} else {
		rules.NewPatchesRule(l.cfg.Patches.Disable).CheckPatches(m.GetPath(), errorList)
	}
}

func (l *Images) Name() string {
	return l.name
}

func (l *Images) Desc() string {
	return l.desc
}
