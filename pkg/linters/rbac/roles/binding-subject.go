/*
Copyright 2021 Flant JSC

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

package roles

import (
	"slices"

	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

//nolint:gocyclo // because
func ObjectBindingSubjectServiceAccountCheck(
	m *module.Module,
	object storage.StoreObject,
	objectStore *storage.UnstructuredObjectStore,
) *errors.LintRuleError {
	if slices.Contains(Cfg.SkipModuleCheckBinding, m.GetName()) {
		return nil
	}

	converter := runtime.DefaultUnstructuredConverter

	var subjects []v1.Subject

	// deckhouse module should contain only global cluster roles
	objectKind := object.Unstructured.GetKind()
	switch objectKind {
	case "ClusterRoleBinding":
		clusterRoleBinding := new(v1.ClusterRoleBinding)
		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), clusterRoleBinding)
		if err != nil {
			panic(err)
		}
		subjects = clusterRoleBinding.Subjects
	case "RoleBinding":
		roleBinding := new(v1.RoleBinding)
		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), roleBinding)
		if err != nil {
			panic(err)
		}
		subjects = roleBinding.Subjects

	default:
		return nil
	}

	for _, subject := range subjects {
		if subject.Kind != "ServiceAccount" {
			continue
		}

		// Prometheus service account has bindings across helm to scrape metrics.
		if subject.Name == "prometheus" && subject.Namespace == "d8-monitoring" {
			continue
		}

		// Grafana service account has binding in loki module.
		if m.GetName() == "loki" && subject.Name == "grafana" && subject.Namespace == "d8-monitoring" {
			continue
		}

		// Log-shipper service account has binding in loki module.
		if m.GetPath() == "loki" && subject.Name == "log-shipper" && subject.Namespace == "d8-log-shipper" {
			continue
		}

		if subject.Namespace == m.GetNamespace() && !objectStore.Exists(storage.ResourceIndex{
			Name: subject.Name, Kind: subject.Kind, Namespace: subject.Namespace,
		}) {
			return errors.NewLintRuleError(
				ID,
				object.Identity(),
				m.GetName(),
				nil,
				"%s bind to the wrong ServiceAccount (doesn't exist in the store)", objectKind,
			)
		}
	}

	return nil
}
