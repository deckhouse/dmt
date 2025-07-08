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
	"strings"

	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
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

func NewKubeRbacProxyRuleTracked(trackedRule *exclusions.TrackedStringRule) *KubeRbacProxyRuleTracked {
	return &KubeRbacProxyRuleTracked{
		RuleMeta: pkg.RuleMeta{
			Name: KubeRbacProxyRuleName,
		},
		StringRule: trackedRule.StringRule,
		trackedRule: trackedRule,
	}
}

type KubeRbacProxyRule struct {
	pkg.RuleMeta
	pkg.StringRule
}

type KubeRbacProxyRuleTracked struct {
	pkg.RuleMeta
	pkg.StringRule
	trackedRule *exclusions.TrackedStringRule
}

func (r *KubeRbacProxyRuleTracked) NamespaceMustContainKubeRBACProxyCA(objectStore *storage.UnstructuredObjectStore, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	proxyInNamespaces := set.New()

	for index := range objectStore.Storage {
		if index.Kind == "ConfigMap" && index.Name == "kube-rbac-proxy-ca.crt" {
			proxyInNamespaces.Add(index.Namespace)
		}
	}

	for index, object := range objectStore.Storage {
		if !strings.HasPrefix(index.Namespace, "d8-") {
			// skip non-deckhouse namespaces
			continue
		}
		
		// Use tracked rule to check if namespace should be excluded and mark exclusions as used
		if !r.trackedRule.Enabled(index.Name) {
			continue
		}
		
		errorList = errorList.WithFilePath(object.ShortPath())
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

func (r *KubeRbacProxyRule) NamespaceMustContainKubeRBACProxyCA(objectStore *storage.UnstructuredObjectStore, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	proxyInNamespaces := set.New()

	for index := range objectStore.Storage {
		if index.Kind == "ConfigMap" && index.Name == "kube-rbac-proxy-ca.crt" {
			proxyInNamespaces.Add(index.Namespace)
		}
	}

	for index, object := range objectStore.Storage {
		if !strings.HasPrefix(index.Namespace, "d8-") {
			// skip non-deckhouse namespaces
			continue
		}
		errorList = errorList.WithFilePath(object.ShortPath())
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
