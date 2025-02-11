package k8sresources

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/k8s-resources/pdb"
	rbacproxy "github.com/deckhouse/dmt/pkg/linters/k8s-resources/rbac-proxy"
	"github.com/deckhouse/dmt/pkg/linters/k8s-resources/vpa"
)

// Resources linter
type Resources struct {
	name string
	cfg  *config.K8SResourcesSettings
}

func Run(m *module.Module) {
	if m == nil {
		return
	}

	o := &Resources{
		name: "k8s-resources",
		cfg:  &config.Cfg.LintersSettings.K8SResources,
	}

	pdb.SkipPDBChecks = o.cfg.SkipPDBChecks
	vpa.SkipVPAChecks = o.cfg.SkipVPAChecks
	rbacproxy.SkipKubeRbacProxyChecks = o.cfg.SkipKubeRbacProxyChecks

	lintError := errors.NewError(o.name, m.GetName())

	rbacproxy.NamespaceMustContainKubeRBACProxyCA(m.GetObjectStore(), lintError.WithLinterID("rbac-proxy"))
	vpa.ControllerMustHaveVPA(m, lintError.WithLinterID("vpa"))
	pdb.ControllerMustHavePDB(m, lintError.WithLinterID("pdb"))
	pdb.DaemonSetMustNotHavePDB(m, lintError.WithLinterID("pdb"))

	for _, object := range m.GetStorage() {
		if slices.Contains(o.cfg.SkipContainerChecks, object.Unstructured.GetName()) {
			continue
		}

		applyContainerRules(object, lintError)
	}

	if isExistsOnFilesystem(m.GetPath(), crdsDir) {
		CrdsModuleRule(m.GetName(), filepath.Join(m.GetPath(), crdsDir), lintError)
	}
}

func isExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}
