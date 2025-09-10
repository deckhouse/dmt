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

// RbacSettings represents rbac linter settings
type RbacSettings struct {
	ExcludeRules  RBACExcludeRules        `mapstructure:"exclude-rules"`
	RulesSettings map[string]RuleSettings `mapstructure:"rules-settings"`

	Impact *pkg.Level `mapstructure:"impact"`
}

// GetRuleImpact returns the impact level for a specific rbac rule
func (r RbacSettings) GetRuleImpact(ruleName string) *pkg.Level {
	// Check rule-specific settings first
	if r.RulesSettings != nil {
		if ruleSettings, exists := r.RulesSettings[ruleName]; exists && ruleSettings.Impact != nil {
			return ruleSettings.Impact
		}
	}
	// Fall back to general impact
	return r.Impact
}

// RBACExcludeRules represents rbac-specific exclude rules
type RBACExcludeRules struct {
	BindingSubject StringRuleExcludeList `mapstructure:"binding-subject"`
	Placement      KindRuleExcludeList   `mapstructure:"placement"`
	Wildcards      KindRuleExcludeList   `mapstructure:"wildcards"`
}
