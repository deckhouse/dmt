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

// ContainerSettings represents container linter settings
type ContainerSettings struct {
	ExcludeRules  ContainerExcludeRules   `mapstructure:"exclude-rules"`
	RulesSettings map[string]RuleSettings `mapstructure:"rules-settings"`

	Impact *pkg.Level `mapstructure:"impact"`
}

// GetRuleImpact returns the impact level for a specific container rule
func (c ContainerSettings) GetRuleImpact(ruleName string) *pkg.Level {
	// Check rule-specific settings first
	if c.RulesSettings != nil {
		if ruleSettings, exists := c.RulesSettings[ruleName]; exists && ruleSettings.Impact != nil {
			return ruleSettings.Impact
		}
	}
	// Fall back to general impact
	return c.Impact
}

// ContainerExcludeRules represents container-specific exclude rules
type ContainerExcludeRules struct {
	ControllerSecurityContext KindRuleExcludeList `mapstructure:"controller-security-context"`
	DNSPolicy                 KindRuleExcludeList `mapstructure:"dns-policy"`

	HostNetworkPorts       ContainerRuleExcludeList `mapstructure:"host-network-ports"`
	Ports                  ContainerRuleExcludeList `mapstructure:"ports"`
	ReadOnlyRootFilesystem ContainerRuleExcludeList `mapstructure:"read-only-root-filesystem"`
	ImageDigest            ContainerRuleExcludeList `mapstructure:"image-digest"`
	Resources              ContainerRuleExcludeList `mapstructure:"resources"`
	SecurityContext        ContainerRuleExcludeList `mapstructure:"security-context"`
	Liveness               ContainerRuleExcludeList `mapstructure:"liveness-probe"`
	Readiness              ContainerRuleExcludeList `mapstructure:"readiness-probe"`

	Description StringRuleExcludeList `mapstructure:"description"`
}
