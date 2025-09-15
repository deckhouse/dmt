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

package config

import (
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config/global"
)

func (dto *UserRootConfig) ToDomain(globalSettings *global.Linters) *DomainRootConfig {
	return &DomainRootConfig{
		LintersConfig: *dto.LintersSettings.ToDomain(globalSettings),
	}
}

func (dto *UserLintersSettings) ToDomain(globalSettings *global.Linters) *DomainLintersConfig {
	if globalSettings == nil {
		globalSettings = &global.Linters{}
	}

	return &DomainLintersConfig{
		Container:  *dto.Container.ToContainerConfig(globalSettings.Container),
		Hooks:      *dto.Hooks.ToHooksConfig(globalSettings.Hooks),
		Images:     *dto.Images.ToImagesConfig(globalSettings.Images),
		Module:     *dto.Module.ToModuleConfig(globalSettings.Module),
		NoCyrillic: *dto.NoCyrillic.ToNoCyrillicConfig(globalSettings.NoCyrillic),
		OpenAPI:    *dto.OpenAPI.ToOpenAPIConfig(globalSettings.OpenAPI),
		Rbac:       *dto.Rbac.ToRbacConfig(globalSettings.Rbac),
		Templates:  *dto.Templates.ToTemplatesConfig(globalSettings.Templates),
	}
}

type baseConfig struct {
	Impact        *pkg.Level
	RulesSettings map[string]RuleConfig
	GetRuleImpact func(ruleID string) *pkg.Level
}

func (dto *UserLinterSettings) toBaseConfig(globalImpact *pkg.Level) baseConfig {
	impact := calculateDomainImpact(dto.Impact, globalImpact)

	rulesSettings := make(map[string]RuleConfig)
	for ruleName, ruleSettings := range dto.RulesSettings {
		rulesSettings[ruleName] = RuleConfig(ruleSettings)
	}

	getRuleImpact := func(ruleID string) *pkg.Level {
		if ruleConfig, exists := rulesSettings[ruleID]; exists && ruleConfig.Impact != nil {
			return ruleConfig.Impact
		}
		return impact
	}

	return baseConfig{
		Impact:        impact,
		RulesSettings: rulesSettings,
		GetRuleImpact: getRuleImpact,
	}
}

func (dto *UserLinterSettings) ToContainerConfig(globalConfig global.LinterConfig) *ContainerConfig {
	baseConfig := dto.toBaseConfig(globalConfig.Impact)
	return &ContainerConfig{
		Impact:        baseConfig.Impact,
		RulesSettings: baseConfig.RulesSettings,
		ExcludeRules:  ContainerExcludeRules{}, // TODO: добавить ремаппинг ExcludeRules
		GetRuleImpact: baseConfig.GetRuleImpact,
	}
}

func (dto *UserLinterSettings) ToHooksConfig(globalConfig global.LinterConfig) *HooksConfig {
	baseConfig := dto.toBaseConfig(globalConfig.Impact)
	return &HooksConfig{
		Impact:        baseConfig.Impact,
		RulesSettings: baseConfig.RulesSettings,
		GetRuleImpact: baseConfig.GetRuleImpact,
	}
}

func (dto *UserLinterSettings) ToImagesConfig(globalConfig global.LinterConfig) *ImagesConfig {
	baseConfig := dto.toBaseConfig(globalConfig.Impact)
	return &ImagesConfig{
		Impact:        baseConfig.Impact,
		RulesSettings: baseConfig.RulesSettings,
		GetRuleImpact: baseConfig.GetRuleImpact,
	}
}

func (dto *UserLinterSettings) ToModuleConfig(globalConfig global.LinterConfig) *ModuleLinterConfig {
	baseConfig := dto.toBaseConfig(globalConfig.Impact)
	return &ModuleLinterConfig{
		Impact:        baseConfig.Impact,
		RulesSettings: baseConfig.RulesSettings,
		GetRuleImpact: baseConfig.GetRuleImpact,
	}
}

func (dto *UserLinterSettings) ToNoCyrillicConfig(globalConfig global.LinterConfig) *NoCyrillicConfig {
	baseConfig := dto.toBaseConfig(globalConfig.Impact)
	return &NoCyrillicConfig{
		Impact:        baseConfig.Impact,
		RulesSettings: baseConfig.RulesSettings,
		GetRuleImpact: baseConfig.GetRuleImpact,
	}
}

func (dto *UserLinterSettings) ToOpenAPIConfig(globalConfig global.LinterConfig) *OpenAPIConfig {
	baseConfig := dto.toBaseConfig(globalConfig.Impact)
	return &OpenAPIConfig{
		Impact:        baseConfig.Impact,
		RulesSettings: baseConfig.RulesSettings,
		GetRuleImpact: baseConfig.GetRuleImpact,
	}
}

func (dto *UserLinterSettings) ToRbacConfig(globalConfig global.LinterConfig) *RbacConfig {
	baseConfig := dto.toBaseConfig(globalConfig.Impact)
	return &RbacConfig{
		Impact:        baseConfig.Impact,
		RulesSettings: baseConfig.RulesSettings,
		GetRuleImpact: baseConfig.GetRuleImpact,
	}
}

func (dto *UserLinterSettings) ToTemplatesConfig(globalConfig global.LinterConfig) *TemplatesConfig {
	baseConfig := dto.toBaseConfig(globalConfig.Impact)
	return &TemplatesConfig{
		Impact:        baseConfig.Impact,
		RulesSettings: baseConfig.RulesSettings,
		GetRuleImpact: baseConfig.GetRuleImpact,
	}
}

func calculateDomainImpact(local, globalLevel *pkg.Level) *pkg.Level {
	if local != nil {
		return local
	}
	return globalLevel
}
