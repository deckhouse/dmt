package rbac

import (
	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/errors"
	"github.com/deckhouse/d8-lint/pkg/linters/rbac/roles"
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
	Cfg        *config.RbacSettings
}

func New(cfg *config.RbacSettings) *Rbac {
	Cfg = cfg
	return &Rbac{
		name: "rbac",
		desc: "Lint rbac objects",
		Cfg:  cfg,
	}
}

func (o *Rbac) Run(m *module.Module) (result errors.LintRuleErrorsList, err error) {
	if m == nil {
		return result, err
	}

	for _, object := range m.GetStorage() {
		result.Add(roles.ObjectUserAuthzClusterRolePath(m, object))
		result.Add(roles.ObjectRBACPlacement(m, object))
		result.Add(roles.ObjectBindingSubjectServiceAccountCheck(m, object, m.GetObjectStore()))
		result.Add(roles.ObjectRolesWildcard(object))
	}

	return result, nil
}

func (o *Rbac) Name() string {
	return o.name
}

func (o *Rbac) Desc() string {
	return o.desc
}
