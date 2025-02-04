/*
Copyright 2022 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rbacproxy

import (
	"fmt"
	"slices"

	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

var SkipKubeRbacProxyChecks []string

func NamespaceMustContainKubeRBACProxyCA(md string, objectStore *storage.UnstructuredObjectStore) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList("kube-rbac-proxy-ca", md)
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
