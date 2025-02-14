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

func (r *KubeRbacProxyRule) NamespaceMustContainKubeRBACProxyCA(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName()).WithFilePath(object.ShortPath())

	if !r.Enabled(object.Unstructured.GetNamespace()) {
		// TODO: add metrics
		return
	}

	proxyInNamespaces := set.New()

	if object.Unstructured.GetKind() == "ConfigMap" && object.Unstructured.GetName() == "kube-rbac-proxy-ca.crt" {
		proxyInNamespaces.Add(object.Unstructured.GetNamespace())
	}

	if object.Unstructured.GetKind() == "Namespace" {
		if !proxyInNamespaces.Has(object.Unstructured.GetName()) {
			errorList.WithObjectID(fmt.Sprintf("namespace = %s", object.Unstructured.GetName())).
				WithValue(proxyInNamespaces.Slice()).
				Error("All system namespaces should contain kube-rbac-proxy CA certificate." +
					"\n\tConsider using corresponding helm_lib helper 'helm_lib_kube_rbac_proxy_ca_certificate'.")
		}
	}
}
