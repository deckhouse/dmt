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

type RuleConfig struct {
	Impact *pkg.Level
}

type ContainerConfig struct {
	Impact        *pkg.Level
	RulesSettings map[string]RuleConfig
	ExcludeRules  ContainerExcludeRules
	GetRuleImpact func(ruleID string) *pkg.Level
}

type HooksConfig struct {
	Impact        *pkg.Level
	RulesSettings map[string]RuleConfig
	GetRuleImpact func(ruleID string) *pkg.Level
}

type ImagesConfig struct {
	Impact        *pkg.Level
	RulesSettings map[string]RuleConfig
	GetRuleImpact func(ruleID string) *pkg.Level
}

type ModuleLinterConfig struct {
	Impact        *pkg.Level
	RulesSettings map[string]RuleConfig
	GetRuleImpact func(ruleID string) *pkg.Level
}

type NoCyrillicConfig struct {
	Impact        *pkg.Level
	RulesSettings map[string]RuleConfig
	GetRuleImpact func(ruleID string) *pkg.Level
}

type OpenAPIConfig struct {
	Impact        *pkg.Level
	RulesSettings map[string]RuleConfig
	GetRuleImpact func(ruleID string) *pkg.Level
}

type RbacConfig struct {
	Impact        *pkg.Level
	RulesSettings map[string]RuleConfig
	GetRuleImpact func(ruleID string) *pkg.Level
}

type TemplatesConfig struct {
	Impact        *pkg.Level
	RulesSettings map[string]RuleConfig
	GetRuleImpact func(ruleID string) *pkg.Level
}

type DomainLintersConfig struct {
	Container  ContainerConfig
	Hooks      HooksConfig
	Images     ImagesConfig
	Module     ModuleLinterConfig
	NoCyrillic NoCyrillicConfig
	OpenAPI    OpenAPIConfig
	Rbac       RbacConfig
	Templates  TemplatesConfig
}

type DomainRootConfig struct {
	LintersConfig DomainLintersConfig
}
