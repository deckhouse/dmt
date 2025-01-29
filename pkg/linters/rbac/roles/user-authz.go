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

	"github.com/iancoleman/strcase"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

/*
ObjectUserAuthzClusterRolePath validates that files for user-authz contains only cluster roles.
Also, it validates that role names equals to d8:user-authz:<ChartName>:<AccessLevel>
*/
func ObjectUserAuthzClusterRolePath(m *module.Module, object storage.StoreObject) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, m.GetName())
	objectKind := object.Unstructured.GetKind()

	shortPath := object.ShortPath()
	if shortPath == UserAuthzClusterRolePath {
		if objectKind != "ClusterRole" {
			return result.WithObjectID(object.Identity()).Add(`Only ClusterRoles can be specified in "templates/user-authz-cluster-roles.yaml"`)
		}

		objectName := object.Unstructured.GetName()
		accessLevel, ok := object.Unstructured.GetAnnotations()["user-authz.deckhouse.io/access-level"]
		if !ok {
			return result.WithObjectID(object.Identity()).Add(`User-authz access ClusterRoles should have annotation "user-authz.deckhouse.io/access-level"`)
		}

		expectedName := fmt.Sprintf("d8:user-authz:%s:%s", m.GetName(), strcase.ToKebab(accessLevel))
		if objectName != expectedName {
			return result.WithObjectID(object.Identity()).Add("Name of user-authz ClusterRoles should be %q", expectedName)
		}
	}
	return nil
}
