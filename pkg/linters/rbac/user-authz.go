package rbac

import (
	"fmt"

	"github.com/iancoleman/strcase"

	"github.com/deckhouse/dmt/internal/module"
)

const (
	objectUserAuthzClusterRolePathRuleName = "object-user-authz-cluster-role-path"
)

/*
objectUserAuthzClusterRolePath validates that files for user-authz contains only cluster roles.
Also, it validates that role names equals to d8:user-authz:<ChartName>:<AccessLevel>
*/
func (l *Rbac) objectUserAuthzClusterRolePath(m *module.Module) {
	errorList := l.ErrorList.WithModule(m.GetName()).WithRule(objectUserAuthzClusterRolePathRuleName)
	for _, object := range m.GetObjectStore().Storage {
		errorList = errorList.WithObjectID(object.Identity()).WithFilePath(object.ShortPath())
		objectKind := object.Unstructured.GetKind()
		shortPath := object.ShortPath()

		if shortPath == UserAuthzClusterRolePath {
			if objectKind != "ClusterRole" {
				errorList.Error(`Only ClusterRoles can be specified in "templates/user-authz-cluster-roles.yaml"`)
				return
			}

			objectName := object.Unstructured.GetName()
			accessLevel, ok := object.Unstructured.GetAnnotations()["user-authz.deckhouse.io/access-level"]
			if !ok {
				errorList.Error(`User-authz access ClusterRoles should have annotation "user-authz.deckhouse.io/access-level"`)
				return
			}

			expectedName := fmt.Sprintf("d8:user-authz:%s:%s", m.GetName(), strcase.ToKebab(accessLevel))
			if objectName != expectedName {
				errorList.Errorf("Name of user-authz ClusterRoles should be %q", expectedName)
				return
			}
		}
	}
}
