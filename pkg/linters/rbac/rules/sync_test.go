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
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/generate"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbacyaml"
)

const syncModule = "cert-manager"

// syncModuleDir lays out the cert-manager fixture of the generator as a module: its rbac.yaml,
// module.yaml and the CRDs the declaration relies on for scopes.
func syncModuleDir(t *testing.T) string {
	t.Helper()

	decl, err := os.ReadFile(filepath.Join("generate", "testdata", "cert-manager", "rbac.yaml"))
	require.NoError(t, err)

	return writeModule(t, map[string]string{
		rbacyaml.Filename: string(decl),
		"module.yaml":     "name: cert-manager\nnamespace: d8-cert-manager\nsubsystems: [security]\n",
		"crds/cm.yaml": crdYAML("cert-manager.io", "certificates", "Namespaced") + "---\n" +
			crdYAML("cert-manager.io", "certificaterequests", "Namespaced") + "---\n" +
			crdYAML("cert-manager.io", "issuers", "Namespaced") + "---\n" +
			crdYAML("cert-manager.io", "clusterissuers", "Cluster") + "---\n" +
			crdYAML("acme.cert-manager.io", "orders", "Namespaced") + "---\n" +
			crdYAML("acme.cert-manager.io", "challenges", "Namespaced"),
	})
}

func syncModel(t *testing.T, modulePath string) *generate.Model {
	t.Helper()

	decl, err := rbacyaml.Load(modulePath)
	require.NoError(t, err)

	model, err := generate.Build(generate.Input{Module: syncModule, Namespace: "d8-cert-manager", Subsystems: []string{"security"}, Decl: decl})
	require.NoError(t, err)

	return model
}

// renderedFrom simulates the Helm render of the model: every object as the chart would produce it,
// with the module labels helm_lib_module_labels adds. tweak may alter or drop an object (return
// false to drop it) before it is stored.
func renderedFrom(t *testing.T, model *generate.Model, tweak func(o *generate.Object) bool) *storage.UnstructuredObjectStore {
	t.Helper()

	store := storage.NewUnstructuredObjectStore()

	for _, file := range model.Files {
		for _, o := range file.Objects {
			obj := o
			if tweak != nil && !tweak(&obj) {
				continue
			}

			putObject(t, store, file.Path, obj)
		}
	}

	return store
}

func putObject(t *testing.T, store *storage.UnstructuredObjectStore, path string, o generate.Object) {
	t.Helper()

	labels := map[string]string{"heritage": "deckhouse", "module": syncModule}
	for k, v := range o.Labels {
		labels[k] = v
	}

	meta := metav1.ObjectMeta{Name: o.Name, Namespace: o.Namespace, Labels: labels, Annotations: o.Annotations}

	rules := make([]rbacv1.PolicyRule, 0, len(o.Rules))
	for _, r := range o.Rules {
		rules = append(rules, rbacv1.PolicyRule{APIGroups: r.APIGroups, Resources: r.Resources, ResourceNames: r.ResourceNames, NonResourceURLs: r.NonResourceURLs, Verbs: r.Verbs})
	}

	subjects := make([]rbacv1.Subject, 0, len(o.Subjects))
	for _, s := range o.Subjects {
		subjects = append(subjects, rbacv1.Subject{Kind: s.Kind, Name: s.Name, Namespace: s.Namespace})
	}

	roleRef := rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: o.RoleRefKind, Name: o.RoleRefName}

	var typed runtime.Object

	switch o.Kind {
	case "ClusterRole":
		typed = &rbacv1.ClusterRole{TypeMeta: metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: o.Kind}, ObjectMeta: meta, Rules: rules}
	case "Role":
		typed = &rbacv1.Role{TypeMeta: metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: o.Kind}, ObjectMeta: meta, Rules: rules}
	case "ClusterRoleBinding":
		typed = &rbacv1.ClusterRoleBinding{TypeMeta: metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: o.Kind}, ObjectMeta: meta, RoleRef: roleRef, Subjects: subjects}
	case "RoleBinding":
		typed = &rbacv1.RoleBinding{TypeMeta: metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: o.Kind}, ObjectMeta: meta, RoleRef: roleRef, Subjects: subjects}
	case "ServiceAccount":
		typed = &corev1.ServiceAccount{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: o.Kind}, ObjectMeta: meta, AutomountServiceAccountToken: o.AutomountToken}
	default:
		t.Fatalf("unexpected kind %s", o.Kind)
	}

	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(typed)
	require.NoError(t, err)
	require.NoError(t, store.Put("/module/"+path, path, content, []byte(o.Identity())))
}

func runSync(t *testing.T, modulePath string, store *storage.UnstructuredObjectStore) *errors.LintRuleErrorsList {
	t.Helper()

	m := mocks.NewModuleMock(minimock.NewController(t))
	m.GetPathMock.Return(modulePath)
	// The rule stops before reading the module when the declaration is missing or invalid.
	m.GetNameMock.Optional().Return(syncModule)
	m.GetNamespaceMock.Optional().Return("d8-cert-manager")
	m.GetStorageMock.Optional().Return(store.Storage)

	errorList := errors.NewLintRuleErrorsList()
	NewSyncRule(nil, m, errorList).Check(context.Background())

	return errorList
}

func TestSync_CleanRenderMatchesDeclaration(t *testing.T) {
	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)

	// An unmanaged controller ClusterRole beside the managed objects is nobody's business.
	store := renderedFrom(t, model, nil)
	putObject(t, store, "templates/cert-manager/rbac-for-us.yaml", generate.Object{
		Kind: "ClusterRole", Name: "d8:cert-manager:controller", Class: generate.ClassDeclared,
		Rules: []generate.Rule{{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}}}},
	})

	assert.Empty(t, texts(runSync(t, modulePath, store)))
}

func TestSync_Divergences(t *testing.T) {
	for name, tc := range map[string]struct {
		tweak func(o *generate.Object) bool
		extra func(t *testing.T, store *storage.UnstructuredObjectStore)
		want  []string
	}{
		"R13b: an unconditional rule is absent from the render": {
			tweak: func(o *generate.Object) bool {
				if o.Name == "d8:namespace-capability:cert-manager:view" {
					o.Rules = o.Rules[:len(o.Rules)-1] // drops issuers (sorted last)
				}

				return true
			},
			want: []string{"error: templates/rbacv2/use/view.yaml does not match rbac.yaml: ClusterRole/d8:namespace-capability:cert-manager:view: get cert-manager.io/issuers is declared but absent from the render; ClusterRole/d8:namespace-capability:cert-manager:view: list cert-manager.io/issuers is declared but absent from the render; ClusterRole/d8:namespace-capability:cert-manager:view: watch cert-manager.io/issuers is declared but absent from the render. Run `dmt lint --linter rbac --fix` to regenerate the file from the declaration"},
		},
		"R13a: a rule under when that did not render is not a divergence": {
			tweak: func(o *generate.Object) bool {
				kept := o.Rules[:0]
				for _, r := range o.Rules {
					if r.When == "" {
						kept = append(kept, r)
					}
				}

				o.Rules = kept

				return true
			},
			want: nil,
		},
		"a rule in the render that the declaration does not have": {
			tweak: func(o *generate.Object) bool {
				if o.Name == "d8:user-authz:cert-manager:user" {
					o.Rules = append(o.Rules, generate.Rule{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}}})
				}

				return true
			},
			want: []string{`error: templates/user-authz-cluster-roles.yaml does not match rbac.yaml: ClusterRole/d8:user-authz:cert-manager:user: get ""/secrets is in the render but not declared. Run ` + "`dmt lint --linter rbac --fix`" + ` to regenerate the file from the declaration`},
		},
		"R25a: the rules agree but a lineage is lost": {
			tweak: func(o *generate.Object) bool {
				if o.Name == "d8:system-capability:cert-manager:view" {
					delete(o.Labels, "rbac.deckhouse.io/aggregate-to-security-as")
				}

				return true
			},
			want: []string{"error: templates/rbacv2/manage/view.yaml does not match rbac.yaml: ClusterRole/d8:system-capability:cert-manager:view: aggregation into security=viewer is declared but absent from the render. Run `dmt lint --linter rbac --fix` to regenerate the file from the declaration"},
		},
		"a declared object is absent from the render": {
			tweak: func(o *generate.Object) bool {
				return o.Name != "access-to-cert-manager-auth" || o.Kind != "RoleBinding"
			},
			want: []string{"error: templates/rbac-to-us.yaml does not match rbac.yaml: d8-cert-manager/RoleBinding/access-to-cert-manager-auth is declared but absent from the render. Run `dmt lint --linter rbac --fix` to regenerate the file from the declaration"},
		},
		"a conditional object absent from the render is fine": {
			tweak: func(o *generate.Object) bool { return o.When == "" },
			want:  nil,
		},
		"D2: a legacy role the declaration does not produce": {
			extra: func(t *testing.T, store *storage.UnstructuredObjectStore) {
				putObject(t, store, "templates/user-authz-cluster-roles.yaml", generate.Object{
					Kind: "ClusterRole", Name: "d8:user-authz:cert-manager:super-admin", Class: generate.ClassLegacy,
					Annotations: map[string]string{"user-authz.deckhouse.io/access-level": "SuperAdmin"},
					Rules:       []generate.Rule{{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{"cert-manager.io"}, Resources: []string{"issuers"}, Verbs: []string{"deletecollection"}}}},
				})
			},
			want: []string{"error: templates/user-authz-cluster-roles.yaml does not match rbac.yaml: ClusterRole/d8:user-authz:cert-manager:super-admin is in the render but rbac.yaml does not produce it: declare its rights in rbac.yaml or remove it from the template. Run `dmt lint --linter rbac --fix` to regenerate the file from the declaration"},
		},
		"D2: a module capability in a file the declaration does not produce": {
			extra: func(t *testing.T, store *storage.UnstructuredObjectStore) {
				putObject(t, store, "templates/rbacv2/use/superadmin.yaml", generate.Object{
					Kind: "ClusterRole", Name: "d8:namespace-capability:cert-manager:superadmin", Class: generate.ClassCapability,
					Labels: map[string]string{"rbac.deckhouse.io/kind": "capability", "rbac.deckhouse.io/scope": "namespace", "rbac.deckhouse.io/aggregate-to-namespace-as": "superadmin"},
					Rules:  []generate.Rule{{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{"cert-manager.io"}, Resources: []string{"issuers"}, Verbs: []string{"deletecollection"}}}},
				})
			},
			want: []string{"error: templates/rbacv2/use/superadmin.yaml does not match rbac.yaml: ClusterRole/d8:namespace-capability:cert-manager:superadmin is in the render but rbac.yaml does not produce it: declare its rights in rbac.yaml or remove it from the template. Only a person can close this: the declaration does not produce this file"},
		},
		"a binding with a different subject": {
			tweak: func(o *generate.Object) bool {
				if o.Kind == "ClusterRoleBinding" && o.Name == "d8:cert-manager:admin-kubeconfig" {
					o.Subjects = []generate.Subject{{Kind: "Group", Name: "kubeadm:cluster-operators"}}
				}

				return true
			},
			want: []string{"error: templates/rbac-for-us.yaml does not match rbac.yaml: ClusterRoleBinding/d8:cert-manager:admin-kubeconfig: subject Group//kubeadm:cluster-admins is declared but absent from the render; ClusterRoleBinding/d8:cert-manager:admin-kubeconfig: subject Group//kubeadm:cluster-operators is in the render but not declared. Run `dmt lint --linter rbac --fix` to regenerate the file from the declaration"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			modulePath := syncModuleDir(t)
			model := syncModel(t, modulePath)
			store := renderedFrom(t, model, tc.tweak)

			if tc.extra != nil {
				tc.extra(t, store)
			}

			got := texts(runSync(t, modulePath, store))
			if tc.want == nil {
				assert.Empty(t, got)
				return
			}

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSync_InvalidDeclarationStopsEverything(t *testing.T) {
	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)

	// Break the declaration after the model is built: the render is fine, the file is not.
	require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), []byte("apiVersion: rbac.deckhouse.io/v1alpha1\nresources:\n  - group: cert-manager.io\n    resource: clusterissuers\n    namespace:\n      viewer: [get]\n"), 0o600))

	got := texts(runSync(t, modulePath, renderedFrom(t, model, nil)))
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], "namespace levels are not allowed for a cluster-scoped resource")
	assert.Contains(t, got[0], "nothing is compared or generated until the declaration is valid")
}

func TestSync_WithoutDeclarationIsSilent(t *testing.T) {
	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	require.NoError(t, os.Remove(rbacyaml.Path(modulePath)))

	assert.Empty(t, texts(runSync(t, modulePath, renderedFrom(t, model, nil))))
}

func TestSync_Autofix(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	t.Run("regenerates a missing file and is idempotent", func(t *testing.T) {
		resetFixState()

		modulePath := syncModuleDir(t)
		model := syncModel(t, modulePath)

		// Nothing of the use/view capability is rendered: the file is missing.
		store := renderedFrom(t, model, func(o *generate.Object) bool { return o.Name != "d8:namespace-capability:cert-manager:view" })
		errorList := runSync(t, modulePath, store)

		fixes := errorList.GetFixes()
		require.Len(t, fixes, 1)
		fixes[0]()

		remaining := errorList.GetErrors()
		assert.Empty(t, remaining, "a successful fix resolves the finding")

		written, err := os.ReadFile(filepath.Join(modulePath, "templates/rbacv2/use/view.yaml"))
		require.NoError(t, err)
		assert.Equal(t, generate.RenderFile(*model.File("templates/rbacv2/use/view.yaml")), string(written))

		generated, version := generate.ParseHeader(string(written))
		assert.True(t, generated)
		assert.Equal(t, "1", version)

		// Running the same fix again, in a new run, changes nothing.
		resetFixState()

		before, _ := os.Stat(filepath.Join(modulePath, "templates/rbacv2/use/view.yaml"))
		for _, fix := range runSync(t, modulePath, store).GetFixes() {
			fix()
		}

		after, _ := os.Stat(filepath.Join(modulePath, "templates/rbacv2/use/view.yaml"))
		assert.Equal(t, before.ModTime(), after.ModTime())
	})

	t.Run("D3: refuses to drop a right the render grants", func(t *testing.T) {
		resetFixState()

		modulePath := syncModuleDir(t)
		model := syncModel(t, modulePath)
		store := renderedFrom(t, model, func(o *generate.Object) bool {
			if o.Name == "d8:user-authz:cert-manager:user" {
				o.Rules = append(o.Rules, generate.Rule{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}}})
			}

			return true
		})

		// The file exists with a generator header, so only the guard stands in the way.
		path := filepath.Join(modulePath, "templates/user-authz-cluster-roles.yaml")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(generate.Header()+"\n# stale\n"), 0o600))

		errorList := runSync(t, modulePath, store)
		for _, fix := range errorList.GetFixes() {
			fix()
		}

		remaining := errorList.GetErrors()
		require.Len(t, remaining, 1)
		require.Error(t, remaining[0].FixError)
		assert.Contains(t, remaining[0].FixError.Error(), `would drop rights the render grants today: ClusterRole/d8:user-authz:cert-manager:user: get ""/secrets`)

		unchanged, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, generate.Header()+"\n# stale\n", string(unchanged), "the file is left alone")
	})

	t.Run("US-F2: a file without the header is maintained by hand", func(t *testing.T) {
		resetFixState()

		modulePath := syncModuleDir(t)
		model := syncModel(t, modulePath)
		store := renderedFrom(t, model, func(o *generate.Object) bool {
			if o.Name == "d8:namespace-capability:cert-manager:view" {
				o.Rules = o.Rules[:len(o.Rules)-1]
			}

			return true
		})

		path := filepath.Join(modulePath, "templates/rbacv2/use/view.yaml")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("# hand-made\napiVersion: v1\n"), 0o600))

		errorList := runSync(t, modulePath, store)
		for _, fix := range errorList.GetFixes() {
			fix()
		}

		remaining := errorList.GetErrors()
		require.Len(t, remaining, 1)
		require.Error(t, remaining[0].FixError)
		assert.Contains(t, remaining[0].FixError.Error(), "is maintained by hand (no generator header)")

		unchanged, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "# hand-made\napiVersion: v1\n", string(unchanged))

		aside, err := os.ReadFile(asidePath(path))
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(string(aside), generate.Header()))
	})
}

// R40: a file whose header names another contract version is a divergence, and the fix rewrites it.
func TestSync_ForeignContractVersionIsRegenerated(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	store := renderedFrom(t, model, nil)

	const rel = "templates/rbacv2/use/view.yaml"

	want := generate.RenderFile(*model.File(rel))
	_, body, _ := strings.Cut(want, "\n")
	stale := strings.Replace(generate.Header(), "contract 1.", "contract 0.", 1) + "\n" + body

	path := filepath.Join(modulePath, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(stale), 0o600))

	errorList := runSync(t, modulePath, store)
	got := texts(errorList)
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], `templates/rbacv2/use/view.yaml does not match rbac.yaml: the file was generated under contract version "0"; the current contract is "1". Run `+"`dmt lint --linter rbac --fix`")

	for _, fix := range errorList.GetFixes() {
		fix()
	}

	assert.Empty(t, errorList.GetErrors())

	written, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, want, string(written))
}

// R36/D3: under --matrix every variant records its render at lint time, so the closure that runs
// first refuses to drop a right only another variant rendered, and the other closures report the
// same outcome instead of writing.
func TestSync_FixSeesEveryRenderVariant(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)

	// Both variants lack a declared rule (the file is stale); variant A also grants a right that is
	// not declared -- as a template with a hand-written {{ if }} would under some values.
	stale := func(o *generate.Object) bool {
		if o.Name == "d8:user-authz:cert-manager:user" {
			o.Rules = o.Rules[:len(o.Rules)-1]
		}

		return true
	}
	variantB := renderedFrom(t, model, stale)
	variantA := renderedFrom(t, model, func(o *generate.Object) bool {
		stale(o)

		if o.Name == "d8:user-authz:cert-manager:user" {
			o.Rules = append(o.Rules, generate.Rule{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}}})
		}

		return true
	})

	path := filepath.Join(modulePath, "templates/user-authz-cluster-roles.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(generate.Header()+"\n# stale\n"), 0o600))

	// Lint both variants first, as the manager does, then apply the fixes: B's closure runs first.
	listB := runSync(t, modulePath, variantB)
	listA := runSync(t, modulePath, variantA)
	require.Len(t, listB.GetFixes(), 1)
	require.Len(t, listA.GetFixes(), 1)

	for _, list := range []*errors.LintRuleErrorsList{listB, listA} {
		for _, fix := range list.GetFixes() {
			fix()
		}
	}

	for name, list := range map[string]*errors.LintRuleErrorsList{"B": listB, "A": listA} {
		remaining := list.GetErrors()
		require.Len(t, remaining, 1, "variant %s", name)
		require.Error(t, remaining[0].FixError, "variant %s", name)
		assert.Contains(t, remaining[0].FixError.Error(), `would drop rights the render grants today: ClusterRole/d8:user-authz:cert-manager:user: get ""/secrets`, "variant %s", name)
	}

	unchanged, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, generate.Header()+"\n# stale\n", string(unchanged), "no variant wrote the file")
}

// R8a/D7: a declaration in an edition overlay is reported by sync and ignored by coverage.
func TestSync_DeclarationInEditionOverlay(t *testing.T) {
	src := syncModuleDir(t)
	modulePath := filepath.Join(t.TempDir(), "ee", "be", "modules", "101-cert-manager")
	require.NoError(t, os.MkdirAll(filepath.Dir(modulePath), 0o755))
	require.NoError(t, os.Rename(src, modulePath))

	got := texts(runSync(t, modulePath, storage.NewUnstructuredObjectStore()))
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], "rbac.yaml lies in the edition overlay ee/be/modules; the declaration describes the union of editions and belongs to modules/<module>/ only")
	assert.Contains(t, got[0], "Only a person can close this")

	assert.Empty(t, texts(runCoverage(t, modulePath)), "coverage leaves the overlay finding to sync")
}

func TestEditionOverlay(t *testing.T) {
	for path, want := range map[string]string{
		"/r/modules/101-cert-manager":  "",
		"/r/ee/modules/500-x":          "ee/modules",
		"/r/ee/be/modules/500-x":       "ee/be/modules",
		"/r/ee/se-plus/modules/500-x/": "ee/se-plus/modules",
		"/r/ee/fe/x":                   "",
		"/r/external/x":                "",
		"/r/ee/x":                      "",
		"modules/x":                    "",
	} {
		assert.Equal(t, want, editionOverlay(path), path)
	}
}

// A template that still renders the manage/use scheme where the declaration produces the 1.78
// model: the declared objects are absent, and the finding says why.
func TestSync_LegacyTemplateIsNamed(t *testing.T) {
	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	store := renderedFrom(t, model, func(o *generate.Object) bool { return o.Name != "d8:namespace-capability:cert-manager:view" })

	putObject(t, store, "templates/rbacv2/use/view.yaml", generate.Object{
		Kind: "ClusterRole", Name: "d8:use:capability:module:cert-manager:view", Class: generate.ClassCapability,
		Labels: map[string]string{"rbac.deckhouse.io/kind": "use", "rbac.deckhouse.io/aggregate-to-kubernetes-as": "viewer"},
		Rules:  []generate.Rule{{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{"cert-manager.io"}, Resources: []string{"certificates"}, Verbs: []string{"get"}}}},
	})

	got := texts(runSync(t, modulePath, store))
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], "ClusterRole/d8:namespace-capability:cert-manager:view is declared but absent from the render")
	assert.Contains(t, got[0], "the template renders the legacy RBACv2 scheme (rbac.deckhouse.io/kind: use, the manage/use model before DKP 1.78) where the declaration produces the 1.78 model; migrate the module with rbacv2-migrate-module.sh")
	assert.NotContains(t, got[0], "d8:use:capability", "the legacy object itself is not reported as extra")
}

// R30: a template that serves both schemes behind the version gate is never regenerated, header or not.
func TestSync_GatedTemplateIsNotRegenerated(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	store := renderedFrom(t, model, func(o *generate.Object) bool {
		if o.Name == "d8:namespace-capability:cert-manager:view" {
			o.Rules = o.Rules[:len(o.Rules)-1]
		}

		return true
	})

	const gated = "{{- if eq (include \"cert-manager.rbacv2_new_scheme\" .) \"true\" }}\n# new\n{{- else }}\n# legacy\n{{- end }}\n"

	path := filepath.Join(modulePath, "templates/rbacv2/use/view.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(gated), 0o600))

	errorList := runSync(t, modulePath, store)
	for _, fix := range errorList.GetFixes() {
		fix()
	}

	remaining := errorList.GetErrors()
	require.Len(t, remaining, 1)
	require.Error(t, remaining[0].FixError)
	assert.Contains(t, remaining[0].FixError.Error(), "renders one of two role models depending on the platform version (the rbacv2_new_scheme gate of rbacv2-migrate-module.sh); regenerating it would drop the legacy branch")

	unchanged, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, gated, string(unchanged))

	_, err = os.Stat(asidePath(path))
	assert.True(t, os.IsNotExist(err), "no .generated copy for a gated file")
}

// A gated template whose legacy branch rendered (values below 1.78) is not a divergence: the 1.78
// objects it declares are compared in the run where the gate answers "new".
func TestSync_GatedTemplateRenderingLegacyBranchIsSilent(t *testing.T) {
	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	store := renderedFrom(t, model, func(o *generate.Object) bool { return o.Name != "d8:namespace-capability:cert-manager:view" })

	putObject(t, store, "templates/rbacv2/use/view.yaml", generate.Object{
		Kind: "ClusterRole", Name: "d8:use:capability:module:cert-manager:view", Class: generate.ClassCapability,
		Labels: map[string]string{"rbac.deckhouse.io/kind": "use", "rbac.deckhouse.io/aggregate-to-kubernetes-as": "viewer"},
		Rules:  []generate.Rule{{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{"cert-manager.io"}, Resources: []string{"certificates"}, Verbs: []string{"get"}}}},
	})

	path := filepath.Join(modulePath, "templates/rbacv2/use/view.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("{{- if eq (include \"cert-manager.rbacv2_new_scheme\" .) \"true\" }}\n# new\n{{- else }}\n# legacy\n{{- end }}\n"), 0o600))

	assert.Empty(t, texts(runSync(t, modulePath, store)))
}

// A generator-owned file must be the text the declaration renders now: a rule under `when` that is
// false today is invisible to the render, so only the text says whether it reached the template.
func TestSync_GeneratedFileTextIsCompared(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	store := renderedFrom(t, model, nil)

	const rel = "templates/rbacv2/use/view.yaml"

	want := generate.RenderFile(*model.File(rel))
	path := filepath.Join(modulePath, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))

	t.Run("the exact text is not a divergence", func(t *testing.T) {
		require.NoError(t, os.WriteFile(path, []byte(want), 0o600))
		assert.Empty(t, texts(runSync(t, modulePath, store)))
	})

	t.Run("a stale generated file is regenerated", func(t *testing.T) {
		stale := strings.Replace(want, "\n{{- if .Values.certManager.internal.acmeEnabled }}", "\n# a conditional rule was declared after this file was generated\n{{- if .Values.certManager.internal.acmeEnabled }}", 1)
		require.NotEqual(t, want, stale, "the fixture must carry a conditional rule")
		require.NoError(t, os.WriteFile(path, []byte(stale), 0o600))

		errorList := runSync(t, modulePath, store)
		got := texts(errorList)
		require.Len(t, got, 1, "got: %v", got)
		assert.Contains(t, got[0], "the file carries the generator header but is not what the declaration renders now")

		for _, fix := range errorList.GetFixes() {
			fix()
		}

		assert.Empty(t, errorList.GetErrors())

		written, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, want, string(written))
	})

	t.Run("a hand-maintained file is judged by its render only", func(t *testing.T) {
		_, body, _ := strings.Cut(want, "\n")
		require.NoError(t, os.WriteFile(path, []byte("# hand-maintained\n"+body), 0o600))
		assert.Empty(t, texts(runSync(t, modulePath, store)))
	})
}
