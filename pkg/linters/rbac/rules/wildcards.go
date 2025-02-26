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
	"slices"
	"strings"

	k8SRbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	WildcardsRuleName = "wildcards"
)

func NewWildcardsRule(excludeRules []pkg.KindRuleExclude) *WildcardsRule {
	return &WildcardsRule{
		RuleMeta: pkg.RuleMeta{
			Name: WildcardsRuleName,
		},
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
	}
}

type WildcardsRule struct {
	pkg.RuleMeta
	pkg.KindRule
}

// objectRolesWildcard is a linter for checking the presence
// of a wildcard in a Role and ClusterRole
func (r *WildcardsRule) ObjectRolesWildcard(m *module.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.Name)
	for _, object := range m.GetStorage() {
		// check only `rbac-for-us.yaml` files
		if !strings.HasSuffix(object.ShortPath(), "rbac-for-us.yaml") {
			continue
		}

		if !r.Enabled(object.Unstructured.GetKind(), object.Unstructured.GetName()) {
			continue
		}

		errorListObj := errorList.WithObjectID(object.Identity()).WithFilePath(object.ShortPath())

		// check Role and ClusterRole for wildcards
		objectKind := object.Unstructured.GetKind()
		switch objectKind {
		case "Role", "ClusterRole":
			r.checkRoles(object, errorListObj)
		}
	}
}

func (*WildcardsRule) checkRoles(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	converter := runtime.DefaultUnstructuredConverter

	role := new(k8SRbac.Role)

	if err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), role); err != nil {
		errorList.Errorf("Cannot convert object to %s: %v", object.Unstructured.GetKind(), err)

		return
	}

	for _, rule := range role.Rules {
		var objs []string
		if slices.Contains(rule.APIGroups, "*") {
			objs = append(objs, "apiGroups")
		}

		if slices.Contains(rule.Resources, "*") {
			objs = append(objs, "resources")
		}

		if slices.Contains(rule.Verbs, "*") {
			objs = append(objs, "verbs")
		}

		if len(objs) > 0 {
			errorList.Errorf("%s contains a wildcards. Replace them with an explicit list of resources", strings.Join(objs, ", "))

			return
		}
	}
}
