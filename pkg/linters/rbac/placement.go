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

package rbac

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	serviceAccountNameDelimiter = "-"
	UserAuthzClusterRolePath    = "templates/user-authz-cluster-roles.yaml"
	RootRBACForUsPath           = "templates/rbac-for-us.yaml"
	RootRBACToUsPath            = "templates/rbac-to-us.yaml"
	RBACv2Path                  = "templates/rbac"
)

// TODO: remove entries after 'd8-system' after fixing RBAC objects names
var deckhouseNamespaces = []string{"d8-monitoring", "d8-system", "d8-admission-policy-engine", "d8-operator-trivy", "d8-log-shipper", "d8-local-path-provisioner"}

func isSystemNamespace(actual string) bool {
	return actual == metav1.NamespaceDefault || actual == metav1.NamespaceSystem
}

func isDeckhouseSystemNamespace(actual string) bool {
	return slices.Contains(deckhouseNamespaces, actual)
}

func (l *Rbac) objectRBACPlacement(m *module.Module) {
	if slices.Contains(l.cfg.SkipObjectCheckBinding, m.GetName()) {
		return
	}

	for _, object := range m.GetObjectStore().Storage {
		errorList := l.ErrorList.WithModule(m.GetName()).WithObjectID(object.Identity())

		shortPath := object.ShortPath()
		if shortPath == UserAuthzClusterRolePath || strings.HasPrefix(shortPath, RBACv2Path) {
			continue
		}

		objectKind := object.Unstructured.GetKind()
		switch objectKind {
		case "ServiceAccount":
			objectRBACPlacementServiceAccount(m, object, errorList)
		case "ClusterRole", "ClusterRoleBinding":
			objectRBACPlacementClusterRole(m, object, errorList)
		case "Role", "RoleBinding":
			objectRBACPlacementRole(m, object, errorList)
		default:
			if strings.HasSuffix(shortPath, "rbac-for-us.yaml") || strings.HasSuffix(shortPath, "rbac-to-us.yaml") {
				errorList.WithFilePath(shortPath).
					Errorf("kind %s not allowed", objectKind)
			}
		}
	}
}

func objectRBACPlacementServiceAccount(m *module.Module, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	objectName := object.Unstructured.GetName()
	shortPath := object.ShortPath()
	namespace := object.Unstructured.GetNamespace()
	errorList = errorList.WithFilePath(shortPath)

	if shortPath == RootRBACForUsPath {
		if isSystemNamespace(namespace) {
			if objectName != "d8-"+m.GetName() {
				errorList.Errorf("Name of ServiceAccount in %q in namespace %q should be equal to d8- + Chart Name (d8-%s)", RootRBACForUsPath, namespace, m.GetName())
			}
			return
		}

		if objectName != m.GetName() {
			errorList.Errorf("Name of ServiceAccount in %q should be equal to Chart Name (%s)", RootRBACForUsPath, m.GetName())
			return
		}

		if !isDeckhouseSystemNamespace(namespace) && m.GetNamespace() != namespace {
			errorList.Errorf("ServiceAccount should be deployed to \"d8-system\", \"d8-monitoring\" or %q", m.GetNamespace())
			return
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
				errorList.Errorf("Name of ServiceAccount in %q in namespace %q should be equal to d8-%s", shortPath, namespace, expectedServiceAccountName)
			}
			return
		}

		if objectName == serviceAccountName {
			if m.GetNamespace() != namespace {
				errorList.Errorf("ServiceAccount should be deployed to %q", m.GetNamespace())
			}
			return
		} else if objectName == expectedServiceAccountName {
			if !isDeckhouseSystemNamespace(namespace) {
				errorList.Error("ServiceAccount should be deployed to \"d8-system\" or \"d8-monitoring\"")
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

		errorList.Errorf("Name of ServiceAccount should be equal to %q or %q", serviceAccountName, expectedServiceAccountName)
		return
	}

	errorList.Errorf("ServiceAccount should be in %q or \"*/rbac-for-us.yaml\"", RootRBACForUsPath)
}

func objectRBACPlacementClusterRole(m *module.Module, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	objectName := object.Unstructured.GetName()
	objectKind := object.Unstructured.GetKind()
	shortPath := object.ShortPath()
	errorList = errorList.WithFilePath(shortPath)

	name := "d8:" + m.GetName()
	switch {
	case shortPath == RootRBACForUsPath:
		if !strings.HasPrefix(objectName, name) {
			errorList.Errorf("Name of %s in %q should start with %q", objectKind, RootRBACForUsPath, name)
		}

	case strings.HasSuffix(shortPath, "rbac-for-us.yaml"):
		parts := strings.Split(
			strings.TrimPrefix(strings.TrimSuffix(shortPath, "/rbac-for-us.yaml"), "templates/"),
			string(os.PathSeparator),
		)

		n := name + ":" + strings.Join(parts, ":")
		if !strings.HasPrefix(objectName, name) {
			errorList.Errorf("Name of %s should start with %q", objectKind, n)
		}

	default:
		errorList.Errorf("%s should be in %q or \"*/rbac-for-us.yaml\"", objectKind, RootRBACForUsPath)
	}
}

func objectRBACPlacementRole(m *module.Module, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	shortPath := object.ShortPath()
	errorList = errorList.WithFilePath(shortPath)

	switch {
	case shortPath == RootRBACForUsPath:
		handleRootRBACForUs(m, object, errorList)

	case shortPath == RootRBACToUsPath:
		handleRootRBACToUs(m, object, errorList)

	case strings.HasSuffix(shortPath, "rbac-for-us.yaml"):
		handleNestedRBACForUs(m, object, errorList)

	case strings.HasSuffix(shortPath, "rbac-to-us.yaml"):
		handleNestedRBACToUs(m, object, errorList)

	default:
		msgTemplate := `%s should be in "templates/rbac-for-us.yaml", "templates/rbac-to-us.yaml", ".*/rbac-to-us.yaml" or ".*/rbac-for-us.yaml"`
		errorList.Errorf(msgTemplate, object.Unstructured.GetKind())
	}
}

// handleRootRBACForUs applies to templates/rbac-for-us.yaml file's objects
func handleRootRBACForUs(m *module.Module, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	prefix := "d8:" + m.GetName()
	objectName := object.Unstructured.GetName()
	objectKind := object.Unstructured.GetKind()
	namespace := object.Unstructured.GetNamespace()

	switch {
	case objectName == m.GetName() && namespace != m.GetNamespace():
		if !isDeckhouseSystemNamespace(namespace) {
			errorList.Errorf("%s in %q should be deployed in namespace \"d8-monitoring\", \"d8-system\" or %q", objectKind, RootRBACForUsPath, m.GetNamespace())
		}

	case strings.HasPrefix(objectName, prefix):
		if !isSystemNamespace(namespace) {
			errorList.Errorf("%s in %q should be deployed in namespace \"default\" or \"kube-system\"", objectKind, RootRBACForUsPath)
		}

	case !strings.HasPrefix(objectName, prefix):
		if !isDeckhouseSystemNamespace(namespace) {
			errorList.Errorf("%s in %q should be deployed in namespace %q", objectKind, RootRBACForUsPath, m.GetNamespace())
		}
	}
}

// handleRootRBACToUs applies to templates/rbac-to-us.yaml file's objects
func handleRootRBACToUs(m *module.Module, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	prefix := "access-to-" + m.GetName()
	objectName := object.Unstructured.GetName()
	objectKind := object.Unstructured.GetKind()

	if !strings.HasPrefix(objectName, prefix) {
		errorList.Errorf("%s in %q should start with %q", objectKind, RootRBACToUsPath, prefix)
		return
	}

	namespace := object.Unstructured.GetNamespace()
	if !isDeckhouseSystemNamespace(namespace) && namespace != m.GetNamespace() {
		errorList.Errorf("%s in %q should be deployed in namespace \"d8-system\", \"d8-monitoring\" or %q", objectKind, RootRBACToUsPath, m.GetNamespace())
	}
}

// handleNestedRBACForUs applies to templates/**/rbac-for-us.yaml file's objects
func handleNestedRBACForUs(m *module.Module, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	objectName := object.Unstructured.GetName()
	objectKind := object.Unstructured.GetKind()
	shortPath := object.ShortPath()
	namespace := object.Unstructured.GetNamespace()

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
			errorList.Errorf("%s with prefix %q should be deployed in namespace %q", objectKind, localPrefix, m.GetNamespace())
		}

	case strings.HasPrefix(objectName, globalPrefix):
		if !isDeckhouseSystemNamespace(namespace) {
			errorList.Errorf("%s with prefix %q should be deployed in namespace \"d8-system\" or \"d8-monitoring\"", objectKind, globalPrefix)
		}

	case strings.HasPrefix(objectName, systemPrefix):
		if !isSystemNamespace(namespace) {
			errorList.Errorf("%s with prefix %q should be deployed in namespace \"default\" or \"kube-system\"", objectKind, systemPrefix)
		}

	default:
		errorList.Errorf("%s in %q should start with %q or %q", objectKind, shortPath, localPrefix, globalPrefix)
	}
}

// handleNestedRBACToUs applies to templates/**/rbac-to-us.yaml file's objects
func handleNestedRBACToUs(m *module.Module, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	objectName := object.Unstructured.GetName()
	objectKind := object.Unstructured.GetKind()
	shortPath := object.ShortPath()
	namespace := object.Unstructured.GetNamespace()
	parts := strings.Split(
		strings.TrimPrefix(strings.TrimSuffix(shortPath, "/rbac-to-us.yaml"), "templates/"),
		string(os.PathSeparator),
	)

	localPrefix := fmt.Sprintf("access-to-%s-", strings.Join(parts, "-"))
	globalPrefix := fmt.Sprintf("access-to-%s-%s-", m.GetName(), strings.Join(parts, "-"))

	switch {
	case strings.HasPrefix(objectName, localPrefix):
		if namespace != m.GetNamespace() {
			errorList.Errorf("%s with prefix %q should be deployed in namespace %q", objectKind, globalPrefix, m.GetNamespace())
		}

	case strings.HasPrefix(objectName, globalPrefix):
		if !isDeckhouseSystemNamespace(namespace) {
			errorList.Errorf("%s with prefix %q should be deployed in namespace \"d8-system\" or \"d8-monitoring\"", objectKind, globalPrefix)
		}

	default:
		errorList.Errorf("%s should start with %q or %q", objectKind, localPrefix, globalPrefix)
	}
}
