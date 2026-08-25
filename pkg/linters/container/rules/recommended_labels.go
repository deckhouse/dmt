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

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	RecommendedLabelsRuleName = "object-recommended-labels"
)

func NewRecommendedLabelsRule(m pkg.Module, errorList *errors.LintRuleErrorsList) *RecommendedLabelsRule {
	return &RecommendedLabelsRule{
		RuleMeta: pkg.RuleMeta{
			Name: RecommendedLabelsRuleName,
		},
		module:    m,
		errorList: errorList.WithRule(RecommendedLabelsRuleName),
	}
}

type RecommendedLabelsRule struct {
	pkg.RuleMeta

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*RecommendedLabelsRule)(nil)

func (r *RecommendedLabelsRule) Check(_ context.Context) {
	for _, object := range r.module.GetStorage() {
		r.checkObject(object)
	}
}

// checkObject must stay a separate method rather than being inlined into the
// loop above: its early returns end the check for one object, not for the rule.
func (r *RecommendedLabelsRule) checkObject(object storage.StoreObject) {
	errorList := r.errorList.WithFilePath(object.GetPath())

	labels := object.Unstructured.GetLabels()
	if _, ok := labels["module"]; !ok {
		errorList.WithObjectID(object.Identity()).WithValue(labels).
			Error(`Object does not have the label "module"`)
	}

	if _, ok := labels["heritage"]; !ok {
		errorList.WithObjectID(object.Identity()).WithValue(labels).
			Error(`Object does not have the label "heritage"`)
	}
}
