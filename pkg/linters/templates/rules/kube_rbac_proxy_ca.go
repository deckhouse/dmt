package rules

import (
	"fmt"

	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	KubeRbacProxyRuleName = "kube-rbac-proxy"
)

func NewKubeRbacProxyRule(excludeRules []pkg.StringRuleExclude) *KubeRbacProxyRule {
	return &KubeRbacProxyRule{
		RuleMeta: pkg.RuleMeta{
			Name: KubeRbacProxyRuleName,
		},
		StringRule: pkg.StringRule{
			ExcludeRules: excludeRules,
		},
	}
}

type KubeRbacProxyRule struct {
	pkg.RuleMeta
	pkg.StringRule
}

func (r *KubeRbacProxyRule) NamespaceMustContainKubeRBACProxyCA(objectStore *storage.UnstructuredObjectStore, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	proxyInNamespaces := set.New()

	for index := range objectStore.Storage {
		if index.Kind == "ConfigMap" && index.Name == "kube-rbac-proxy-ca.crt" {
			proxyInNamespaces.Add(index.Namespace)
		}
	}

	for index := range objectStore.Storage {
		if index.Kind == "Namespace" {
			if !proxyInNamespaces.Has(index.Name) {
				errorList.WithObjectID(fmt.Sprintf("namespace = %s", index.Name)).
					WithValue(proxyInNamespaces.Slice()).
					Error("All system namespaces should contain kube-rbac-proxy CA certificate." +
						"\n\tConsider using corresponding helm_lib helper 'helm_lib_kube_rbac_proxy_ca_certificate'.",
					)
			}
		}
	}
}
