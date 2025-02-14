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

package rbac

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ID = "rbac"
)

// Rbac linter
type Rbac struct {
	name, desc string
	cfg        *config.RbacSettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Rbac {
	return &Rbac{
		name:      ID,
		desc:      "Lint rbac objects",
		cfg:       &cfg.LintersSettings.Rbac,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Rbac.Impact),
	}
}

func (l *Rbac) Run(m *module.Module) {
	if m == nil {
		return
	}

	l.objectUserAuthzClusterRolePath(m)
	l.objectRBACPlacement(m)
	l.objectBindingSubjectServiceAccountCheck(m)
	l.objectRolesWildcard(m)
}

func (l *Rbac) Name() string {
	return l.name
}

func (l *Rbac) Desc() string {
	return l.desc
}
