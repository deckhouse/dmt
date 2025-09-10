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

package module

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/module/rules"
)

// Module linter
type Module struct {
	name, desc string
	cfg        *config.ModuleSettings
	ErrorList  *errors.LintRuleErrorsList
	moduleCfg  *config.ModuleConfig
}

const ID = "module"

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Module {
	return &Module{
		name:      ID,
		desc:      "Lint module rules",
		cfg:       &cfg.LintersSettings.Module,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Module.Impact),
		moduleCfg: cfg,
	}
}

func (l *Module) GetRuleImpact(ruleName string) *pkg.Level {
	if l.moduleCfg != nil {
		return l.moduleCfg.LintersSettings.GetRuleImpact(ID, ruleName)
	}
	return l.cfg.Impact
}

func (l *Module) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())

	// Apply rule-specific impact for each rule
	definitionFileRuleImpact := l.GetRuleImpact("definition-file")
	if definitionFileRuleImpact != nil {
		definitionFileErrorList := errorList.WithMaxLevel(definitionFileRuleImpact)
		rules.NewDefinitionFileRule(l.cfg.DefinitionFile.Disable).CheckDefinitionFile(m.GetPath(), definitionFileErrorList)
	} else {
		rules.NewDefinitionFileRule(l.cfg.DefinitionFile.Disable).CheckDefinitionFile(m.GetPath(), errorList)
	}

	ossRuleImpact := l.GetRuleImpact("oss")
	if ossRuleImpact != nil {
		ossErrorList := errorList.WithMaxLevel(ossRuleImpact)
		rules.NewOSSRule(l.cfg.OSS.Disable).OssModuleRule(m.GetPath(), ossErrorList)
	} else {
		rules.NewOSSRule(l.cfg.OSS.Disable).OssModuleRule(m.GetPath(), errorList)
	}

	conversionsRuleImpact := l.GetRuleImpact("conversions")
	if conversionsRuleImpact != nil {
		conversionsErrorList := errorList.WithMaxLevel(conversionsRuleImpact)
		rules.NewConversionsRule(l.cfg.Conversions.Disable).CheckConversions(m.GetPath(), conversionsErrorList)
	} else {
		rules.NewConversionsRule(l.cfg.Conversions.Disable).CheckConversions(m.GetPath(), errorList)
	}

	helmignoreRuleImpact := l.GetRuleImpact("helmignore")
	if helmignoreRuleImpact != nil {
		helmignoreErrorList := errorList.WithMaxLevel(helmignoreRuleImpact)
		rules.NewHelmignoreRule(l.cfg.Helmignore.Disable).CheckHelmignore(m.GetPath(), helmignoreErrorList)
	} else {
		rules.NewHelmignoreRule(l.cfg.Helmignore.Disable).CheckHelmignore(m.GetPath(), errorList)
	}

	licenseRuleImpact := l.GetRuleImpact("license")
	if licenseRuleImpact != nil {
		licenseErrorList := errorList.WithMaxLevel(licenseRuleImpact)
		rules.NewLicenseRule(l.cfg.ExcludeRules.License.Files.Get(), l.cfg.ExcludeRules.License.Directories.Get()).
			CheckFiles(m, licenseErrorList)
	} else {
		rules.NewLicenseRule(l.cfg.ExcludeRules.License.Files.Get(), l.cfg.ExcludeRules.License.Directories.Get()).
			CheckFiles(m, errorList)
	}

	requirementsRuleImpact := l.GetRuleImpact("requirements")
	if requirementsRuleImpact != nil {
		requirementsErrorList := errorList.WithMaxLevel(requirementsRuleImpact)
		rules.NewRequirementsRule().CheckRequirements(m.GetPath(), requirementsErrorList)
	} else {
		rules.NewRequirementsRule().CheckRequirements(m.GetPath(), errorList)
	}
}

func (l *Module) Name() string {
	return l.name
}

func (l *Module) Desc() string {
	return l.desc
}
