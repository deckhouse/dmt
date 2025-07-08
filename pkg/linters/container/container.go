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
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
)

const (
	ID = "container"
)

// Container linter
type Container struct {
	name, desc string
	cfg        *config.ContainerSettings
	ErrorList  *errors.LintRuleErrorsList
	tracker    *exclusions.ExclusionTracker
}

// New creates a new container linter
func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Container {
	return &Container{
		name:      ID,
		desc:      "Lint container objects",
		cfg:       &cfg.LintersSettings.Container,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Container.Impact),
	}
}

// NewWithTracker creates a new container linter with exclusion tracking
func NewWithTracker(cfg *config.ModuleConfig, tracker *exclusions.ExclusionTracker, errorList *errors.LintRuleErrorsList) *Container {
	return &Container{
		name:      ID,
		desc:      "Lint container objects with exclusion tracking",
		cfg:       &cfg.LintersSettings.Container,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Container.Impact),
		tracker:   tracker,
	}
}

func (l *Container) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())
	for _, object := range m.GetStorage() {
		if l.tracker != nil {
			l.applyContainerRulesWithTracking(object, m.GetName(), errorList)
		} else {
			l.applyContainerRules(object, errorList)
		}
	}
}

func (l *Container) Name() string {
	return l.name
}

func (l *Container) Desc() string {
	return l.desc
}
