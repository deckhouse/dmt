/*
Copyright 2025 Flant JSC

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
	errorList = errorList.WithRule(r.GetName())

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
