package rbacproxy

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ID = "kube-rbac-proxy-resources"
)

// Object linter
type Object struct {
	name, desc string
	cfg        *config.K8SResourcesSettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Object {
	skipKubeRbacProxyChecks = cfg.LintersSettings.K8SResources.SkipKubeRbacProxyChecks

	return &Object{
		name:      "kube-rbac-proxy-resources",
		desc:      "Lint kube-rbac-proxy-resources",
		cfg:       &cfg.LintersSettings.K8SResources,
		ErrorList: errorList,
	}
}

func (o *Object) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(o.Name(), m.GetName()).WithMaxLevel(o.cfg.Impact)
	if m == nil {
		return result
	}

	result.Merge(namespaceMustContainKubeRBACProxyCA(m.GetName(), m.GetObjectStore()))

	result.CorrespondToMaxLevel()

	o.ErrorList.Merge(result)

	return result
}

func (o *Object) Name() string {
	return o.name
}

func (o *Object) Desc() string {
	return o.desc
}
