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

import "github.com/deckhouse/dmt/pkg"

type ModuleSettings struct {
	ExcludeRules  ModuleExcludeRules      `mapstructure:"exclude-rules"`
	RulesSettings map[string]RuleSettings `mapstructure:"rules-settings"`

	OSS            ModuleOSSRuleSettings            `mapstructure:"oss"`
	DefinitionFile ModuleDefinitionFileRuleSettings `mapstructure:"definition-file"`
	Conversions    ConversionsRuleSettings          `mapstructure:"conversions"`
	Helmignore     HelmignoreRuleSettings           `mapstructure:"helmignore"`

	Impact *pkg.Level `mapstructure:"impact"`
}

func (m *ModuleSettings) GetRuleImpact(ruleName string) *pkg.Level {
	if m.RulesSettings != nil {
		if ruleSettings, exists := m.RulesSettings[ruleName]; exists && ruleSettings.Impact != nil {
			return ruleSettings.Impact
		}
	}
	return m.Impact
}

type ModuleExcludeRules struct {
	License LicenseExcludeRule `mapstructure:"license"`
}

type ModuleOSSRuleSettings struct {
	// disable oss rule completely
	Disable bool `mapstructure:"disable"`
}

type ModuleDefinitionFileRuleSettings struct {
	// disable definition-file rule completely
	Disable bool `mapstructure:"disable"`
}

type ConversionsRuleSettings struct {
	// disable conversions rule completely
	Disable bool `mapstructure:"disable"`
}

type HelmignoreRuleSettings struct {
	// disable helmignore rule completely
	Disable bool `mapstructure:"disable"`
}

type LicenseExcludeRule struct {
	Files       StringRuleExcludeList `mapstructure:"files"`
	Directories PrefixRuleExcludeList `mapstructure:"directories"`
}
