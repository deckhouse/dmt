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

package module

import (
	"context"

	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/module/rules"
)

// Module linter
type Module struct {
	name, desc string
	cfg        *pkg.ModuleLinterConfig
	module     pkg.Module
	ruleIDs    set.Set
	ErrorList  *errors.LintRuleErrorsList
}

const ID = "module"

func New(cfg *pkg.ModuleLinterConfig, ruleIDs set.Set, m pkg.Module, errorList *errors.LintRuleErrorsList) *Module {
	return &Module{
		name:      ID,
		desc:      "Lint module rules",
		cfg:       cfg,
		module:    m,
		ruleIDs:   ruleIDs,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.Impact),
	}
}

func (l *Module) Lint(ctx context.Context) {
	pkg.RunRules(ctx, l.ruleIDs, l.rules())
}

// rules builds this linter's rule set. Keeping the set as data — rather than a
// sequence of hand-written calls — is what lets rules be selected or grouped
// later without touching the rules themselves.
//
// The per-rule impact levels here are real: mapModuleRules reads global per-rule
// impacts and only falls back to the linter's, so every rule is scoped by its own.
func (l *Module) rules() []pkg.Rule {
	m := l.module
	cfg := l.cfg
	errorList := l.ErrorList.WithModule(m.GetName())

	// level scopes errorList to the configured impact of a single rule.
	level := func(rule pkg.RuleConfig) *errors.LintRuleErrorsList {
		return errorList.WithMaxLevel(rule.GetLevel())
	}

	return []pkg.Rule{
		rules.NewDefinitionFileRule(cfg.DefinitionFileRuleSettings.Disable, m, level(cfg.Rules.DefinitionFileRule)),
		rules.NewOSSRule(cfg.OSSRuleSettings.Disable, cfg.ExcludeRules.OSS.VersionNotSemver, m, level(cfg.Rules.OSSRule)),
		rules.NewConversionsRule(cfg.ConversionsRuleSettings.Disable, m, level(cfg.Rules.ConversionRule)),
		rules.NewHelmignoreRule(cfg.HelmignoreRuleSettings.Disable, m, level(cfg.Rules.HelmignoreRule)),
		rules.NewLicenseRule(cfg.ExcludeRules.License.Files.Get(), cfg.ExcludeRules.License.Directories.Get(),
			m, level(cfg.Rules.LicenseRule)),
		rules.NewRequirementsRule(m, level(cfg.Rules.RequarementsRule)),
		rules.NewPackageYAMLRule(m, level(cfg.Rules.PackageYAMLRule)),
		rules.NewModulePackageConsistencyRule(m, level(cfg.Rules.ModulePackageConsistencyRule)),
		rules.NewLegacyReleaseFileRule(m, level(cfg.Rules.LegacyReleaseFileRule)),
		rules.NewEnabledScriptRule(m, level(cfg.Rules.EnabledScriptRule)),
		rules.NewReleaseLayoutRule(m, level(cfg.Rules.ReleaseLayoutRule)),
		rules.NewBundleLayoutRule(m, level(cfg.Rules.BundleLayoutRule)),
	}
}

func (l *Module) GetName() string {
	return l.name
}

func (l *Module) Desc() string {
	return l.desc
}
