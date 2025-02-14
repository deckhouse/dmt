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
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
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

	NewFilesRule(l.cfg.ExcludeRules.Files.Get()).
		checkFiles(m, errorList)
}

func (r *FilesRule) checkFiles(mod *module.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	files := fsutils.GetFiles(mod.GetPath(), false, filterFiles)
	for _, fileName := range files {
		name, _ := strings.CutPrefix(fileName, mod.GetPath())
		name = mod.GetName() + ":" + name

		if !r.Enabled(name) {
			// TODO: add metrics
			continue
		}

		ok, err := checkFileCopyright(fileName)
		if !ok {
			path, _ := strings.CutPrefix(fileName, mod.GetPath())

			errorList.WithFilePath(path).Error(err.Error())
		}
	}
}

func filterFiles(path string) bool {
	if fileToCheckRe.MatchString(path) && !fileToSkipRe.MatchString(path) {
		return true
	}

	return false
}

func (l *Copyright) Name() string {
	return l.name
}

func (l *Copyright) Desc() string {
	return l.desc
}
