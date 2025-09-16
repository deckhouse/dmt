package config

func NewRbacLinterConfig(userDTO *UserLinterSettingsDTO, globalDTO *GlobalLinterSettingsDTO) RbacLinterConfig {
	rulesSettings, excludeRules, getRuleImpact := newLinterConfig(userDTO, globalDTO)

	return RbacLinterConfig{
		Impact:        mergeImpact(userDTO.Impact, globalDTO.Impact),
		RulesSettings: rulesSettings,
		ExcludeRules:  excludeRules,
		GetRuleImpact: getRuleImpact,
	}
}
