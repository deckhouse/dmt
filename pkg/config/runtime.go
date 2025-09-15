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

type RuleImpactFunc func(linterID, ruleID string) *pkg.Level

type RuntimeLinterSettings struct {
	Impact         *pkg.Level
	RulesSettings  map[string]RuleSettings
	RuleImpactFunc RuleImpactFunc
}

type RuntimeLintersSettings struct {
	Container  RuntimeLinterSettings
	Hooks      RuntimeLinterSettings
	Images     RuntimeLinterSettings
	License    RuntimeLinterSettings
	Module     RuntimeLinterSettings
	NoCyrillic RuntimeLinterSettings
	OpenAPI    RuntimeLinterSettings
	Rbac       RuntimeLinterSettings
	Templates  RuntimeLinterSettings
}

type RuntimeRootConfig struct {
	LintersSettings RuntimeLintersSettings
}

// GetRuleImpactFunc returns the rule impact function for a specific linter
func (r *RuntimeLintersSettings) GetRuleImpactFunc(linterID, ruleID string) *pkg.Level {
	switch linterID {
	case "container":
		return r.Container.RuleImpactFunc(linterID, ruleID)
	case "hooks":
		return r.Hooks.RuleImpactFunc(linterID, ruleID)
	case "images":
		return r.Images.RuleImpactFunc(linterID, ruleID)
	case "license":
		return r.License.RuleImpactFunc(linterID, ruleID)
	case "module":
		return r.Module.RuleImpactFunc(linterID, ruleID)
	case "no-cyrillic":
		return r.NoCyrillic.RuleImpactFunc(linterID, ruleID)
	case "openapi":
		return r.OpenAPI.RuleImpactFunc(linterID, ruleID)
	case "rbac":
		return r.Rbac.RuleImpactFunc(linterID, ruleID)
	case "templates":
		return r.Templates.RuleImpactFunc(linterID, ruleID)
	default:
		return nil
	}
}
