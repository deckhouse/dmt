package config

import "github.com/deckhouse/dmt/pkg"

type RuleConfig struct {
	Impact *pkg.Level
}

type ContainerLinterConfig struct {
	Impact        *pkg.Level
	RulesSettings map[string]RuleConfig
	ExcludeRules  []string
	GetRuleImpact func(ruleID string) *pkg.Level
	Rules         ContainerLinterRules
}

type ContainerLinterRules struct {
	RecommendedLabelsRule RuleConfig
}

type HooksLinterConfig struct {
	Impact        *pkg.Level
	RulesSettings map[string]RuleConfig
	ExcludeRules  []string
	GetRuleImpact func(ruleID string) *pkg.Level
}

type ImagesLinterConfig struct {
	Impact        *pkg.Level
	RulesSettings map[string]RuleConfig
	ExcludeRules  []string
	GetRuleImpact func(ruleID string) *pkg.Level
}

type ModuleLinterConfig struct {
	Impact        *pkg.Level
	RulesSettings map[string]RuleConfig
	ExcludeRules  []string
	GetRuleImpact func(ruleID string) *pkg.Level
}

type NoCyrillicLinterConfig struct {
	Impact        *pkg.Level
	RulesSettings map[string]RuleConfig
	ExcludeRules  []string
	GetRuleImpact func(ruleID string) *pkg.Level
}

type OpenAPILinterConfig struct {
	Impact        *pkg.Level
	RulesSettings map[string]RuleConfig
	ExcludeRules  []string
	GetRuleImpact func(ruleID string) *pkg.Level
}

type RbacLinterConfig struct {
	Impact        *pkg.Level
	RulesSettings map[string]RuleConfig
	ExcludeRules  []string
	GetRuleImpact func(ruleID string) *pkg.Level
}

type TemplatesLinterConfig struct {
	Impact        *pkg.Level
	RulesSettings map[string]RuleConfig
	ExcludeRules  []string
	GetRuleImpact func(ruleID string) *pkg.Level
}

type LintersConfig struct {
	Container  ContainerLinterConfig
	Hooks      HooksLinterConfig
	Images     ImagesLinterConfig
	Module     ModuleLinterConfig
	NoCyrillic NoCyrillicLinterConfig
	OpenAPI    OpenAPILinterConfig
	Rbac       RbacLinterConfig
	Templates  TemplatesLinterConfig
}

type DomainRootConfig struct {
	LintersConfig LintersConfig
}
