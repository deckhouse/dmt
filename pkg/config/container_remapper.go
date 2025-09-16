package config

func NewContainerLinterConfig(userDTO *UserLinterSettingsDTO, globalDTO *GlobalLinterSettingsDTO) ContainerLinterConfig {
	rulesSettings, excludeRules, getRuleImpact := newLinterConfig(userDTO, globalDTO)

	return ContainerLinterConfig{
		Impact:        mergeImpact(userDTO.Impact, globalDTO.Impact),
		RulesSettings: rulesSettings,
		ExcludeRules:  excludeRules,
		GetRuleImpact: getRuleImpact,
	}
}
