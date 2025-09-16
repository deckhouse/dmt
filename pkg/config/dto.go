package config

import "github.com/deckhouse/dmt/pkg"

type UserRuleSettingsDTO struct {
	Impact *pkg.Level `mapstructure:"impact"`
}

type UserLinterSettingsDTO struct {
	Impact        *pkg.Level                     `mapstructure:"impact"`
	RulesSettings map[string]UserRuleSettingsDTO `mapstructure:"rules-settings"`
	ExcludeRules  []string                       `mapstructure:"exclude-rules"`
}

type UserLintersSettingsDTO struct {
	Container  UserLinterSettingsDTO `mapstructure:"container"`
	Hooks      UserLinterSettingsDTO `mapstructure:"hooks"`
	Images     UserLinterSettingsDTO `mapstructure:"images"`
	Module     UserLinterSettingsDTO `mapstructure:"module"`
	NoCyrillic UserLinterSettingsDTO `mapstructure:"no-cyrillic"`
	OpenAPI    UserLinterSettingsDTO `mapstructure:"openapi"`
	Rbac       UserLinterSettingsDTO `mapstructure:"rbac"`
	Templates  UserLinterSettingsDTO `mapstructure:"templates"`
}

type UserRootConfigDTO struct {
	LintersSettings UserLintersSettingsDTO `mapstructure:"linters-settings"`
}

// Global DTO structures
type GlobalLinterSettingsDTO struct {
	Impact        *pkg.Level                     `mapstructure:"impact"`
	RulesSettings map[string]UserRuleSettingsDTO `mapstructure:"rules-settings"`
	ExcludeRules  []string                       `mapstructure:"exclude-rules"`
}

type GlobalLintersDTO struct {
	Container  GlobalLinterSettingsDTO `mapstructure:"container"`
	Hooks      GlobalLinterSettingsDTO `mapstructure:"hooks"`
	Images     GlobalLinterSettingsDTO `mapstructure:"images"`
	Module     GlobalLinterSettingsDTO `mapstructure:"module"`
	NoCyrillic GlobalLinterSettingsDTO `mapstructure:"no-cyrillic"`
	OpenAPI    GlobalLinterSettingsDTO `mapstructure:"openapi"`
	Rbac       GlobalLinterSettingsDTO `mapstructure:"rbac"`
	Templates  GlobalLinterSettingsDTO `mapstructure:"templates"`
}

type GlobalRootConfigDTO struct {
	Global GlobalLintersDTO `mapstructure:"global"`
}
