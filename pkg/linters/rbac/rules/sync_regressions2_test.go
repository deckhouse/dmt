/*
Copyright 2026 Flant JSC

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
	"strings"
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/generate"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbacyaml"
)

// What the generator writes for accounts and access entries in nested directories passes the
// placement rule (regression hunt 2, A1).
func TestSyncRegression_GeneratedNamesPassPlacement(t *testing.T) {
	get := []rbacyaml.PolicyRule{{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}}}
	nodes := []rbacyaml.PolicyRule{{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"get"}}}
	group := []rbacyaml.Subject{{Kind: "Group", Name: "g"}}

	decl := &rbacyaml.Declaration{APIVersion: rbacyaml.APIVersionV1Alpha1,
		ServiceAccounts: []rbacyaml.ServiceAccount{
			{Name: "webhook-tls", Path: "webhook/tls", ClusterRules: nodes, NamespaceRules: get, BindClusterRoles: []string{"d8:rbac-proxy"},
				BindRoles: []rbacyaml.RoleRef{{Namespace: "kube-system", Name: "extension-apiserver-authentication-reader"}}},
			{Name: "cainjector", Path: "cainjector", NamespaceRules: get},
			{Name: syncModule, ClusterRules: nodes, NamespaceRules: get},
		},
		Access: []rbacyaml.Access{
			{Name: "reader", Path: "webhook/tls", Subjects: group, NamespaceRules: get},
			{Name: "nodes", Path: "webhook", Subjects: group, ClusterRules: nodes},
		},
	}
	require.Empty(t, rbacyaml.Validate(decl, nil))

	model, err := generate.Build(generate.Input{Module: syncModule, Namespace: "d8-cert-manager", Subsystems: []string{"security"}, Decl: decl})
	require.NoError(t, err)

	store := renderedFrom(t, model, nil)

	m := mocks.NewModuleMock(minimock.NewController(t))
	m.GetNameMock.Return(syncModule)
	m.GetNamespaceMock.Optional().Return("d8-cert-manager")
	m.GetStorageMock.Return(store.Storage)

	errorList := errors.NewLintRuleErrorsList()
	NewPlacementRule(nil, m, errorList).Check(context.Background())

	assert.Empty(t, texts(errorList))
}

// A hand-written role of another account with the same rules as a declared one is not a
// replaced copy: it is granted to other subjects (regression hunt 2, A3).
func TestSyncRegression_EqualRulesOfAnotherAccountAreNoCopy(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	store := renderedFrom(t, model, nil)

	var role generate.Object

	for _, o := range model.File("templates/cainjector/rbac-for-us.yaml").Objects {
		if o.Kind == "Role" {
			role = o
		}
	}

	require.NotEmpty(t, role.Name)

	other := role
	other.Name = "other"
	putObject(t, store, "templates/other/rbac-for-us.yaml", other)
	putObject(t, store, "templates/other/rbac-for-us.yaml", generate.Object{
		Kind: "RoleBinding", Name: "other", Namespace: role.Namespace, RoleRefKind: "Role", RoleRefName: "other",
		Subjects: []generate.Subject{{Kind: "ServiceAccount", Name: "other", Namespace: role.Namespace}},
	})

	got := strings.Join(texts(runSync(t, modulePath, store)), "\n")
	assert.NotContains(t, got, "the declaration replaced it")
	assert.NotContains(t, got, "the old copy of")
}

// Labels of a declared object are compared, the module labels aside; a ServiceAccount subject
// without a namespace is in the RoleBinding's (regression hunt 2, A4 and A5).
func TestSyncRegression_LabelsAndSubjectNamespace(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	errorList := runSync(t, modulePath, renderedFrom(t, model, func(o *generate.Object) bool {
		switch {
		case o.Kind == "ClusterRole" && o.Name == "d8:cert-manager:cainjector":
			o.Labels = map[string]string{"app": "cainjector", "rbac.authorization.k8s.io/aggregate-to-admin": "true"}
		case o.Kind == "RoleBinding" && o.Name == "cainjector":
			subjects := make([]generate.Subject, 0, len(o.Subjects))
			for _, s := range o.Subjects {
				s.Namespace = ""
				subjects = append(subjects, s)
			}

			o.Subjects = subjects
		}

		return true
	}))

	got := strings.Join(texts(errorList), "\n")
	assert.Contains(t, got, "ClusterRole/d8:cert-manager:cainjector: label rbac.authorization.k8s.io/aggregate-to-admin is in the render but not declared")
	assert.NotContains(t, got, "label heritage")
	assert.NotContains(t, got, "label module")
	assert.NotContains(t, got, "RoleBinding/cainjector: subject")
}

func TestSplitChanges(t *testing.T) {
	added, removed := splitChanges([]string{
		"X: (, pods, , get) is declared but absent from the render",
		"X: (, pods, , list) is in the render but not declared",
		"X binds Role a, the declaration binds Role b",
	})
	assert.Equal(t, []string{"X: (, pods, , get) is declared but absent from the render"}, added)
	assert.Len(t, removed, 2)
}
