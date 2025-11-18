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
	"strings"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	NamespaceLabelsRuleName = "object-namespace-labels"
)

func NewNamespaceLabelsRule() *NamespaceLabelsRule {
	return &NamespaceLabelsRule{
		RuleMeta: pkg.RuleMeta{
			Name: NamespaceLabelsRuleName,
		},
	}
}

type NamespaceLabelsRule struct {
	pkg.RuleMeta
}

func (r *NamespaceLabelsRule) ObjectNamespaceLabels(object storage.StoreObject, storageMap map[storage.ResourceIndex]storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	if object.Unstructured.GetKind() != "Namespace" || !strings.HasPrefix(object.Unstructured.GetName(), "d8-") {
		return
	}

	namespaceName := object.Unstructured.GetName()

	hasPrometheusRules := false
	for _, obj := range storageMap {
		if obj.Unstructured.GetKind() == "PrometheusRule" {
			if obj.Unstructured.GetNamespace() == namespaceName {
				hasPrometheusRules = true
				break
			}
		}
	}

	if !hasPrometheusRules {
		return
	}

	labels := object.Unstructured.GetLabels()

	if label := labels["prometheus.deckhouse.io/rules-watcher-enabled"]; label == "true" {
		return
	}

	errorList.WithObjectID(object.Identity()).WithValue(labels).
		Error(`Namespace object does not have the label "prometheus.deckhouse.io/rules-watcher-enabled"`)
}
