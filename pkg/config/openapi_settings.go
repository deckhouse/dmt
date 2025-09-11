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

type OpenAPISettings struct {
	OpenAPIExcludeRules OpenAPIExcludeRules     `mapstructure:"exclude-rules"`
	RulesSettings       map[string]RuleSettings `mapstructure:"rules-settings"`

	Impact *pkg.Level `mapstructure:"impact"`
}

func (o *OpenAPISettings) GetRuleImpact(ruleName string) *pkg.Level {
	if o.RulesSettings != nil {
		if ruleSettings, exists := o.RulesSettings[ruleName]; exists && ruleSettings.Impact != nil {
			return ruleSettings.Impact
		}
	}
	return o.Impact
}

func (o *OpenAPISettings) SetRuleImpact(ruleName string, impact *pkg.Level) {
	if o.RulesSettings == nil {
		o.RulesSettings = make(map[string]RuleSettings)
	}
	o.RulesSettings[ruleName] = RuleSettings{Impact: impact}
}

type OpenAPIExcludeRules struct {
	KeyBannedNames         []string              `mapstructure:"key-banned-names"`
	EnumFileExcludes       []string              `mapstructure:"enum"`
	HAAbsoluteKeysExcludes StringRuleExcludeList `mapstructure:"ha-absolute-keys"`
	CRDNamesExcludes       StringRuleExcludeList `mapstructure:"crd-names"`
}
