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
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
	"github.com/deckhouse/dmt/pkg/linters/module/rules"
)

// Module linter
type Module struct {
	name, desc string
	cfg        *config.ModuleSettings
	ErrorList  *errors.LintRuleErrorsList
	tracker    *exclusions.ExclusionTracker
}

const ID = "module"

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Module {
	return &Module{
		name:      ID,
		desc:      "Lint module rules",
		cfg:       &cfg.LintersSettings.Module,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Module.Impact),
	}
}

func NewWithTracker(cfg *config.ModuleConfig, tracker *exclusions.ExclusionTracker, errorList *errors.LintRuleErrorsList) *Module {
	return &Module{
		name:      ID,
		desc:      "Lint module rules (with exclusion tracking)",
		cfg:       &cfg.LintersSettings.Module,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Module.Impact),
		tracker:   tracker,
	}
}

func (l *Module) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())

	if l.tracker != nil {
		l.runWithTracking(m, m.GetName(), errorList)
	} else {
		l.runWithoutTracking(m, errorList)
	}
}

func (l *Module) runWithoutTracking(m *module.Module, errorList *errors.LintRuleErrorsList) {
	rules.NewDefinitionFileRule(l.cfg.DefinitionFile.Disable).CheckDefinitionFile(m.GetPath(), errorList)
	rules.NewOSSRule(l.cfg.OSS.Disable).OssModuleRule(m.GetPath(), errorList)

	// For conversions we use disable flag
	rules.NewConversionsRule(l.cfg.Conversions.Disable).CheckConversions(m.GetPath(), errorList)

	rules.NewLicenseRule(l.cfg.ExcludeRules.License.Files.Get(), l.cfg.ExcludeRules.License.Directories.Get()).
		CheckFiles(m, errorList)
}

func (l *Module) runWithTracking(m *module.Module, moduleName string, errorList *errors.LintRuleErrorsList) {
	// Register rules without exclusions in tracker
	l.tracker.RegisterExclusionsForModule(ID, "definition-file", []string{}, moduleName)
	l.tracker.RegisterExclusionsForModule(ID, "oss", []string{}, moduleName)

	rules.NewDefinitionFileRule(l.cfg.DefinitionFile.Disable).CheckDefinitionFile(m.GetPath(), errorList)
	rules.NewOSSRule(l.cfg.OSS.Disable).OssModuleRule(m.GetPath(), errorList)

	// --- Tracking for conversions ---
	// If the rule is disabled, register this as a used exclusion
	if l.cfg.Conversions.Disable {
		l.tracker.RegisterExclusionsForModule(ID, "conversions", []string{}, moduleName)
	} else {
		// If the rule is enabled, use exclusions for specific files
		trackedConversionsRule := exclusions.NewTrackedStringRuleForModule(
			l.cfg.ExcludeRules.Conversions.Files.Get(),
			l.tracker,
			ID,
			"conversions",
			moduleName,
		)
		rules.NewConversionsRuleTracked(trackedConversionsRule).CheckConversions(m.GetPath(), errorList)
	}
	// --- end ---

	trackedLicenseRule := exclusions.NewTrackedPathRuleForModule(
		l.cfg.ExcludeRules.License.Files.Get(),
		l.cfg.ExcludeRules.License.Directories.Get(),
		l.tracker,
		ID,
		"license",
		moduleName,
	)
	rules.NewLicenseRuleTracked(trackedLicenseRule).CheckFiles(m, errorList)
}

func (l *Module) Name() string {
	return l.name
}

func (l *Module) Desc() string {
	return l.desc
}
