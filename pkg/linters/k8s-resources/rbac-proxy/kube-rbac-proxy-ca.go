package rbacproxy

import (
	"fmt"
	"slices"

	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

var SkipKubeRbacProxyChecks []string

func NamespaceMustContainKubeRBACProxyCA(moduleName string, objectStore *storage.UnstructuredObjectStore) {
	lintError := errors.NewError("kube-rbac-proxy-ca", moduleName)
	proxyInNamespaces := set.New()

	for index := range objectStore.Storage {
		if index.Kind == "ConfigMap" && index.Name == "kube-rbac-proxy-ca.crt" {
			proxyInNamespaces.Add(index.Namespace)
		}
	}

	for index := range objectStore.Storage {
		if index.Kind == "Namespace" {
			if slices.Contains(SkipKubeRbacProxyChecks, index.Namespace) {
				continue
			}
			if !proxyInNamespaces.Has(index.Name) {
				lintError.WithObjectID(fmt.Sprintf("namespace = %s", index.Name)).
					WithValue(proxyInNamespaces.Slice()).
					Add("All system namespaces should contain kube-rbac-proxy CA certificate." +
						"\n\tConsider using corresponding helm_lib helper 'helm_lib_kube_rbac_proxy_ca_certificate'.",
					)
			}
		}
	}
}
