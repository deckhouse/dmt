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
	lintError *errors.Error,
) {
	if slices.Contains(Cfg.SkipModuleCheckBinding, m.GetName()) {
		return
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
			lintError.WithObjectID(object.Identity()).Add(
				"Cannot convert object to %s: %v", object.Unstructured.GetKind(), err)
			return
		}
		subjects = clusterRoleBinding.Subjects
	case "RoleBinding":
		roleBinding := new(v1.RoleBinding)
		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), roleBinding)
		if err != nil {
			lintError.WithObjectID(object.Identity()).Add(
				"Cannot convert object to %s: %v", object.Unstructured.GetKind(), err)
			return
		}
		subjects = roleBinding.Subjects

	default:
		return
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
			lintError.WithObjectID(object.Identity()).Add(
				"%s bind to the wrong ServiceAccount (doesn't exist in the store)", objectKind,
			)
			return
		}
	}
}
