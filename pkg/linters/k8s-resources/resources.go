package k8sresources

import (
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/k8s-resources/pdb"
	rbacproxy "github.com/deckhouse/dmt/pkg/linters/k8s-resources/rbac-proxy"
)

const (
	ID      = "k8s-resources"
	CrdsDir = "crds"
)

// Object linter
type Object struct {
	name, desc string
	cfg        *config.K8SResourcesSettings
}

var Cfg *config.K8SResourcesSettings

func New(cfg *config.K8SResourcesSettings) *Object {
	Cfg = cfg
	pdb.SkipPDBChecks = cfg.SkipPDBChecks
	rbacproxy.SkipKubeRbacProxyChecks = cfg.SkipKubeRbacProxyChecks

	return &Object{
		name: "k8s-resources",
		desc: "Lint k8s-resources",
		cfg:  cfg,
	}
}

func (o *Object) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(o.Name(), m.GetName())
	if m == nil {
		return result
	}

	name := m.GetName()
	result.Merge(rbacproxy.NamespaceMustContainKubeRBACProxyCA(name, m.GetObjectStore()))
	result.Merge(pdb.ControllerMustHavePDB(m))
	result.Merge(pdb.DaemonSetMustNotHavePDB(m))

	if isExistsOnFilesystem(m.GetPath(), CrdsDir) {
		result.Merge(CrdsModuleRule(m.GetName(), filepath.Join(m.GetPath(), CrdsDir)))
	}

	return result
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
