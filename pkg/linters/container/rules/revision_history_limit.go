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

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	RevisionHistoryLimitRuleName = "object-revision-history-limit"
)

func NewRevisionHistoryLimitRule(m pkg.Module, errorList *errors.LintRuleErrorsList) *RevisionHistoryLimitRule {
	return &RevisionHistoryLimitRule{
		RuleMeta: pkg.RuleMeta{
			Name: RevisionHistoryLimitRuleName,
		},
		module:    m,
		errorList: errorList.WithRule(RevisionHistoryLimitRuleName),
	}
}

type RevisionHistoryLimitRule struct {
	pkg.RuleMeta

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*RevisionHistoryLimitRule)(nil)

func (r *RevisionHistoryLimitRule) Check(_ context.Context) {
	for _, object := range r.module.GetStorage() {
		r.checkObject(object)
	}
}

// checkObject must stay a separate method rather than being inlined into the
// loop above: its early returns end the check for one object, not for the rule.
func (r *RevisionHistoryLimitRule) checkObject(object storage.StoreObject) {
	errorList := r.errorList.WithFilePath(object.GetPath())

	if object.Unstructured.GetKind() == "Deployment" {
		converter := runtime.DefaultUnstructuredConverter
		deployment := new(appsv1.Deployment)

		if err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), deployment); err != nil {
			errorList.WithObjectID(object.Identity()).
				Errorf("Cannot convert object to %s: %v", object.Unstructured.GetKind(), err)

			return
		}

		// https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#revision-history-limit
		// Revision history limit controls the number of replicasets stored in the cluster for each deployment.
		// Higher number means higher resource consumption, lower means inability to rollback.
		//
		// Since Deckhouse does not use rollback, we can set it to 2 to be able to manually check the previous version.
		// It is more important to reduce the control plane pressure.
		maxHistoryLimit := int32(2)
		actualLimit := deployment.Spec.RevisionHistoryLimit

		if actualLimit == nil {
			errorList.WithObjectID(object.Identity()).
				Errorf("Deployment spec.revisionHistoryLimit must be less or equal to %d", maxHistoryLimit)
		} else if *actualLimit > maxHistoryLimit {
			errorList.WithObjectID(object.Identity()).WithValue(*actualLimit).
				Errorf("Deployment spec.revisionHistoryLimit must be less or equal to %d", maxHistoryLimit)
		}
	}
}
