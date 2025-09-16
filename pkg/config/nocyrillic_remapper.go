package config

func NewNoCyrillicLinterConfig(userDTO *UserLinterSettingsDTO, globalDTO *GlobalLinterSettingsDTO) NoCyrillicLinterConfig {
	rulesSettings, excludeRules, getRuleImpact := newLinterConfig(userDTO, globalDTO)

	return NoCyrillicLinterConfig{
		Impact:        mergeImpact(userDTO.Impact, globalDTO.Impact),
		RulesSettings: rulesSettings,
		ExcludeRules:  excludeRules,
		GetRuleImpact: getRuleImpact,
	}
}
