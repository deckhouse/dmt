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

// NoCyrillicSettings represents no-cyrillic linter settings
type NoCyrillicSettings struct {
	NoCyrillicExcludeRules NoCyrillicExcludeRules  `mapstructure:"exclude-rules"`
	RulesSettings          map[string]RuleSettings `mapstructure:"rules-settings"`

	Impact *pkg.Level `mapstructure:"impact"`
}

// GetRuleImpact returns the impact level for a specific no-cyrillic rule
func (n NoCyrillicSettings) GetRuleImpact(ruleName string) *pkg.Level {
	// Check rule-specific settings first
	if n.RulesSettings != nil {
		if ruleSettings, exists := n.RulesSettings[ruleName]; exists && ruleSettings.Impact != nil {
			return ruleSettings.Impact
		}
	}
	// Fall back to general impact
	return n.Impact
}

// NoCyrillicExcludeRules represents no-cyrillic-specific exclude rules
type NoCyrillicExcludeRules struct {
	Files       StringRuleExcludeList `mapstructure:"files"`
	Directories PrefixRuleExcludeList `mapstructure:"directories"`
}
