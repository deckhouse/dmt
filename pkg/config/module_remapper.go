package config

import "github.com/deckhouse/dmt/pkg"

func NewModuleLinterConfig(userDTO *UserLinterSettingsDTO, globalDTO *GlobalLinterSettingsDTO) ModuleLinterConfig {
	config := ModuleLinterConfig{
		Impact:        mergeImpact(userDTO.Impact, globalDTO.Impact),
		RulesSettings: make(map[string]RuleConfig),
		ExcludeRules:  mergeExcludeRules(userDTO.ExcludeRules, globalDTO.ExcludeRules),
	}

	for ruleID, ruleDTO := range userDTO.RulesSettings {
		config.RulesSettings[ruleID] = RuleConfig(ruleDTO)
	}
	for ruleID, ruleDTO := range globalDTO.RulesSettings {
		if _, exists := config.RulesSettings[ruleID]; !exists {
			config.RulesSettings[ruleID] = RuleConfig(ruleDTO)
		}
	}
	config.GetRuleImpact = func(ruleID string) *pkg.Level {
		if rule, exists := config.RulesSettings[ruleID]; exists {
			return rule.Impact
		}
		return config.Impact
	}

	return config
}
