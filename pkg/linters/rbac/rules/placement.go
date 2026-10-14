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
	"fmt"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbaccontract"
)

const (
	PlacementRuleName = "placement"
)

func NewPlacementRule(excludeRules []pkg.KindRuleExclude,
	m pkg.Module, errorList *errors.LintRuleErrorsList) *PlacementRule {
	return &PlacementRule{
		RuleMeta: pkg.RuleMeta{
			Name: PlacementRuleName,
		},
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
		module:    m,
		errorList: errorList.WithRule(PlacementRuleName),
	}
}

type PlacementRule struct {
	pkg.RuleMeta
	pkg.KindRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*PlacementRule)(nil)

const (
	serviceAccountNameDelimiter = "-"
	UserAuthzClusterRolePath    = "templates/user-authz-cluster-roles.yaml"
	RootRBACForUsPath           = "templates/rbac-for-us.yaml"
	RootRBACToUsPath            = "templates/rbac-to-us.yaml"
	RBACv2Path                  = "templates/rbac"
)

func isSystemNamespace(actual string) bool {
	return actual == metav1.NamespaceDefault || actual == metav1.NamespaceSystem
}

func isDeckhouseSystemNamespace(actual string) bool {
	return rbaccontract.IsDeckhouseNamespace(actual)
}

func (r *PlacementRule) Check(_ context.Context) {
	m, errorList := r.module, r.errorList

	for _, object := range m.GetStorage() {
		errorListObj := errorList.WithObjectID(object.Identity())

		if !r.Enabled(object.Unstructured.GetKind(), object.Unstructured.GetName()) {
			// TODO: add metrics
			continue
		}

		shortPath := object.ShortPath()
		if shortPath == UserAuthzClusterRolePath || strings.HasPrefix(shortPath, RBACv2Path) {
			continue
		}

		objectKind := object.Unstructured.GetKind()
		switch objectKind {
		case "ServiceAccount":
			objectRBACPlacementServiceAccount(m, object, errorListObj)
		case "ClusterRole", "ClusterRoleBinding":
			objectRBACPlacementClusterRole(m, object, errorListObj)
		case "Role", "RoleBinding":
			objectRBACPlacementRole(m, object, errorListObj)
		default:
			if strings.HasSuffix(shortPath, "rbac-for-us.yaml") || strings.HasSuffix(shortPath, "rbac-to-us.yaml") {
				errorListObj.WithFilePath(shortPath).
					Errorf("kind %s not allowed", objectKind)
			}
		}
	}
}

func objectRBACPlacementServiceAccount(m pkg.Module, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
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

		switch objectName {
		case serviceAccountName:
			if m.GetNamespace() != namespace {
				if isDeckhouseSystemNamespace(namespace) {
					errorList.Errorf("Service account namespace should be equal to %q namespace. If the namespace is correct, change name to %q", m.GetNamespace(), expectedServiceAccountName)
					return
				}

				errorList.Errorf("Service account namespace should be equal to %q namespace", m.GetNamespace())
			}

			return
		case expectedServiceAccountName:
			if !isDeckhouseSystemNamespace(namespace) {
				if m.GetNamespace() == namespace {
					errorList.Errorf("Service account namespace should be equal to deckhouse namespaces like \"d8-system\" or \"d8-monitoring\". If this namespaces is correct, change name to %q", serviceAccountName)
					return
				}

				errorList.Errorf("Service account namespace should be equal to deckhouse namespaces like \"d8-system\" or \"d8-monitoring\"")
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

func objectRBACPlacementClusterRole(m pkg.Module, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
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
		errorList.Errorf("%s should be in %q or %q or \"*/rbac-for-us.yaml\"", objectKind, UserAuthzClusterRolePath, RootRBACForUsPath)
	}
}

func objectRBACPlacementRole(m pkg.Module, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	shortPath := object.ShortPath()
	errorList = errorList.WithFilePath(shortPath)

	if finding := rolePlacementFinding(m.GetName(), m.GetNamespace(), shortPath, object.Unstructured.GetKind(), object.Unstructured.GetName(), object.Unstructured.GetNamespace()); finding != "" {
		errorList.Errorf("%s", finding)
	}
}

// rolePlacementFinding is what the placement rule says about a Role or a RoleBinding of the module
// named name in namespace that renders from shortPath, or "" when it accepts the object. The sync rule
// asks it too, for a name the module already has, before it keeps the name in a first declaration.
func rolePlacementFinding(module, moduleNamespace, shortPath, kind, name, namespace string) string {
	switch {
	case shortPath == RootRBACForUsPath:
		return rootRBACForUsFinding(module, moduleNamespace, kind, name, namespace)

	case shortPath == RootRBACToUsPath:
		return rootRBACToUsFinding(module, moduleNamespace, kind, name, namespace)

	case strings.HasSuffix(shortPath, "rbac-for-us.yaml"):
		return nestedRBACForUsFinding(module, moduleNamespace, shortPath, kind, name, namespace)

	case strings.HasSuffix(shortPath, "rbac-to-us.yaml"):
		return nestedRBACToUsFinding(module, moduleNamespace, shortPath, kind, name, namespace)

	default:
		msgTemplate := `%s should be in "templates/rbac-for-us.yaml", "templates/rbac-to-us.yaml", ".*/rbac-to-us.yaml" or ".*/rbac-for-us.yaml"`
		return fmt.Sprintf(msgTemplate, kind)
	}
}

// rootRBACForUsFinding applies to templates/rbac-for-us.yaml file's objects
func rootRBACForUsFinding(module, moduleNamespace, objectKind, objectName, namespace string) string {
	prefix := "d8:" + module

	switch {
	case objectName == module && namespace != moduleNamespace:
		if !isDeckhouseSystemNamespace(namespace) {
			return fmt.Sprintf("%s in %q should be deployed in namespace \"d8-monitoring\", \"d8-system\" or %q", objectKind, RootRBACForUsPath, moduleNamespace)
		}

	case strings.HasPrefix(objectName, prefix):
		if !isSystemNamespace(namespace) {
			return fmt.Sprintf("%s in %q should be deployed in namespace \"default\" or \"kube-system\"", objectKind, RootRBACForUsPath)
		}

	case !strings.HasPrefix(objectName, prefix):
		if !isDeckhouseSystemNamespace(namespace) {
			return fmt.Sprintf("%s in %q should be deployed in namespace %q", objectKind, RootRBACForUsPath, moduleNamespace)
		}
	}

	return ""
}

// rootRBACToUsFinding applies to templates/rbac-to-us.yaml file's objects
func rootRBACToUsFinding(module, moduleNamespace, objectKind, objectName, namespace string) string {
	prefix := "access-to-" + module

	if !strings.HasPrefix(objectName, prefix) {
		return fmt.Sprintf("%s in %q should start with %q", objectKind, RootRBACToUsPath, prefix)
	}

	if !isDeckhouseSystemNamespace(namespace) && namespace != moduleNamespace {
		return fmt.Sprintf("%s in %q should be deployed in namespace \"d8-system\", \"d8-monitoring\" or %q", objectKind, RootRBACToUsPath, moduleNamespace)
	}

	return ""
}

// nestedRBACForUsFinding applies to templates/**/rbac-for-us.yaml file's objects
func nestedRBACForUsFinding(module, moduleNamespace, shortPath, objectKind, objectName, namespace string) string {
	parts := strings.Split(
		strings.TrimPrefix(strings.TrimSuffix(shortPath, "/rbac-for-us.yaml"), "templates/"),
		string(os.PathSeparator),
	)

	localPrefix := strings.Join(parts, ":")
	globalPrefix := fmt.Sprintf("%s:%s", module, strings.Join(parts, ":"))
	systemPrefix := fmt.Sprintf("d8:%s", globalPrefix)

	switch {
	case strings.HasPrefix(objectName, localPrefix):
		if namespace != moduleNamespace {
			return fmt.Sprintf("%s with prefix %q should be deployed in namespace %q", objectKind, localPrefix, moduleNamespace)
		}

	case strings.HasPrefix(objectName, globalPrefix):
		if !isDeckhouseSystemNamespace(namespace) {
			return fmt.Sprintf("%s with prefix %q should be deployed in namespace \"d8-system\" or \"d8-monitoring\"", objectKind, globalPrefix)
		}

	case strings.HasPrefix(objectName, systemPrefix):
		if !isSystemNamespace(namespace) {
			return fmt.Sprintf("%s with prefix %q should be deployed in namespace \"default\" or \"kube-system\"", objectKind, systemPrefix)
		}

	default:
		return fmt.Sprintf("%s in %q should start with %q or %q", objectKind, shortPath, localPrefix, globalPrefix)
	}

	return ""
}

// nestedRBACToUsFinding applies to templates/**/rbac-to-us.yaml file's objects
func nestedRBACToUsFinding(module, moduleNamespace, shortPath, objectKind, objectName, namespace string) string {
	parts := strings.Split(
		strings.TrimPrefix(strings.TrimSuffix(shortPath, "/rbac-to-us.yaml"), "templates/"),
		string(os.PathSeparator),
	)

	localPrefix := fmt.Sprintf("access-to-%s-", strings.Join(parts, "-"))
	globalPrefix := fmt.Sprintf("access-to-%s-%s-", module, strings.Join(parts, "-"))

	switch {
	case strings.HasPrefix(objectName, localPrefix):
		if namespace != moduleNamespace {
			return fmt.Sprintf("%s with prefix %q should be deployed in namespace %q", objectKind, globalPrefix, moduleNamespace)
		}

	case strings.HasPrefix(objectName, globalPrefix):
		if !isDeckhouseSystemNamespace(namespace) {
			return fmt.Sprintf("%s with prefix %q should be deployed in namespace \"d8-system\" or \"d8-monitoring\"", objectKind, globalPrefix)
		}

	default:
		return fmt.Sprintf("%s should start with %q or %q", objectKind, localPrefix, globalPrefix)
	}

	return ""
}
