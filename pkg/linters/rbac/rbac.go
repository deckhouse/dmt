package rbac

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/rbac/roles"
)

// Rbac linter
type Rbac struct {
	name string
}

func Run(m *module.Module) {
	if m == nil {
		return
	}

	o := &Rbac{
		name: "rbac",
	}
	lintError := errors.NewError(o.name, m.GetName())
	roles.Cfg = &config.Cfg.LintersSettings.Rbac

	for _, object := range m.GetStorage() {
		roles.ObjectUserAuthzClusterRolePath(m, object, lintError)
		roles.ObjectRBACPlacement(m, object, lintError)
		roles.ObjectBindingSubjectServiceAccountCheck(m, object, m.GetObjectStore(), lintError)
		roles.ObjectRolesWildcard(object, lintError)
	}
}
