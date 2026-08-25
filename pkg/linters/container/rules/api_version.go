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
	APIVersionRuleName = "object-api-version"
)

func NewAPIVersionRule(m pkg.Module, errorList *errors.LintRuleErrorsList) *APIVersionRule {
	return &APIVersionRule{
		RuleMeta: pkg.RuleMeta{
			Name: APIVersionRuleName,
		},
		module:    m,
		errorList: errorList.WithRule(APIVersionRuleName),
	}
}

type APIVersionRule struct {
	pkg.RuleMeta

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*APIVersionRule)(nil)

func (r *APIVersionRule) Check(_ context.Context) {
	for _, object := range r.module.GetStorage() {
		r.checkObject(object)
	}
}

// checkObject must stay a separate method rather than being inlined into the
// loop above: its early returns end the check for one object, not for the rule.
func (r *APIVersionRule) checkObject(object storage.StoreObject) {
	errorList := r.errorList.WithFilePath(object.GetPath())

	version := object.Unstructured.GetAPIVersion()

	switch object.Unstructured.GetKind() {
	case "Role", "RoleBinding", "ClusterRole", "ClusterRoleBinding":
		compareAPIVersion("rbac.authorization.k8s.io/v1", version, object.Identity(), errorList)
	case "Deployment", "DaemonSet", "StatefulSet":
		compareAPIVersion("apps/v1", version, object.Identity(), errorList)
	case "Ingress":
		compareAPIVersion("networking.k8s.io/v1", version, object.Identity(), errorList)
	case "PriorityClass":
		compareAPIVersion("scheduling.k8s.io/v1", version, object.Identity(), errorList)
	case "PodSecurityPolicy":
		compareAPIVersion("policy/v1beta1", version, object.Identity(), errorList)
	case "NetworkPolicy":
		compareAPIVersion("networking.k8s.io/v1", version, object.Identity(), errorList)
	}
}

func compareAPIVersion(wanted, version, objectID string, errorList *errors.LintRuleErrorsList) {
	if version != wanted {
		errorList.WithObjectID(objectID).
			Errorf("Object defined using deprecated api version, wanted %q", wanted)
	}
}
