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

package container

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ID = "container"
)

// Container linter
type Container struct {
	name, desc string
	cfg        *pkg.ContainerLinterConfig
	ErrorList  *errors.LintRuleErrorsList
}

func New(containerCfg *pkg.ContainerLinterConfig, errorList *errors.LintRuleErrorsList) *Container {
	return &Container{
		name:      ID,
		desc:      "Lint container objects",
		cfg:       containerCfg,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(containerCfg.Impact),
	}
}

func (l *Container) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())
	storage := m.GetStorage()
	for _, object := range storage {
		l.applyContainerRules(object, storage, errorList)
	}
}

func (l *Container) Name() string {
	return l.name
}

func (l *Container) Desc() string {
	return l.desc
}
