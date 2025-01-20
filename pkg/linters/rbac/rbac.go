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

func (*Rbac) Run(m *module.Module) (*errors.LintRuleErrorsList, error) {
	result := &errors.LintRuleErrorsList{}
	if m == nil {
		return result, nil
	}

	for _, object := range m.GetStorage() {
		result.Add(roles.ObjectUserAuthzClusterRolePath(m, object))
		result.Add(roles.ObjectRBACPlacement(m, object))
		result.Add(roles.ObjectBindingSubjectServiceAccountCheck(m, object, m.GetObjectStore()))
		result.Add(roles.ObjectRolesWildcard(m, object))
	}

	return result, nil
}

func (o *Rbac) Name() string {
	return o.name
}

func (o *Rbac) Desc() string {
	return o.desc
}
