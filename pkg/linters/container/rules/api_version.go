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
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	APIVersionRuleName = "object-api-version"
)

func NewAPIVersionRule() *APIVersionRule {
	return &APIVersionRule{
		RuleMeta: pkg.RuleMeta{
			Name: APIVersionRuleName,
		},
	}
}

type APIVersionRule struct {
	pkg.RuleMeta
}

func (r *APIVersionRule) ObjectAPIVersion(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

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
