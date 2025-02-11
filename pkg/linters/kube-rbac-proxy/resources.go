package rbacproxy

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ID = "kube-rbac-proxy-resources"
)

// KubeRbacProxy linter
type KubeRbacProxy struct {
	name, desc string
	cfg        *config.K8SResourcesSettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *KubeRbacProxy {
	return &KubeRbacProxy{
		name:      ID,
		desc:      "Lint kube-rbac-proxy-resources",
		cfg:       &cfg.LintersSettings.K8SResources,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.K8SResources.Impact),
	}
}

func (l *KubeRbacProxy) Run(m *module.Module) {
	if m == nil {
		return
	}

	l.namespaceMustContainKubeRBACProxyCA(m.GetName(), m.GetObjectStore())
}

func (l *KubeRbacProxy) Name() string {
	return l.name
}

func (l *KubeRbacProxy) Desc() string {
	return l.desc
}
