package rbac

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/rbac/roles"
)

// Rbac linter
type Rbac struct {
	name, desc string
	Cfg        *config.RbacSettings
}

func New(cfg *config.RbacSettings) *Rbac {
	roles.Cfg = cfg

	return &Rbac{
		name: "rbac",
		desc: "Lint rbac objects",
		Cfg:  cfg,
	}
}

func (o *Rbac) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewError(o.Name(), m.GetName())
	if m == nil {
		return result
	}

	for _, object := range m.GetStorage() {
		result.Merge(roles.ObjectUserAuthzClusterRolePath(m, object))
		result.Merge(roles.ObjectRBACPlacement(m, object))
		result.Merge(roles.ObjectBindingSubjectServiceAccountCheck(m, object, m.GetObjectStore()))
		result.Merge(roles.ObjectRolesWildcard(m, object))
	}

	return result
}

func (o *Rbac) Name() string {
	return o.name
}

func (o *Rbac) Desc() string {
	return o.desc
}
