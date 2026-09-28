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
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/pkg"
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

// A label of the module on a capability -- one a role of the module outside the role model selects
// -- is compared like the labels of the role model: the rewrite writes the declared ones only, and a
// dropped selector label is a lost right.
func TestSync_ModuleLabelsOfACapability(t *testing.T) {
	const (
		view  = "d8:namespace-capability:cert-manager:view"
		agent = "cert-manager.deckhouse.io/aggregate-to-agent"
	)

	t.Run("undeclared", func(t *testing.T) {
		resetFixState()
		t.Cleanup(resetFixState)

		modulePath := syncModuleDir(t)
		model := syncModel(t, modulePath)
		writeGenerated(t, modulePath, model)

		got := strings.Join(texts(runSync(t, modulePath, renderedFrom(t, model, func(o *generate.Object) bool {
			if o.Name == view {
				o.Labels[agent] = "true"
			}

			return true
		}))), "\n")
		assert.Contains(t, got, "ClusterRole/"+view+": label "+agent+" is in the render but not declared")
	})

	t.Run("declared", func(t *testing.T) {
		resetFixState()
		t.Cleanup(resetFixState)

		modulePath := syncModuleDir(t)

		decl, err := rbacyaml.Load(modulePath)
		require.NoError(t, err)

		decl.Capabilities["namespace.view"] = rbacyaml.Capability{Labels: map[string]string{agent: "true"}}

		raw, err := yaml.Marshal(decl)
		require.NoError(t, err)
		require.NotContains(t, string(raw), "title: {}", "a labels-only entry is written without texts")
		require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), raw, 0o600))

		model := syncModel(t, modulePath)
		writeGenerated(t, modulePath, model)

		assert.Empty(t, texts(runSync(t, modulePath, renderedFrom(t, model, nil))))

		got := strings.Join(texts(runSync(t, modulePath, renderedFrom(t, model, func(o *generate.Object) bool {
			if o.Name == view {
				delete(o.Labels, agent)
			}

			return true
		}))), "\n")
		assert.Contains(t, got, "ClusterRole/"+view+": label "+agent+" is declared but absent from the render")

		got = strings.Join(texts(runSync(t, modulePath, renderedFrom(t, model, func(o *generate.Object) bool {
			if o.Name == view {
				o.Labels = maps.Clone(o.Labels)
				o.Labels[agent] = "false"
			}

			return true
		}))), "\n")
		assert.Contains(t, got, "ClusterRole/"+view+": label "+agent+` is "false" in the render, the declaration produces "true"`)
	})
}

// dropRule makes the named role diverge: the render lacks its last rule, so its file gets a finding
// and, unless a case keeps it off, the fix.
func dropRule(name string) func(o *generate.Object) bool {
	return func(o *generate.Object) bool {
		if o.Name == name && len(o.Rules) > 0 {
			o.Rules = o.Rules[:len(o.Rules)-1]
		}

		return true
	}
}

// What a rewrite would drop or widen beyond what sync compares keeps the fix off the file, and the
// finding names it (review of #480).
func TestSync_RewriteKeepsWhatItCannotCompare(t *testing.T) {
	const (
		view   = "d8:namespace-capability:cert-manager:view"
		editor = "d8:user-authz:cert-manager:editor"
		user   = "d8:user-authz:cert-manager:user"
	)

	t.Run("a condition the declaration does not write", func(t *testing.T) {
		resetFixState()
		t.Cleanup(resetFixState)

		modulePath := syncModuleDir(t)
		model := syncModel(t, modulePath)
		writeGenerated(t, modulePath, model)

		path := filepath.Join(modulePath, "templates/rbacv2/use/view.yaml")
		text, err := os.ReadFile(path)
		require.NoError(t, err)

		hand := "rules:\n{{- if .Values.certManager.secretRead }}\n- apiGroups:\n  - \"\"\n  resources:\n  - secrets\n  verbs:\n  - get\n{{- end }}\n"
		require.NoError(t, os.WriteFile(path, []byte(strings.Replace(string(text), "rules:\n", hand, 1)), 0o600))

		errorList := runSync(t, modulePath, renderedFrom(t, model, dropRule(view)))
		assert.Contains(t, strings.Join(texts(errorList), "\n"), "The autofix leaves the file as it is: the template holds {{ if .Values.certManager.secretRead }}, which rbac.yaml does not write")
		assert.Empty(t, errorList.GetFixes())
	})

	t.Run("an object excluded from sync", func(t *testing.T) {
		resetFixState()
		t.Cleanup(resetFixState)

		modulePath := syncModuleDir(t)
		model := syncModel(t, modulePath)
		writeGenerated(t, modulePath, model)

		errorList := runSync(t, modulePath, renderedFrom(t, model, dropRule(user)), pkg.KindRuleExclude{Kind: "ClusterRole", Name: editor})
		assert.Contains(t, strings.Join(texts(errorList), "\n"), "ClusterRole/"+editor+" is excluded from sync (exclude-rules.sync), and a rewrite would write it from the declaration over what the template holds")
		assert.Empty(t, errorList.GetFixes())
	})

	t.Run("an annotation of a capability the format cannot hold", func(t *testing.T) {
		resetFixState()
		t.Cleanup(resetFixState)

		modulePath := syncModuleDir(t)
		model := syncModel(t, modulePath)
		writeGenerated(t, modulePath, model)

		errorList := runSync(t, modulePath, renderedFrom(t, model, func(o *generate.Object) bool {
			if o.Name == view {
				o.Annotations = maps.Clone(o.Annotations)
				o.Annotations["helm.sh/resource-policy"] = "keep"
			}

			return true
		}))
		got := strings.Join(texts(errorList), "\n")
		assert.Contains(t, got, "ClusterRole/"+view+": annotation helm.sh/resource-policy is in the render, and rbac.yaml cannot declare it")
		assert.Contains(t, got, "The autofix leaves the file as it is: ClusterRole/"+view+" carries annotation helm.sh/resource-policy, which rbac.yaml cannot declare and a rewrite would drop")
		assert.Empty(t, errorList.GetFixes())
	})

	t.Run("a label of a legacy role", func(t *testing.T) {
		resetFixState()
		t.Cleanup(resetFixState)

		modulePath := syncModuleDir(t)
		model := syncModel(t, modulePath)
		writeGenerated(t, modulePath, model)

		errorList := runSync(t, modulePath, renderedFrom(t, model, func(o *generate.Object) bool {
			if o.Name == user {
				o.Labels = map[string]string{"rbac.authorization.k8s.io/aggregate-to-view": "true"}
			}

			return true
		}))
		assert.Contains(t, strings.Join(texts(errorList), "\n"), "ClusterRole/"+user+" carries label rbac.authorization.k8s.io/aggregate-to-view, which rbac.yaml cannot declare and a rewrite would drop")
		assert.Empty(t, errorList.GetFixes())
	})

	t.Run("the secrets of a ServiceAccount", func(t *testing.T) {
		resetFixState()
		t.Cleanup(resetFixState)

		modulePath := syncModuleDir(t)
		model := syncModel(t, modulePath)
		writeGenerated(t, modulePath, model)

		store := renderedFrom(t, model, func(o *generate.Object) bool {
			if o.Name == "d8:cert-manager:cainjector" && o.Kind == "ClusterRole" {
				o.Rules = o.Rules[:len(o.Rules)-1]
			}

			return o.Kind != "ServiceAccount" || o.Name != "cainjector"
		})

		sa := &corev1.ServiceAccount{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ServiceAccount"},
			ObjectMeta:       metav1.ObjectMeta{Name: "cainjector", Namespace: "d8-cert-manager", Labels: map[string]string{"heritage": "deckhouse", "module": syncModule}},
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "registry"}}}
		content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(sa)
		require.NoError(t, err)
		require.NoError(t, store.Put("/module/templates/cainjector/rbac-for-us.yaml", "templates/cainjector/rbac-for-us.yaml", content, []byte("sa")))

		errorList := runSync(t, modulePath, store)
		assert.Contains(t, strings.Join(texts(errorList), "\n"), "ServiceAccount/cainjector has imagePullSecrets, which rbac.yaml cannot declare and a rewrite would drop")
		assert.Empty(t, errorList.GetFixes())
	})

	t.Run("a symbolic link", func(t *testing.T) {
		resetFixState()
		t.Cleanup(resetFixState)

		modulePath := syncModuleDir(t)
		model := syncModel(t, modulePath)
		writeGenerated(t, modulePath, model)

		path := filepath.Join(modulePath, "templates/rbacv2/use/view.yaml")
		require.NoError(t, os.Rename(path, path+".target"))
		require.NoError(t, os.Symlink("view.yaml.target", path))

		errorList := runSync(t, modulePath, renderedFrom(t, model, dropRule(view)))
		assert.Contains(t, strings.Join(texts(errorList), "\n"), "the template is a symbolic link, and a rewrite would replace the link")
		assert.Empty(t, errorList.GetFixes())
	})
}

// A changed localized text of a capability is a divergence the rewrite restores.
func TestSync_CapabilityTextsAreCompared(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	errorList := runSync(t, modulePath, renderedFrom(t, model, func(o *generate.Object) bool {
		if o.Name == "d8:namespace-capability:cert-manager:view" {
			o.Annotations = maps.Clone(o.Annotations)
			o.Annotations[rbaccontract.AnnotationTitleEN] = "Edited by hand"
		}

		return true
	}))
	assert.Contains(t, strings.Join(texts(errorList), "\n"), `annotation en.meta.deckhouse.io/title is "Edited by hand" in the render`)
	assert.NotEmpty(t, errorList.GetFixes())
}

// A grant newly declared under a condition some other grant already renders under is drift: the
// condition holds in this render, whatever file shows it.
func TestSync_ConditionThatHoldsElsewhere(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	before := syncModel(t, modulePath)
	writeGenerated(t, modulePath, before)

	decl, err := rbacyaml.Load(modulePath)
	require.NoError(t, err)

	decl.Resources = append(decl.Resources, rbacyaml.Resource{Group: "cert-manager.io", Resource: "issuers/status", When: ".Values.certManager.internal.acmeEnabled",
		Namespace: map[string][]string{"viewer": {"get"}}})

	raw, err := yaml.Marshal(decl)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), raw, 0o600))

	errorList := runSync(t, modulePath, renderedFrom(t, before, nil))
	assert.Contains(t, strings.Join(texts(errorList), "\n"), "get cert-manager.io/issuers/status is declared but absent from the render")
	assert.NotEmpty(t, errorList.GetFixes())
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

// Every file the declaration produces reads back as the declaration's, whatever `when` validation
// lets through: a brace in a string literal, a field named like an abort of the render (review of
// #480).
func TestSync_GeneratedConditionsReadBack(t *testing.T) {
	for _, when := range []string{`eq .Values.x "}"`, `.Values.x.required`, `and .Values.x.fail (eq .Values.y "a}b")`} {
		t.Run(when, func(t *testing.T) {
			decl := &rbacyaml.Declaration{APIVersion: rbacyaml.APIVersionV1Alpha1,
				Resources: []rbacyaml.Resource{
					{Group: "x.io", Resource: "things", Scope: "Namespaced", When: when, Namespace: map[string][]string{"viewer": {"get"}}},
					{Group: "x.io", Resource: "others", Scope: "Namespaced", Namespace: map[string][]string{"viewer": {"get"}}},
				},
			}
			require.Empty(t, rbacyaml.Validate(decl, nil))

			model, err := generate.Build(generate.Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Decl: decl})
			require.NoError(t, err)

			conditions := map[string]struct{}{}

			for _, file := range model.Files {
				text := generate.RenderFile(file)
				for _, doc := range textDocuments(text) {
					assert.False(t, doc.unreadable, "%s: %s", file.Path, text)
				}

				maps.Copy(conditions, templateConditions(text))
			}

			assert.Equal(t, map[string]struct{}{"if " + when: {}}, conditions, "the condition the declaration writes is read back")
		})
	}
}

// The rewrite keeps the file's permissions, and the bootstrap fix writes nothing over a declaration
// that appeared in the meantime (review of #480).
func TestSync_FixesKeepWhatIsThere(t *testing.T) {
	t.Run("permissions", func(t *testing.T) {
		resetFixState()
		t.Cleanup(resetFixState)

		modulePath := syncModuleDir(t)
		model := syncModel(t, modulePath)
		writeGenerated(t, modulePath, model)

		path := filepath.Join(modulePath, "templates/rbacv2/use/view.yaml")
		require.NoError(t, os.Chmod(path, 0o640))

		list := runSync(t, modulePath, renderedFrom(t, model, dropRule("d8:namespace-capability:cert-manager:view")))
		fixes := list.GetFixes()
		require.Len(t, fixes, 1)
		fixes[0]()

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
	})

	t.Run("a declaration written in the meantime", func(t *testing.T) {
		resetFixState()
		t.Cleanup(resetFixState)

		modulePath := syncModuleDir(t)
		model := syncModel(t, modulePath)
		writeGenerated(t, modulePath, model)
		require.NoError(t, os.Remove(rbacyaml.Path(modulePath)))

		list := runSync(t, modulePath, renderedFrom(t, model, nil))
		fixes := list.GetFixes()
		require.Len(t, fixes, 1, "the bootstrap fix")

		const mine = "apiVersion: rbac.deckhouse.io/v1alpha1\n# written by hand\n"
		require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), []byte(mine), 0o600))

		fixes[0]()

		got, err := os.ReadFile(rbacyaml.Path(modulePath))
		require.NoError(t, err)
		assert.Equal(t, mine, string(got))
	})
}

// Conditions are compared per object, both ways, from the text: the same in every render variant
// (review of #480, AlwxSin 1 and 3).
func TestSync_ConditionsOfEveryObject(t *testing.T) {
	const acme = ".Values.certManager.internal.acmeEnabled"

	t.Run("the declaration's condition missing from the template is written", func(t *testing.T) {
		resetFixState()
		t.Cleanup(resetFixState)

		modulePath := syncModuleDir(t)
		model := syncModel(t, modulePath)
		writeGenerated(t, modulePath, model)

		const rel = "templates/rbacv2/use/view.yaml"

		path := filepath.Join(modulePath, rel)
		text, err := os.ReadFile(path)
		require.NoError(t, err)

		unconditional := strings.Replace(strings.Replace(string(text), "{{- if "+acme+" }}\n", "", 1), "{{- end }}\n", "", 1)
		require.NotEqual(t, string(text), unconditional)
		require.NoError(t, os.WriteFile(path, []byte(unconditional), 0o600))

		errorList := runSync(t, modulePath, renderedFrom(t, model, nil))
		assert.Contains(t, strings.Join(texts(errorList), "\n"), "ClusterRole/d8:namespace-capability:cert-manager:view: the declaration writes {{ if "+acme+" }} around it or its rules, which the template does not hold")

		fixes := errorList.GetFixes()
		require.Len(t, fixes, 1)
		fixes[0]()

		written, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, generate.RenderFile(*model.File(rel)), string(written))
	})

	t.Run("a condition around a whole file that renders nothing is a finding", func(t *testing.T) {
		resetFixState()
		t.Cleanup(resetFixState)

		modulePath := syncModuleDir(t)
		model := syncModel(t, modulePath)
		writeGenerated(t, modulePath, model)

		const rel = "templates/rbac-to-us.yaml"

		path := filepath.Join(modulePath, rel)
		text, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, []byte("{{- if .Values.foo }}\n"+string(text)+"{{- end }}\n"), 0o600))

		// foo is false: nothing renders from the file.
		errorList := runSync(t, modulePath, renderedFrom(t, model, func(o *generate.Object) bool {
			return model.File(rel) == nil || !slices.ContainsFunc(model.File(rel).Objects, func(x generate.Object) bool { return x.Identity() == o.Identity() })
		}))
		got := strings.Join(texts(errorList), "\n")
		assert.Contains(t, got, "the template holds {{ if .Values.foo }} around it or its rules, which the declaration does not write")
		assert.Contains(t, got, "The autofix leaves the file as it is")
		assert.Empty(t, errorList.GetFixes())
	})
}

// An object under an old name is a rename only when its metadata comes along: an aggregation label
// or helm.sh/resource-policy would leave with it (review of #480, AlwxSin 2).
func TestSync_RenameCarriesMetadata(t *testing.T) {
	for name, tweak := range map[string]func(o *generate.Object){
		"aggregation label": func(o *generate.Object) {
			o.Labels = map[string]string{"rbac.authorization.k8s.io/aggregate-to-admin": "true"}
		},
		"resource policy": func(o *generate.Object) {
			o.Annotations = map[string]string{"helm.sh/resource-policy": "keep"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			resetFixState()
			t.Cleanup(resetFixState)

			modulePath := syncModuleDir(t)
			model := syncModel(t, modulePath)
			writeGenerated(t, modulePath, model)

			const old = "access-to-cert-manager-prometheus-metrics"

			store := renderedFrom(t, model, func(o *generate.Object) bool {
				if o.Name == "access-to-cert-manager" && (o.Kind == "Role" || o.Kind == "RoleBinding") {
					o.Name = old

					if o.Kind == "RoleBinding" {
						o.RoleRefName = old
					} else {
						tweak(o)
					}
				}

				return true
			})

			errorList := runSync(t, modulePath, store)
			assert.Contains(t, strings.Join(texts(errorList), "\n"), "Role/"+old+", which rbac.yaml does not produce")
			assert.Empty(t, errorList.GetFixes())
		})
	}
}

// A file that renders legacy objects of both kinds names the same one in every run: map order does
// not pick it (review of #480, AlwxSin 7).
func TestSync_LegacySchemeMessageIsStable(t *testing.T) {
	const rel = "templates/rbacv2/use/view.yaml"

	seen := map[string]bool{}

	for range 20 {
		resetFixState()

		modulePath := syncModuleDir(t)
		model := syncModel(t, modulePath)
		writeGenerated(t, modulePath, model)

		store := renderedFrom(t, model, dropRule("d8:namespace-capability:cert-manager:view"))
		for _, kind := range []string{"use", "manage"} {
			putObject(t, store, rel, generate.Object{Kind: "ClusterRole", Name: "d8:" + kind + ":capability:module:cert-manager:x",
				Labels: map[string]string{rbaccontract.LabelKind: kind},
				Rules:  []generate.Rule{{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{"x.io"}, Resources: []string{"things"}, Verbs: []string{"get"}}}}})
		}

		for _, text := range texts(runSync(t, modulePath, store)) {
			if strings.Contains(text, "legacy RBACv2 scheme") {
				seen[text] = true
			}
		}
	}

	t.Cleanup(resetFixState)
	require.Len(t, seen, 1, "one text in every run: %v", seen)

	for text := range seen {
		assert.Contains(t, text, "rbac.deckhouse.io/kind: manage")
	}
}
