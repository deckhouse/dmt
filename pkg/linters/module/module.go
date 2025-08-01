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
	"github.com/deckhouse/dmt/pkg/linters/module/rules"
)

// Module linter
type Module struct {
	name, desc string
	cfg        *config.ModuleSettings
	ErrorList  *errors.LintRuleErrorsList
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

func (l *Module) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())

	rules.NewDefinitionFileRule(l.cfg.DefinitionFile.Disable).CheckDefinitionFile(m.GetPath(), errorList)
	rules.NewOSSRule(l.cfg.OSS.Disable).OssModuleRule(m.GetPath(), errorList)
	rules.NewConversionsRule(l.cfg.Conversions.Disable).CheckConversions(m.GetPath(), errorList)
	rules.NewHelmignoreRule(l.cfg.Helmignore.Disable).CheckHelmignore(m.GetPath(), errorList)
	rules.NewLicenseRule(l.cfg.ExcludeRules.License.Files.Get(), l.cfg.ExcludeRules.License.Directories.Get()).
		CheckFiles(m, errorList)
	rules.NewRequirementsRule().CheckRequirements(m.GetPath(), errorList)
}

func (l *Module) Name() string {
	return l.name
}

func (l *Module) Desc() string {
	return l.desc
}
