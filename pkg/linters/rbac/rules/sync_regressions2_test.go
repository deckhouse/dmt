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
	"os"
	"strings"
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/bootstrap"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/generate"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbacyaml"
)

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

// A ServiceAccount subject without a namespace is in the RoleBinding's, as Kubernetes has it
// (regression hunt 2, A5).
func TestSyncRegression_SubjectWithoutNamespace(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	errorList := runSync(t, modulePath, renderedFrom(t, model, func(o *generate.Object) bool {
		if o.Kind == "RoleBinding" && o.Name == "cainjector" {
			subjects := make([]generate.Subject, 0, len(o.Subjects))
			for _, s := range o.Subjects {
				s.Namespace = ""
				subjects = append(subjects, s)
			}

			o.Subjects = subjects
		}

		return true
	}))

	assert.NotContains(t, strings.Join(texts(errorList), "\n"), "RoleBinding/cainjector: subject")
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

// A declaration bootstrap produced that does not parse is a bug of dmt, yet it is written: the
// error names the line, the developer fixes it and goes on. The next lint reads the file, it does
// not bootstrap again.
func TestWriteBootstrapped_UnparsableIsWrittenWithTheLine(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	path := rbacyaml.Path(modulePath)
	require.NoError(t, os.Remove(path))

	broken := []byte("# Written by dmt\n# - a note over\n  two lines that lost its #\napiVersion: rbac.deckhouse.io/v1alpha1\n")

	err := writeBootstrapped(path, broken, nil, bootstrap.Input{Module: syncModule, Namespace: "d8-cert-manager"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rbac.yaml is written, but it does not parse")
	assert.Contains(t, err.Error(), "line 3")
	assert.Contains(t, err.Error(), "this is a bug of dmt")

	written, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, broken, written)

	got := strings.Join(texts(runSync(t, modulePath, renderedFrom(t, syncModelFromFixture(t), nil))), "\n")
	assert.Contains(t, got, "nothing is compared or generated until the declaration parses")
	assert.NotContains(t, got, "rbac.yaml is missing")
}

// syncModelFromFixture builds the model of the cert-manager fixture without reading the module's
// rbac.yaml, which a test may have broken.
func syncModelFromFixture(t *testing.T) *generate.Model {
	t.Helper()

	return syncModel(t, syncModuleDir(t))
}

// What the generator writes for the shapes this version supports passes the placement rule: an
// account in a component directory, access with clusterRules in one, namespace access at the root
// (review of #479, finding 37).
func TestSyncRegression_GeneratedNamesPassPlacement(t *testing.T) {
	get := []rbacyaml.PolicyRule{{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}}}
	nodes := []rbacyaml.PolicyRule{{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"get"}}}
	group := []rbacyaml.Subject{{Kind: "Group", Name: "g"}}

	decl := &rbacyaml.Declaration{APIVersion: rbacyaml.APIVersionV1Alpha1,
		ServiceAccounts: []rbacyaml.ServiceAccount{
			{Name: "webhook", Path: "webhook", ClusterRules: nodes, NamespaceRules: get, BindClusterRoles: []string{"d8:rbac-proxy"},
				BindRoles: []rbacyaml.RoleRef{{Namespace: "kube-system", Name: "extension-apiserver-authentication-reader"}}},
			{Name: syncModule, ClusterRules: nodes, NamespaceRules: get},
		},
		Access: []rbacyaml.Access{
			{Name: "reader", Subjects: group, NamespaceRules: get},
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
