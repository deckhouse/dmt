package config

func NewImagesLinterConfig(userDTO *UserLinterSettingsDTO, globalDTO *GlobalLinterSettingsDTO) ImagesLinterConfig {
	rulesSettings, excludeRules, getRuleImpact := newLinterConfig(userDTO, globalDTO)

	return ImagesLinterConfig{
		Impact:        mergeImpact(userDTO.Impact, globalDTO.Impact),
		RulesSettings: rulesSettings,
		ExcludeRules:  excludeRules,
		GetRuleImpact: getRuleImpact,
	}
}
