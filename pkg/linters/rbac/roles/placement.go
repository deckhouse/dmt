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
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	serviceAccountNameDelimiter = "-"
	UserAuthzClusterRolePath    = "templates/user-authz-cluster-roles.yaml"
	RootRBACForUsPath           = "templates/rbac-for-us.yaml"
	RootRBACToUsPath            = "templates/rbac-to-us.yaml"
	RBACv2Path                  = "templates/rbac"
)

func isSystemNamespace(actual string) bool {
	return actual == "default" || actual == "kube-system"
}

func isDeckhouseSystemNamespace(actual string) bool {
	return actual == "d8-monitoring" ||
		actual == "d8-system" ||
		// Temporary code required to ignore existing objects with incorrect naming
		// TODO: remove next lines after RBAC objects naming fixes
		actual == "d8-admission-policy-engine" ||
		actual == "d8-operator-trivy" ||
		actual == "d8-log-shipper" ||
		actual == "d8-local-path-provisioner"
}

func ObjectRBACPlacement(m *module.Module, object storage.StoreObject, lintError *errors.Error) {
	if slices.Contains(Cfg.SkipObjectCheckBinding, m.GetName()) {
		return
	}
	if object.ShortPath() == UserAuthzClusterRolePath || strings.HasPrefix(object.ShortPath(), RBACv2Path) {
		return
	}

	objectKind := object.Unstructured.GetKind()
	switch objectKind {
	case "ServiceAccount":
		objectRBACPlacementServiceAccount(m, object, lintError)
	case "ClusterRole", "ClusterRoleBinding":
		objectRBACPlacementClusterRole(objectKind, m, object, lintError)
	case "Role", "RoleBinding":
		objectRBACPlacementRole(objectKind, m, object, lintError)
	default:
		shortPath := object.ShortPath()
		if strings.HasSuffix(shortPath, "rbac-for-us.yaml") || strings.HasSuffix(shortPath, "rbac-to-us.yaml") {
			lintError.WithObjectID(object.Identity()).Add("kind %s not allowed in %q", objectKind, shortPath)
		}
	}
}

//nolint:gocyclo // because
func objectRBACPlacementServiceAccount(m *module.Module, object storage.StoreObject, lintError *errors.Error) {
	objectName := object.Unstructured.GetName()
	shortPath := object.ShortPath()
	namespace := object.Unstructured.GetNamespace()

	if shortPath == RootRBACForUsPath {
		if isSystemNamespace(namespace) {
			if objectName != "d8-"+m.GetName() {
				lintError.WithObjectID(object.Identity()).Add(
					"Name of ServiceAccount in %q in namespace %q should be equal to d8- + Chart Name (d8-%s)",
					RootRBACForUsPath, namespace, m.GetName(),
				)
			}
			return
		}
		if objectName != m.GetName() {
			lintError.WithObjectID(object.Identity()).Add(
				"Name of ServiceAccount in %q should be equal to Chart Name (%s)",
				RootRBACForUsPath, m.GetName(),
			)
			return
		}
		if !isDeckhouseSystemNamespace(namespace) && m.GetNamespace() != namespace {
			lintError.WithObjectID(object.Identity()).Add(
				"ServiceAccount should be deployed to \"d8-system\", \"d8-monitoring\" or %q", m.GetNamespace(),
			)
		}
		return
	} else if strings.HasSuffix(shortPath, "rbac-for-us.yaml") {
		parts := strings.Split(
			strings.TrimPrefix(strings.TrimSuffix(shortPath, "/rbac-for-us.yaml"), "templates/"),
			string(os.PathSeparator),
		)

		serviceAccountName := strings.Join(parts, serviceAccountNameDelimiter)
		expectedServiceAccountName := m.GetName() + serviceAccountNameDelimiter + serviceAccountName

		if isSystemNamespace(namespace) {
			if objectName != "d8-"+expectedServiceAccountName {
				lintError.WithObjectID(object.Identity()).Add(
					"Name of ServiceAccount in %q in namespace %q should be equal to d8-%s",
					shortPath, namespace, expectedServiceAccountName,
				)
			}
			return
		}
		if objectName == serviceAccountName {
			if m.GetNamespace() != namespace {
				lintError.WithObjectID(object.Identity()).Add(
					"ServiceAccount should be deployed to %q", m.GetNamespace(),
				)
			}
			return
		} else if objectName == expectedServiceAccountName {
			if !isDeckhouseSystemNamespace(namespace) {
				lintError.WithObjectID(object.Identity()).WithValue(namespace).
					Add("ServiceAccount should be deployed to \"d8-system\" or \"d8-monitoring\"")
			}
			return
		}

		if strings.HasPrefix(objectName, "istiod") && namespace == "d8-istio" {
			// istiod Deployment is rendered by istio-operator with serviceAccountName according to its
			// naming conventions we can't change (i.e. istiod-v1x19).
			// In our convention it has to be named as "iop" according to template folder, but within the folder we render
			// not a single istiod instance, but several for different versions and can't use the shared ServiceAccount for them.
			return
		}

		lintError.WithObjectID(object.Identity()).Add(
			"Name of ServiceAccount should be equal to %q or %q",
			serviceAccountName, expectedServiceAccountName,
		)
		return
	}
	lintError.WithObjectID(object.Identity()).Add(
		"ServiceAccount should be in %q or \"*/rbac-for-us.yaml\"", RootRBACForUsPath,
	)
}

func objectRBACPlacementClusterRole(kind string, m *module.Module, object storage.StoreObject, lintError *errors.Error) {
	objectName := object.Unstructured.GetName()
	shortPath := object.ShortPath()

	name := "d8:" + m.GetName()
	switch {
	case shortPath == RootRBACForUsPath:
		if !strings.HasPrefix(objectName, name) {
			lintError.WithObjectID(object.Identity()).Add(
				"Name of %s in %q should start with %q",
				kind, RootRBACForUsPath, name,
			)
			return
		}
	case strings.HasSuffix(shortPath, "rbac-for-us.yaml"):
		parts := strings.Split(
			strings.TrimPrefix(strings.TrimSuffix(shortPath, "/rbac-for-us.yaml"), "templates/"),
			string(os.PathSeparator),
		)
		n := name + ":" + strings.Join(parts, ":")
		if !strings.HasPrefix(objectName, name) {
			lintError.WithObjectID(object.Identity()).Add(
				"Name of %s should start with %q",
				kind, n,
			)
			return
		}
	default:
		lintError.WithObjectID(object.Identity()).Add(
			"%s should be in %q or \"*/rbac-for-us.yaml\"",
			kind, RootRBACForUsPath,
		)
		return
	}
}

func objectRBACPlacementRole(kind string, m *module.Module, object storage.StoreObject, lintError *errors.Error) {
	objectName := object.Unstructured.GetName()
	shortPath := object.ShortPath()
	namespace := object.Unstructured.GetNamespace()

	switch {
	case shortPath == RootRBACForUsPath:
		handleRootRBACForUs(m, object, objectName, kind, lintError)
	case shortPath == RootRBACToUsPath:
		handleRootRBACToUs(m, object, objectName, kind, lintError)
	case strings.HasSuffix(shortPath, "rbac-for-us.yaml"):
		handleNestedRBACForUs(m, object, shortPath, objectName, namespace, kind, lintError)
	case strings.HasSuffix(shortPath, "rbac-to-us.yaml"):
		handleNestedRBACToUs(m, object, shortPath, objectName, kind, lintError)
	default:
		msgTemplate := `%s should be in "templates/rbac-for-us.yaml", "templates/rbac-to-us.yaml", ".*/rbac-to-us.yaml" or ".*/rbac-for-us.yaml"`
		lintError.WithObjectID(object.Identity()).Add(msgTemplate, kind)
	}
}

// handleRootRBACForUs applies to templates/rbac-for-us.yaml file's objects
func handleRootRBACForUs(m *module.Module, object storage.StoreObject, objectName, kind string, lintError *errors.Error) {
	prefix := "d8:" + m.GetName()
	namespace := object.Unstructured.GetNamespace()

	switch {
	case objectName == m.GetName() && namespace != m.GetNamespace():
		if !isDeckhouseSystemNamespace(namespace) {
			lintError.WithObjectID(object.Identity()).Add(
				"%s in %q should be deployed in namespace \"d8-monitoring\", \"d8-system\" or %q",
				kind, RootRBACForUsPath, m.GetNamespace(),
			)
			return
		}
	case strings.HasPrefix(objectName, prefix):
		if !isSystemNamespace(namespace) {
			lintError.WithObjectID(object.Identity()).Add(
				"%s in %q should be deployed in namespace \"default\" or \"kube-system\"",
				kind, RootRBACForUsPath,
			)
			return
		}
	case !strings.HasPrefix(objectName, prefix):
		if !isDeckhouseSystemNamespace(namespace) {
			lintError.WithObjectID(object.Identity()).Add(
				"%s in %q should be deployed in namespace %q",
				kind, RootRBACForUsPath, m.GetNamespace(),
			)
			return
		}
	}
}

// handleRootRBACToUs applies to templates/rbac-to-us.yaml file's objects
func handleRootRBACToUs(m *module.Module, object storage.StoreObject, objectName, kind string, lintError *errors.Error) {
	prefix := "access-to-" + m.GetName()
	if !strings.HasPrefix(objectName, prefix) {
		lintError.WithObjectID(object.Identity()).Add(
			"%s in %q should start with %q",
			kind, RootRBACToUsPath, prefix,
		)
		return
	}

	namespace := object.Unstructured.GetNamespace()
	if !isDeckhouseSystemNamespace(namespace) && namespace != m.GetNamespace() {
		lintError.WithObjectID(object.Identity()).Add(
			"%s in %q should be deployed in namespace \"d8-system\", \"d8-monitoring\" or %q",
			kind, RootRBACToUsPath, m.GetNamespace(),
		)
		return
	}
}

// handleNestedRBACForUs applies to templates/**/rbac-for-us.yaml file's objects
func handleNestedRBACForUs(m *module.Module, object storage.StoreObject, shortPath, objectName, namespace, kind string, lintError *errors.Error) {
	if m == nil {
		return
	}
	parts := strings.Split(
		strings.TrimPrefix(strings.TrimSuffix(shortPath, "/rbac-for-us.yaml"), "templates/"),
		string(os.PathSeparator),
	)
	localPrefix := strings.Join(parts, ":")
	globalPrefix := fmt.Sprintf("%s:%s", m.GetName(), strings.Join(parts, ":"))
	systemPrefix := fmt.Sprintf("d8:%s", globalPrefix)

	switch {
	case strings.HasPrefix(objectName, localPrefix):
		if namespace != m.GetNamespace() {
			lintError.WithObjectID(object.Identity()).Add(
				"%s with prefix %q should be deployed in namespace %q",
				kind, localPrefix, m.GetNamespace(),
			)
			return
		}
	case strings.HasPrefix(objectName, globalPrefix):
		if !isDeckhouseSystemNamespace(namespace) {
			lintError.WithObjectID(object.Identity()).Add(
				"%s with prefix %q should be deployed in namespace \"d8-system\" or \"d8-monitoring\"",
				kind, globalPrefix,
			)
			return
		}
	case strings.HasPrefix(objectName, systemPrefix):
		if !isSystemNamespace(namespace) {
			lintError.WithObjectID(object.Identity()).Add(
				"%s with prefix %q should be deployed in namespace \"default\" or \"kube-system\"",
				kind, systemPrefix,
			)
			return
		}
	default:
		lintError.WithObjectID(object.Identity()).Add(
			"%s in %q should start with %q or %q",
			kind, shortPath, localPrefix, globalPrefix,
		)
		return
	}
}

// handleNestedRBACToUs applies to templates/**/rbac-to-us.yaml file's objects
func handleNestedRBACToUs(m *module.Module, object storage.StoreObject, shortPath, objectName, kind string, lintError *errors.Error) {
	parts := strings.Split(
		strings.TrimPrefix(strings.TrimSuffix(shortPath, "/rbac-to-us.yaml"), "templates/"),
		string(os.PathSeparator),
	)

	localPrefix := fmt.Sprintf("access-to-%s-", strings.Join(parts, "-"))
	globalPrefix := fmt.Sprintf("access-to-%s-%s-", m.GetName(), strings.Join(parts, "-"))
	namespace := object.Unstructured.GetNamespace()

	switch {
	case strings.HasPrefix(objectName, localPrefix):
		if namespace != m.GetNamespace() {
			lintError.WithObjectID(object.Identity()).Add(
				"%s with prefix %q should be deployed in namespace %q",
				kind, globalPrefix, m.GetNamespace(),
			)
			return
		}
	case strings.HasPrefix(objectName, globalPrefix):
		if !isDeckhouseSystemNamespace(namespace) {
			lintError.WithObjectID(object.Identity()).Add(
				"%s with prefix %q should be deployed in namespace \"d8-system\" or \"d8-monitoring\"",
				kind, globalPrefix,
			)
			return
		}
	default:
		lintError.WithObjectID(object.Identity()).Add(
			"%s should start with %q or %q", kind, localPrefix, globalPrefix,
		)
		return
	}
}
