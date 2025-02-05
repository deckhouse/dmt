package rbacproxy

import (
	"fmt"
	"slices"

	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

var skipKubeRbacProxyChecks []string

func namespaceMustContainKubeRBACProxyCA(md string, objectStore *storage.UnstructuredObjectStore) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList("kube-rbac-proxy-ca", md)
	proxyInNamespaces := set.New()

	for index := range objectStore.Storage {
		if index.Kind == "ConfigMap" && index.Name == "kube-rbac-proxy-ca.crt" {
			proxyInNamespaces.Add(index.Namespace)
		}
	}

	for index := range objectStore.Storage {
		if index.Kind == "Namespace" {
			if slices.Contains(skipKubeRbacProxyChecks, index.Namespace) {
				continue
			}
			if !proxyInNamespaces.Has(index.Name) {
				result.WithObjectID(fmt.Sprintf("namespace = %s", index.Name)).
					WithValue(proxyInNamespaces.Slice()).
					Add("All system namespaces should contain kube-rbac-proxy CA certificate." +
						"\n\tConsider using corresponding helm_lib helper 'helm_lib_kube_rbac_proxy_ca_certificate'.",
					)
			}
		}
	}

	return result
}
