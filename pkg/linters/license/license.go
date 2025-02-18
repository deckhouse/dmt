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

package license

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/license/rules"
)

const (
	ID = "license"
)

// Copyright linter
type Copyright struct {
	name, desc string
	cfg        *config.LicenseSettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Copyright {
	return &Copyright{
		name:      ID,
		desc:      "Copyright will check all files in the modules for contains copyright",
		cfg:       &cfg.LintersSettings.License,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.License.Impact),
	}
}

func (l *Copyright) Run(m *module.Module) {
	if m.GetPath() == "" {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())

	rules.NewFilesRule(l.cfg.ExcludeRules.Files.Get(), l.cfg.ExcludeRules.Directories.Get()).
		CheckFiles(m, errorList)
}

func (l *Copyright) Name() string {
	return l.name
}

func (l *Copyright) Desc() string {
	return l.desc
}
