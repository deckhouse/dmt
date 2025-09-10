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

// TemplatesSettings represents templates linter settings
type TemplatesSettings struct {
	ExcludeRules      TemplatesExcludeRules        `mapstructure:"exclude-rules"`
	RulesSettings     map[string]RuleSettings      `mapstructure:"rules-settings"`
	GrafanaDashboards GrafanaDashboardsExcludeList `mapstructure:"grafana-dashboards"`
	PrometheusRules   PrometheusRulesExcludeList   `mapstructure:"prometheus-rules"`

	Impact *pkg.Level `mapstructure:"impact"`
}

// GetRuleImpact returns the impact level for a specific templates rule
func (t TemplatesSettings) GetRuleImpact(ruleName string) *pkg.Level {
	// Check rule-specific settings first
	if t.RulesSettings != nil {
		if ruleSettings, exists := t.RulesSettings[ruleName]; exists && ruleSettings.Impact != nil {
			return ruleSettings.Impact
		}
	}
	// Fall back to general impact
	return t.Impact
}

// TemplatesExcludeRules represents templates-specific exclude rules
type TemplatesExcludeRules struct {
	VPAAbsent     KindRuleExcludeList    `mapstructure:"vpa"`
	PDBAbsent     KindRuleExcludeList    `mapstructure:"pdb"`
	ServicePort   ServicePortExcludeList `mapstructure:"service-port"`
	KubeRBACProxy StringRuleExcludeList  `mapstructure:"kube-rbac-proxy"`
	Ingress       KindRuleExcludeList    `mapstructure:"ingress"`
}

// GrafanaDashboardsExcludeList represents grafana dashboards exclude list
type GrafanaDashboardsExcludeList struct {
	Disable bool `mapstructure:"disable"`
}

// PrometheusRulesExcludeList represents prometheus rules exclude list
type PrometheusRulesExcludeList struct {
	Disable bool `mapstructure:"disable"`
}
