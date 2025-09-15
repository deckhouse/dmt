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

func (dto *UserRootConfig) ToRuntime(globalSettings *global.Linters) *RuntimeRootConfig {
	return &RuntimeRootConfig{
		LintersSettings: *dto.LintersSettings.ToRuntime(globalSettings),
	}
}

func (dto *UserLintersSettings) ToRuntime(globalSettings *global.Linters) *RuntimeLintersSettings {
	if globalSettings == nil {
		globalSettings = &global.Linters{}
	}

	return &RuntimeLintersSettings{
		Container:  *dto.Container.ToRuntime(globalSettings.Container),
		Hooks:      *dto.Hooks.ToRuntime(globalSettings.Hooks),
		Images:     *dto.Images.ToRuntime(globalSettings.Images),
		License:    *dto.License.ToRuntime(globalSettings.License),
		Module:     *dto.Module.ToRuntime(globalSettings.Module),
		NoCyrillic: *dto.NoCyrillic.ToRuntime(globalSettings.NoCyrillic),
		OpenAPI:    *dto.OpenAPI.ToRuntime(globalSettings.OpenAPI),
		Rbac:       *dto.Rbac.ToRuntime(globalSettings.Rbac),
		Templates:  *dto.Templates.ToRuntime(globalSettings.Templates),
	}
}

func (dto *UserLinterSettings) ToRuntime(globalConfig global.LinterConfig) *RuntimeLinterSettings {
	impact := calculateRuntimeImpact(dto.Impact, globalConfig.Impact)

	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleSettings := range dto.RulesSettings {
		rulesSettings[ruleName] = RuleSettings{
			Impact: ruleSettings.Impact,
		}
	}

	ruleImpactFunc := func(linterID, ruleID string) *pkg.Level {
		if rulesSettings, exists := rulesSettings[ruleID]; exists && rulesSettings.Impact != nil {
			return rulesSettings.Impact
		}
		return impact
	}

	return &RuntimeLinterSettings{
		Impact:         impact,
		RulesSettings:  rulesSettings,
		RuleImpactFunc: ruleImpactFunc,
	}
}

func calculateRuntimeImpact(local, global *pkg.Level) *pkg.Level {
	if local != nil {
		return local
	}
	return global
}
