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

type ContainerSettings struct {
	ExcludeRules  ContainerExcludeRules   `mapstructure:"exclude-rules"`
	RulesSettings map[string]RuleSettings `mapstructure:"rules-settings"`

	Impact *pkg.Level `mapstructure:"impact"`
}

func (c *ContainerSettings) GetRuleImpact(ruleName string) *pkg.Level {
	if c.RulesSettings != nil {
		if ruleSettings, exists := c.RulesSettings[ruleName]; exists && ruleSettings.Impact != nil {
			return ruleSettings.Impact
		}
	}
	return c.Impact
}

func (c *ContainerSettings) SetRuleImpact(ruleName string, impact *pkg.Level) {
	if c.RulesSettings == nil {
		c.RulesSettings = make(map[string]RuleSettings)
	}
	c.RulesSettings[ruleName] = RuleSettings{Impact: impact}
}

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
