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

type ImageSettings struct {
	ExcludeRules  ImageExcludeRules       `mapstructure:"exclude-rules"`
	RulesSettings map[string]RuleSettings `mapstructure:"rules-settings"`

	Patches PatchesRuleSettings `mapstructure:"patches"`
	Werf    WerfRuleSettings    `mapstructure:"werf"`

	Impact *pkg.Level `mapstructure:"impact"`
}

func (i *ImageSettings) GetRuleImpact(ruleName string) *pkg.Level {
	if i.RulesSettings != nil {
		if ruleSettings, exists := i.RulesSettings[ruleName]; exists && ruleSettings.Impact != nil {
			return ruleSettings.Impact
		}
	}
	return i.Impact
}

type ImageExcludeRules struct {
	SkipImageFilePathPrefix      PrefixRuleExcludeList `mapstructure:"skip-image-file-path-prefix"`
	SkipDistrolessFilePathPrefix PrefixRuleExcludeList `mapstructure:"skip-distroless-file-path-prefix"`
}

type PatchesRuleSettings struct {
	// disable conversions rule completely
	Disable bool `mapstructure:"disable"`
}

type WerfRuleSettings struct {
	// disable werf rule completely
	Disable bool `mapstructure:"disable"`
}
