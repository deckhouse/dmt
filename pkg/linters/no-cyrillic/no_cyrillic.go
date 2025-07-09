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

package nocyrillic

import (
	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
	"github.com/deckhouse/dmt/pkg/linters/no-cyrillic/rules"
)

const (
	ID = "no-cyrillic"
)

var (
	fileExtensions = []string{"yaml", "yml", "json", "go"}
)

// NoCyrillic linter
type NoCyrillic struct {
	name, desc string
	cfg        *config.NoCyrillicSettings
	ErrorList  *errors.LintRuleErrorsList
	tracker    *exclusions.ExclusionTracker
}

func New(cfg *config.ModuleConfig, tracker *exclusions.ExclusionTracker, errorList *errors.LintRuleErrorsList) *NoCyrillic {
	return &NoCyrillic{
		name:      ID,
		desc:      "NoCyrillic will check all files in the modules for contains cyrillic symbols",
		cfg:       &cfg.LintersSettings.NoCyrillic,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.NoCyrillic.Impact),
		tracker:   tracker,
	}
}

func (l *NoCyrillic) Run(m *module.Module) {
	errorList := l.ErrorList.WithModule(m.GetName())

	if m.GetPath() == "" {
		return
	}

	l.run(m, errorList, m.GetName())
}

func (l *NoCyrillic) run(m *module.Module, errorList *errors.LintRuleErrorsList, moduleName string) {
	if l.tracker != nil {
		// With tracking
		filesRule := exclusions.NewTrackedRule(
			rules.NewFilesRule(l.cfg.NoCyrillicExcludeRules.Files.Get(), l.cfg.NoCyrillicExcludeRules.Directories.Get()),
			exclusions.PathRuleKeys(l.cfg.NoCyrillicExcludeRules.Files.Get(), l.cfg.NoCyrillicExcludeRules.Directories.Get()),
			l.tracker,
			ID,
			"files",
			moduleName,
		)

		files := fsutils.GetFiles(m.GetPath(), false, fsutils.FilterFileByExtensions(fileExtensions...))
		for _, fileName := range files {
			filesRule.CheckFile(m, fileName, errorList)
		}
	} else {
		// Without tracking
		filesRule := rules.NewFilesRule(
			l.cfg.NoCyrillicExcludeRules.Files.Get(),
			l.cfg.NoCyrillicExcludeRules.Directories.Get())

		files := fsutils.GetFiles(m.GetPath(), false, fsutils.FilterFileByExtensions(fileExtensions...))
		for _, fileName := range files {
			filesRule.CheckFile(m, fileName, errorList)
		}
	}
}

func (l *NoCyrillic) Name() string {
	return l.name
}

func (l *NoCyrillic) Desc() string {
	return l.desc
}
