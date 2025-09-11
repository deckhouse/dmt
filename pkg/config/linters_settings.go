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

import (
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config/global"
)

type RuleSettings struct {
	Impact *pkg.Level `mapstructure:"impact"`
}

type LintersSettings struct {
	Container  ContainerSettings  `mapstructure:"container"`
	Hooks      HooksSettings      `mapstructure:"hooks"`
	Images     ImageSettings      `mapstructure:"images"`
	Module     ModuleSettings     `mapstructure:"module"`
	NoCyrillic NoCyrillicSettings `mapstructure:"no-cyrillic"`
	OpenAPI    OpenAPISettings    `mapstructure:"openapi"`
	Rbac       RbacSettings       `mapstructure:"rbac"`
	Templates  TemplatesSettings  `mapstructure:"templates"`
}

// MergeGlobal merges global configuration with linter settings
func (cfg *LintersSettings) MergeGlobal(lcfg *global.Linters) {
	if lcfg == nil {
		return
	}

	cfg.OpenAPI.Impact = calculateImpact(cfg.OpenAPI.Impact, lcfg.OpenAPI.Impact)
	cfg.NoCyrillic.Impact = calculateImpact(cfg.NoCyrillic.Impact, lcfg.NoCyrillic.Impact)
	cfg.Container.Impact = calculateImpact(cfg.Container.Impact, lcfg.Container.Impact)
	cfg.Templates.Impact = calculateImpact(cfg.Templates.Impact, lcfg.Templates.Impact)
	cfg.Images.Impact = calculateImpact(cfg.Images.Impact, lcfg.Images.Impact)
	cfg.Rbac.Impact = calculateImpact(cfg.Rbac.Impact, lcfg.Rbac.Impact)
	cfg.Hooks.Impact = calculateImpact(cfg.Hooks.Impact, lcfg.Hooks.Impact)
	cfg.Module.Impact = calculateImpact(cfg.Module.Impact, lcfg.Module.Impact)
}

// GetRuleImpact returns the impact level for a specific rule in a specific linter
func (cfg *LintersSettings) GetRuleImpact(linterName, ruleName string) *pkg.Level {
	switch linterName {
	case "container":
		return cfg.Container.GetRuleImpact(ruleName)
	case "hooks":
		return cfg.Hooks.GetRuleImpact(ruleName)
	case "images":
		return cfg.Images.GetRuleImpact(ruleName)
	case "module":
		return cfg.Module.GetRuleImpact(ruleName)
	case "no-cyrillic":
		return cfg.NoCyrillic.GetRuleImpact(ruleName)
	case "openapi":
		return cfg.OpenAPI.GetRuleImpact(ruleName)
	case "rbac":
		return cfg.Rbac.GetRuleImpact(ruleName)
	case "templates":
		return cfg.Templates.GetRuleImpact(ruleName)
	default:
		return nil
	}
}
