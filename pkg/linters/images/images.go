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

package images

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
	"github.com/deckhouse/dmt/pkg/linters/images/rules"
)

const (
	ID = "images"
)

// Images linter
type Images struct {
	name, desc string
	cfg        *config.ImageSettings
	ErrorList  *errors.LintRuleErrorsList
	tracker    *exclusions.ExclusionTracker
}

func New(cfg *config.ModuleConfig, tracker *exclusions.ExclusionTracker, errorList *errors.LintRuleErrorsList) *Images {
	return &Images{
		name:      ID,
		desc:      "Lint docker images",
		cfg:       &cfg.LintersSettings.Images,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Images.Impact),
		tracker:   tracker,
	}
}

func (l *Images) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())
	l.run(m, m.GetName(), errorList)
}

func (l *Images) run(m *module.Module, moduleName string, errorList *errors.LintRuleErrorsList) {
	// Register rules without exclusions in tracker if available
	if l.tracker != nil {
		l.tracker.RegisterExclusionsForModule(ID, "werf-file", []string{}, moduleName)
	}

	if l.tracker != nil {
		// With tracking
		trackedImageRule := exclusions.NewTrackedPrefixRuleForModule(
			l.cfg.ExcludeRules.SkipImageFilePathPrefix.Get(),
			l.tracker,
			ID,
			"image-file-path-prefix",
			moduleName,
		)
		rules.NewImageRuleTracked(trackedImageRule).CheckImageNamesInDockerFiles(m.GetPath(), errorList)

		trackedDistrolessRule := exclusions.NewTrackedPrefixRuleForModule(
			l.cfg.ExcludeRules.SkipDistrolessFilePathPrefix.Get(),
			l.tracker,
			ID,
			"distroless-file-path-prefix",
			moduleName,
		)
		rules.NewDistrolessRuleTracked(trackedDistrolessRule).CheckImageNamesInDockerFiles(m.GetPath(), errorList)

		rules.NewWerfRule().LintWerfFile(m.GetWerfFile(), errorList)

		// --- Tracking for patches ---
		// If the rule is disabled, register this as a used exclusion
		if l.cfg.Patches.Disable {
			l.tracker.RegisterExclusionsForModule(ID, "patches", []string{}, moduleName)
		} else {
			// If the rule is enabled, perform the check
			rules.NewPatchesRule(false).CheckPatches(m.GetPath(), errorList)
		}
		// --- end ---
	} else {
		// Without tracking
		rules.NewImageRule(l.cfg).CheckImageNamesInDockerFiles(m.GetPath(), errorList)
		rules.NewDistrolessRule(l.cfg).CheckImageNamesInDockerFiles(m.GetPath(), errorList)
		rules.NewWerfRule().LintWerfFile(m.GetWerfFile(), errorList)
		rules.NewPatchesRule(l.cfg.Patches.Disable).CheckPatches(m.GetPath(), errorList)
	}
}

func (l *Images) Name() string {
	return l.name
}

func (l *Images) Desc() string {
	return l.desc
}
