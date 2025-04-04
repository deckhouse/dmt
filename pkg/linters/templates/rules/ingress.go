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

	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	IngressRuleName = "ingress-rules"
	snippet         = `{{ include "helm_lib_module_ingress_configuration_snippet" . | nindent 6 }}`
)

type IngressRule struct {
	pkg.RuleMeta
	pkg.KindRule
}

func NewIngressRule(excludeRules []pkg.KindRuleExclude) *IngressRule {
	return &IngressRule{
		RuleMeta: pkg.RuleMeta{
			Name: IngressRuleName,
		},
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
	}
}

func (r *IngressRule) CheckSnippetsRule(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName()).WithFilePath(object.ShortPath())

	switch object.Unstructured.GetKind() {
	case "Ingress":
	default:
		return
	}

	if !r.Enabled(object.Unstructured.GetKind(), object.Unstructured.GetName()) {
		fmt.Printf("⚠️ Skip Ingress %q due to exclusion rule.\n", object.Unstructured.GetName())
		return
	}

	converter := runtime.DefaultUnstructuredConverter
	ingress := new(v1.Ingress)
	if err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), ingress); err != nil {
		errorList.WithObjectID(object.Unstructured.GetName()).
			Errorf("Cannot convert object to %s: %v", object.Unstructured.GetKind(), err)

		return
	}

	for key, value := range ingress.ObjectMeta.GetAnnotations() {
		if key == "nginx.ingress.kubernetes.io/configuration-snippet" {
			if !strings.Contains(value, "add_header Strict-Transport-Security") {
				errorList.WithObjectID(object.Unstructured.GetName()).
					Errorf("Ingress annotation %q does not contain required snippet %q.", key, snippet)
			}
		}
	}
}
