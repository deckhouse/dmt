package k8sresources

import (
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/k8s-resources/pdb"
	rbacproxy "github.com/deckhouse/dmt/pkg/linters/k8s-resources/rbac-proxy"
	"github.com/deckhouse/dmt/pkg/linters/k8s-resources/vpa"
)

const (
	CrdsDir = "crds"
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

	rbacproxy.NamespaceMustContainKubeRBACProxyCA(m.GetName(), m.GetObjectStore())
	vpa.ControllerMustHaveVPA(m, lintError)
	pdb.ControllerMustHavePDB(m)
	pdb.DaemonSetMustNotHavePDB(m)

	for _, object := range m.GetStorage() {
		applyContainerRules(m.GetName(), object)
	}

	if isExistsOnFilesystem(m.GetPath(), CrdsDir) {
		CrdsModuleRule(m.GetName(), filepath.Join(m.GetPath(), CrdsDir))
	}
}

func isExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}
