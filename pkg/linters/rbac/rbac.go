package rbac

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ID = "rbac"
)

var (
	Cfg *config.RbacSettings
)

// Rbac linter
type Rbac struct {
	name, desc string
	cfg        *config.RbacSettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Rbac {
	Cfg = &cfg.LintersSettings.Rbac

	return &Rbac{
		name:      ID,
		desc:      "Lint rbac objects",
		cfg:       &cfg.LintersSettings.Rbac,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Rbac.Impact),
	}
}

func (o *Rbac) Run(m *module.Module) *errors.LintRuleErrorsList {
	if m == nil {
		return nil
	}

	o.objectUserAuthzClusterRolePath(m)
	o.objectRBACPlacement(m)
	o.objectBindingSubjectServiceAccountCheck(m)
	o.objectRolesWildcard(m)

	return nil
}

func (o *Rbac) Name() string {
	return o.name
}

func (o *Rbac) Desc() string {
	return o.desc
}
