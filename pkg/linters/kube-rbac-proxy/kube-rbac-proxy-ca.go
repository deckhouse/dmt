package rbacproxy

import (
	"fmt"
	"slices"

	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/internal/storage"
)

func (l *KubeRbacProxy) namespaceMustContainKubeRBACProxyCA(moduleName string, objectStore *storage.UnstructuredObjectStore) {
	errorList := l.ErrorList.WithModule(moduleName)

	proxyInNamespaces := set.New()

	for index := range objectStore.Storage {
		if index.Kind == "ConfigMap" && index.Name == "kube-rbac-proxy-ca.crt" {
			proxyInNamespaces.Add(index.Namespace)
		}
	}

	for index := range objectStore.Storage {
		if index.Kind == "Namespace" {
			if slices.Contains(l.cfg.SkipKubeRbacProxyChecks, index.Namespace) {
				continue
			}

			if !proxyInNamespaces.Has(index.Name) {
				errorList.WithObjectID(fmt.Sprintf("namespace = %s", index.Name)).
					WithValue(proxyInNamespaces.Slice()).
					Error("All system namespaces should contain kube-rbac-proxy CA certificate." +
						"\n\tConsider using corresponding helm_lib helper 'helm_lib_kube_rbac_proxy_ca_certificate'.")
			}
		}
	}
}
