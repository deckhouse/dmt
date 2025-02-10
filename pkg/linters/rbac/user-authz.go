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

	"github.com/iancoleman/strcase"

	"github.com/deckhouse/dmt/internal/module"
)

/*
objectUserAuthzClusterRolePath validates that files for user-authz contains only cluster roles.
Also, it validates that role names equals to d8:user-authz:<ChartName>:<AccessLevel>
*/
func (l *Rbac) objectUserAuthzClusterRolePath(m *module.Module) {
	for _, object := range m.GetObjectStore().Storage {
		errorList := l.ErrorList.WithModule(m.GetName()).WithObjectID(object.Identity())
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
