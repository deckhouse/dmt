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

	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/internal/storage"
	"github.com/deckhouse/d8-lint/pkg/errors"
	"github.com/deckhouse/d8-lint/pkg/linters/rbac"
)

/*
ObjectUserAuthzClusterRolePath validates that files for user-authz contains only cluster roles.
Also, it validates that role names equals to d8:user-authz:<ChartName>:<AccessLevel>
*/
func ObjectUserAuthzClusterRolePath(m *module.Module, object storage.StoreObject) *errors.LintRuleError {
	objectKind := object.Unstructured.GetKind()

	shortPath := object.ShortPath()
	if shortPath == UserAuthzClusterRolePath {
		if objectKind != "ClusterRole" {
			return errors.NewLintRuleError(
				rbac.ID,
				object.Identity(),
				m.GetName(),
				nil,
				`Only ClusterRoles can be specified in "templates/user-authz-cluster-roles.yaml"`,
			)
		}

		objectName := object.Unstructured.GetName()
		accessLevel, ok := object.Unstructured.GetAnnotations()["user-authz.deckhouse.io/access-level"]
		if !ok {
			return errors.NewLintRuleError(
				rbac.ID,
				object.Identity(),
				m.GetName(),
				nil,
				`User-authz access ClusterRoles should have annotation "user-authz.deckhouse.io/access-level"`,
			)
		}

		expectedName := fmt.Sprintf("d8:user-authz:%s:%s", m.GetName(), strcase.ToKebab(accessLevel))
		if objectName != expectedName {
			return errors.NewLintRuleError(
				rbac.ID,
				object.Identity(),
				m.GetName(),
				nil,
				"Name of user-authz ClusterRoles should be %q", expectedName,
			)
		}
	}
	return errors.EmptyRuleError
}
