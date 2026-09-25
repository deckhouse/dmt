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
	"path/filepath"
	"strings"
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/bootstrap"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/generate"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbaccontract"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbacyaml"
)

// What the generator writes for accounts and access entries in nested directories passes the
// placement rule (regression hunt 2, A1).
func TestSyncRegression_NestedNamesPassPlacement(t *testing.T) {
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

// A subchart's objects stay hand-written: its templates read its own values (regression hunt 2,
// B6).
func TestLocateInTemplate_Subchart(t *testing.T) {
	o := bootstrap.Object{Kind: "ServiceAccount", Name: "subsa", Path: "charts/sub/templates/rbac-for-us.yaml"}
	locateInTemplate(t.TempDir(), &o, map[string][]bootstrap.Doc{})

	assert.True(t, o.Located)
	assert.Equal(t, "rendered by the subchart sub, whose templates and values are its own", o.Unmanageable)
}

// Objects of includes under different conditions stay the library's: they are not imported as
// the module's own and their file is not generated. A legacy role sync owns by class is imported
// with a TODO reason, so the run stays red (review of #480).
func TestLocateInTemplate_LibraryIncludesUnderDifferentConditions(t *testing.T) {
	modulePath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(modulePath, "templates"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(modulePath, "templates", "rbac-for-us.yaml"), []byte(`{{ include "helm_lib_csi_controller_rbac" . }}
---
{{- if .Values.m.on }}
{{ include "helm_lib_other_rbac" . }}
{{- end }}
`), 0o600))

	labels := map[string]string{"module": "m"}
	objects := []bootstrap.Object{
		{Kind: "ServiceAccount", Name: "csi", Namespace: "d8-m", Path: "templates/rbac-for-us.yaml", Labels: labels},
		{Kind: "ClusterRole", Name: "d8:m:user", Path: "templates/rbac-for-us.yaml", Labels: labels,
			Annotations: map[string]string{rbaccontract.AccessLevelAnnotation: "User"},
			Rules:       []rbacv1.PolicyRule{{APIGroups: []string{"x.io"}, Resources: []string{"things"}, Verbs: []string{"get"}}}},
	}

	cache := map[string][]bootstrap.Doc{}
	for i := range objects {
		locateInTemplate(modulePath, &objects[i], cache)

		assert.True(t, objects[i].Library, "%s is rendered by a library", objects[i].Name)
		assert.Contains(t, objects[i].Unmanageable, "under different conditions (no condition; `.Values.m.on`)")
	}

	markLibraryFiles(objects)
	assert.True(t, objects[0].LibraryFile, "the file is not generatable")

	got := bootstrap.Build(bootstrap.Input{Module: "m", Namespace: "d8-m", Objects: objects, CRDs: map[string]string{"x.io/things": "Namespaced"}})

	unmanaged := strings.Join(got.Unmanaged, "\n")
	assert.Contains(t, unmanaged, "ServiceAccount/csi (templates/rbac-for-us.yaml): rendered by includes of named templates under different conditions")
	assert.NotContains(t, strings.Join(got.Notes, "\n"), "was not found in the text of its template")
	assert.Empty(t, got.Decl.ServiceAccounts, "the library's account is not the module's")

	require.Len(t, got.Decl.Resources, 1, "the legacy role is sync's whatever renders it")
	assert.Contains(t, got.Decl.Resources[0].Reason, "TODO: ClusterRole d8:m:user renders under `TODO: rendered by includes under different conditions")
}

// A declaration bootstrap produced that does not parse would be a bug of dmt; the fix writes it and
// succeeds, and the next lint reports it with the line instead of bootstrapping again.
func TestWriteBootstrapped_UnparsableIsALintFinding(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	path := rbacyaml.Path(modulePath)
	require.NoError(t, os.Remove(path))

	broken := []byte("# The module RBAC declaration\n# - a note over\n  two lines that lost its #\napiVersion: rbac.deckhouse.io/v1alpha1\n")

	require.NoError(t, writeBootstrapped(path, broken))

	written, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, broken, written)

	got := strings.Join(texts(runSync(t, modulePath, renderedFrom(t, syncModelFromFixture(t), nil))), "\n")
	assert.Contains(t, got, "line 3")
	assert.Contains(t, got, "until the declaration parses")
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
// (review of #479, finding 37). The placement rule skips templates/rbac-for-us.yaml and
// templates/rbac-to-us.yaml today (RBACv2Path is a prefix of both, finding 45), so the root shapes
// are not proven here until that is fixed upstream.
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

// Under --matrix an object only some variants rendered is named in the written declaration: its
// condition is not in it (review of #479, finding 40).
func TestSyncRegression_BootstrapNotesObjectsOfSomeVariants(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	require.NoError(t, os.Remove(rbacyaml.Path(modulePath)))

	withoutInjector := renderedFrom(t, model, func(o *generate.Object) bool { return o.When == "" })
	withInjector := renderedFrom(t, model, nil)

	lists := []*errors.LintRuleErrorsList{runSync(t, modulePath, withoutInjector), runSync(t, modulePath, withInjector)}
	for _, list := range lists {
		for _, fix := range list.GetFixes() {
			fix()
		}
	}

	content, err := os.ReadFile(rbacyaml.Path(modulePath))
	require.NoError(t, err)
	decl, err := rbacyaml.Parse(content)
	require.NoError(t, err)

	whens := map[string]string{}
	for _, sa := range decl.ServiceAccounts {
		whens[sa.Name] = sa.When
	}

	// The account and its objects render only with the injector on: a TODO for their condition
	// keeps the run red (review of #479, finding 41).
	assert.True(t, strings.HasPrefix(whens["cainjector"], "TODO: "), whens["cainjector"])
	assert.Contains(t, whens["cainjector"], "ServiceAccount cainjector")
	assert.Empty(t, whens["cert-manager"])
}
