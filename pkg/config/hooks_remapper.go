package config

func NewHooksLinterConfig(userDTO *UserLinterSettingsDTO, globalDTO *GlobalLinterSettingsDTO) HooksLinterConfig {
	rulesSettings, excludeRules, getRuleImpact := newLinterConfig(userDTO, globalDTO)

	return HooksLinterConfig{
		Impact:        mergeImpact(userDTO.Impact, globalDTO.Impact),
		RulesSettings: rulesSettings,
		ExcludeRules:  excludeRules,
		GetRuleImpact: getRuleImpact,
	}
}
