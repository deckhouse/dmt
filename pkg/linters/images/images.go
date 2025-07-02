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
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Images {
	return &Images{
		name:      ID,
		desc:      "Lint docker images",
		cfg:       &cfg.LintersSettings.Images,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Images.Impact),
	}
}

func (l *Images) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())

	rules.NewImageRule(l.cfg).CheckImageNamesInDockerFiles(m.GetPath(), errorList)
	rules.NewDistrolessRule(l.cfg).CheckImageNamesInDockerFiles(m.GetPath(), errorList)
	rules.NewWerfRule().LintWerfFile(m.GetName(), m.GetWerfFile(), errorList)
	rules.NewPatchesRule(l.cfg.Patches.Disable).CheckPatches(m.GetPath(), errorList)
}

func (l *Images) Name() string {
	return l.name
}

func (l *Images) Desc() string {
	return l.desc
}
