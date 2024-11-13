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
	ID      = "object"
	CrdsDir = "crds"
)

// Object linter
type Object struct {
	name, desc string
	cfg        *config.K8SResourcesSettings
}

func New(cfg *config.K8SResourcesSettings) *Object {
	return &Object{
		name: "object",
		desc: "Lint objects",
		cfg:  cfg,
	}
}

func (*Object) Run(m *module.Module) (result errors.LintRuleErrorsList, err error) {
	if m == nil {
		return result, err
	}

	result.Merge(rbacproxy.NamespaceMustContainKubeRBACProxyCA(m.GetObjectStore()))
	result.Merge(vpa.ControllerMustHaveVPA(m))
	result.Merge(pdb.ControllerMustHavePDB(m))
	result.Merge(pdb.DaemonSetMustNotHavePDB(m))

	for _, object := range m.GetStorage() {
		result.Merge(applyContainerRules(object))
	}

	if isExistsOnFilesystem(m.GetPath(), CrdsDir) {
		result.Merge(CrdsModuleRule(m.GetName(), filepath.Join(m.GetPath(), CrdsDir)))
	}

	return result, nil
}

func (o *Object) Name() string {
	return o.name
}

func (o *Object) Desc() string {
	return o.desc
}

func isExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}
