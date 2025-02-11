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
