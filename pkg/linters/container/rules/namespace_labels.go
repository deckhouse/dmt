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
	"context"
	"strings"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	NamespaceLabelsRuleName = "object-namespace-labels"
)

func NewNamespaceLabelsRule(excludeRules []pkg.KindRuleExclude,
	m pkg.Module, errorList *errors.LintRuleErrorsList) *NamespaceLabelsRule {
	return &NamespaceLabelsRule{
		RuleMeta: pkg.RuleMeta{
			Name: NamespaceLabelsRuleName,
		},
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
		module:    m,
		errorList: errorList.WithRule(NamespaceLabelsRuleName),
	}
}

type NamespaceLabelsRule struct {
	pkg.RuleMeta
	pkg.KindRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*NamespaceLabelsRule)(nil)

func (r *NamespaceLabelsRule) Check(_ context.Context) {
	storageMap := r.module.GetStorage()

	for _, object := range storageMap {
		r.checkObject(object, storageMap)
	}
}

// checkObject must stay a separate method rather than being inlined into the
// loop above: its early returns end the check for one object, not for the rule.
func (r *NamespaceLabelsRule) checkObject(object storage.StoreObject, storageMap map[storage.ResourceIndex]storage.StoreObject) {
	errorList := r.errorList.WithFilePath(object.GetPath())

	if object.Unstructured.GetKind() != "Namespace" || !strings.HasPrefix(object.Unstructured.GetName(), "d8-") {
		return
	}

	namespaceName := object.Unstructured.GetName()

	if !r.Enabled(object.Unstructured.GetKind(), namespaceName) {
		// TODO: add metrics
		return
	}

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
