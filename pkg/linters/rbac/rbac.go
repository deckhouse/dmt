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
	cfg        *config.RbacSettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Rbac {
	roles.Cfg = &cfg.LintersSettings.Rbac

	return &Rbac{
		name:      "rbac",
		desc:      "Lint rbac objects",
		cfg:       &cfg.LintersSettings.Rbac,
		ErrorList: errorList,
	}
}

func (o *Rbac) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(o.Name(), m.GetName()).WithMaxLevel(o.cfg.Impact)
	if m == nil {
		return result
	}

	for _, object := range m.GetStorage() {
		result.Merge(roles.ObjectUserAuthzClusterRolePath(m, object))
		result.Merge(roles.ObjectRBACPlacement(m, object))
		result.Merge(roles.ObjectBindingSubjectServiceAccountCheck(m, object, m.GetObjectStore()))
		result.Merge(roles.ObjectRolesWildcard(m, object))
	}

	result.CorrespondToMaxLevel()

	o.ErrorList.Merge(result)

	return result
}

func (o *Rbac) Name() string {
	return o.name
}

func (o *Rbac) Desc() string {
	return o.desc
}
