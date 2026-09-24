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
	yaml "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/generate"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbaccontract"
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

// writeGenerated puts every file of the model on disk as the generator writes it: the render the
// tests simulate came from somewhere, and a declared file that does not exist is a finding of its own.
func writeGenerated(t *testing.T, modulePath string, model *generate.Model) {
	t.Helper()

	for _, r := range generate.Render(model) {
		full := filepath.Join(modulePath, r.Path)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(r.Content), 0o600))
	}
}

func runSync(t *testing.T, modulePath string, store *storage.UnstructuredObjectStore, excludes ...pkg.KindRuleExclude) *errors.LintRuleErrorsList {
	t.Helper()

	m := mocks.NewModuleMock(minimock.NewController(t))
	m.GetPathMock.Return(modulePath)
	// The rule stops before reading the module when the declaration is missing or invalid.
	m.GetNameMock.Optional().Return(syncModule)
	m.GetNamespaceMock.Optional().Return("d8-cert-manager")
	m.GetStorageMock.Optional().Return(store.Storage)
	m.GetObjectStoreMock.Optional().Return(store)

	errorList := errors.NewLintRuleErrorsList()
	NewSyncRule(excludes, m, errorList).Check(context.Background())

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
		"D2: capabilities of the project lineage and platform-wide ones are not the declaration's": {
			extra: func(t *testing.T, store *storage.UnstructuredObjectStore) {
				putObject(t, store, "templates/rbacv2/project/capabilities/manage_rbac.yaml", generate.Object{
					Kind: "ClusterRole", Name: "d8:project-capability:cert-manager:manage_rbac", Class: generate.ClassCapability,
					Labels: map[string]string{"rbac.deckhouse.io/kind": "capability", "rbac.deckhouse.io/scope": "project", "rbac.deckhouse.io/aggregate-to-project-as": "admin"},
					Rules:  []generate.Rule{{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{"rbac.authorization.k8s.io"}, Resources: []string{"rolebindings"}, Verbs: []string{"create"}}}},
				})
				putObject(t, store, "templates/rbacv2/global/namespace/capabilities/view_logs.yaml", generate.Object{
					Kind: "ClusterRole", Name: "d8:namespace-capability:kubernetes:view_logs", Class: generate.ClassCapability,
					Labels: map[string]string{"rbac.deckhouse.io/kind": "capability", "rbac.deckhouse.io/scope": "namespace", "rbac.deckhouse.io/aggregate-to-namespace-as": "viewer"},
					Rules:  []generate.Rule{{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{""}, Resources: []string{"pods/log"}, Verbs: []string{"get"}}}},
				})
			},
			want: nil,
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
			writeGenerated(t, modulePath, model)
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

// Without rbac.yaml the rule reports the declaration missing, and --fix writes it from the render:
// the file a person would have transcribed from the templates, ready to be read and corrected.
func TestSync_WithoutDeclarationBootstrapsIt(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	require.NoError(t, os.Remove(rbacyaml.Path(modulePath)))

	errorList := runSync(t, modulePath, renderedFrom(t, model, nil))
	got := texts(errorList)
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], "rbac.yaml is missing: `dmt lint --linter rbac --fix` writes it from the RBAC objects the module renders today (22 of 22 objects described")

	for _, fix := range errorList.GetFixes() {
		fix()
	}

	assert.Empty(t, errorList.GetErrors())

	written, err := rbacyaml.Load(modulePath)
	require.NoError(t, err, "the written declaration parses")
	assert.Len(t, written.ServiceAccounts, 1)
	assert.NotEmpty(t, written.Resources)

	// The next run compares against it and, the render being what it declares, is silent.
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
		assert.Equal(t, rbaccontract.ContractVersion, version)

		// Running the same fix again, in a new run, changes nothing.
		resetFixState()

		list := runSync(t, modulePath, store)
		for _, fix := range list.GetFixes() {
			fix()
		}

		after, err := os.ReadFile(filepath.Join(modulePath, "templates/rbacv2/use/view.yaml"))
		require.NoError(t, err)
		assert.Equal(t, string(written), string(after))
		assert.Empty(t, list.GetErrors(), "a no-op run reports no fix error")
	})

	t.Run("the declaration wins: a right it does not name leaves the template", func(t *testing.T) {
		resetFixState()

		modulePath := syncModuleDir(t)
		model := syncModel(t, modulePath)
		store := renderedFrom(t, model, func(o *generate.Object) bool {
			if o.Name == "d8:user-authz:cert-manager:user" {
				o.Rules = append(o.Rules, generate.Rule{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}}})
			}

			return true
		})

		const rel = "templates/user-authz-cluster-roles.yaml"

		path := filepath.Join(modulePath, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(generate.Header()+"\n# stale, with the secrets rule the declaration does not name\n"), 0o600))

		errorList := runSync(t, modulePath, store)
		got := texts(errorList)
		require.Len(t, got, 1, "got: %v", got)
		assert.Contains(t, got[0], `get ""/secrets is in the render but not declared`, "the finding names what the rewrite removes")

		for _, fix := range errorList.GetFixes() {
			fix()
		}

		assert.Empty(t, errorList.GetErrors())

		written, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, generate.RenderFile(*model.File(rel)), string(written))
		assert.NotContains(t, string(written), "secrets")
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
	stale := strings.Replace(generate.Header(), "contract "+rbaccontract.ContractVersion+".", "contract 0.", 1) + "\n" + body

	path := filepath.Join(modulePath, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(stale), 0o600))

	errorList := runSync(t, modulePath, store)
	got := texts(errorList)
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], `templates/rbacv2/use/view.yaml does not match rbac.yaml: the file was generated under contract version "0"; the current contract is "`+rbaccontract.ContractVersion+`". Run `+"`dmt lint --linter rbac --fix`")

	for _, fix := range errorList.GetFixes() {
		fix()
	}

	assert.Empty(t, errorList.GetErrors())

	written, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, want, string(written))
}

// R36: under --matrix every variant records at lint time the objects it rendered into a file that the
// declaration does not produce; the closure that runs first judges the union, so a foreign object
// rendered only under some values still protects the file, and the other closures report the same.
func TestSync_FixSeesEveryRenderVariant(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	const rel = "templates/rbacv2/use/view.yaml"

	// Both variants lack a declared rule (the file is stale); variant A also renders a foreign
	// object from the same file, as a hand-added {{ if }} block would under some values.
	stale := func(o *generate.Object) bool {
		if o.Name == "d8:namespace-capability:cert-manager:view" {
			o.Rules = o.Rules[:len(o.Rules)-1]
		}

		return true
	}
	variantB := renderedFrom(t, model, stale)
	variantA := renderedFrom(t, model, stale)
	putObject(t, variantA, rel, generate.Object{
		Kind: "ClusterRole", Name: "d8:cert-manager:only-sometimes", Class: generate.ClassDeclared,
		Rules: []generate.Rule{{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}}}},
	})

	before, err := os.ReadFile(filepath.Join(modulePath, rel))
	require.NoError(t, err)

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
		assert.Contains(t, remaining[0].FixError.Error(), "also holds objects the declaration does not produce: ClusterRole/d8:cert-manager:only-sometimes", "variant %s", name)
	}

	unchanged, err := os.ReadFile(filepath.Join(modulePath, rel))
	require.NoError(t, err)
	assert.Equal(t, string(before), string(unchanged), "no variant wrote the file")
}

// R8a/D7: a declaration in an edition overlay is reported by sync and ignored by coverage.
func TestSync_DeclarationInEditionOverlay(t *testing.T) {
	src := syncModuleDir(t)
	root := t.TempDir()
	// The module has a base in modules/, so the edition directory is an overlay.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "modules", "101-cert-manager"), 0o755))
	modulePath := filepath.Join(root, "ee", "be", "modules", "101-cert-manager")
	require.NoError(t, os.MkdirAll(filepath.Dir(modulePath), 0o755))
	require.NoError(t, os.Rename(src, modulePath))

	got := texts(runSync(t, modulePath, storage.NewUnstructuredObjectStore()))
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], "rbac.yaml lies in the edition overlay ee/be/modules; the declaration describes the union of editions and belongs to modules/<module>/ only")
	assert.Contains(t, got[0], "Only a person can close this")

	assert.Empty(t, texts(runCoverage(t, modulePath)), "coverage leaves the overlay finding to sync")
}

func TestEditionOverlay(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "modules", "110-istio"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "ee", "modules", "110-istio"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "ee", "modules", "030-cloud-provider-openstack"), 0o755))

	assert.Equal(t, "ee/modules", editionOverlay(filepath.Join(root, "ee", "modules", "110-istio")), "merged over modules/110-istio")
	assert.Empty(t, editionOverlay(filepath.Join(root, "ee", "modules", "030-cloud-provider-openstack")), "an EE-only module: ee/modules is its base")

	// Review of #479, finding 19: a module of one edition directory only has its base there.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "ee", "be", "modules", "350-node-local-dns"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "ee", "se-plus", "modules", "110-istio"), 0o755))
	assert.Empty(t, editionOverlay(filepath.Join(root, "ee", "be", "modules", "350-node-local-dns")), "only in ee/be")
	assert.Equal(t, "ee/se-plus/modules", editionOverlay(filepath.Join(root, "ee", "se-plus", "modules", "110-istio")), "merged over a base")

	for path, want := range map[string]string{
		"/r/modules/101-cert-manager": "",
		"/r/ee/be/modules/500-x":      "", // no base anywhere: this edition is its home
		"/r/ee/fe/x":                  "",
		"/r/external/x":               "",
		"/r/ee/x":                     "",
		"modules/x":                   "",
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
	assert.Contains(t, remaining[0].FixError.Error(), "renders one of two role models depending on the platform version (the rbacv2_new_scheme gate of rbacv2-migrate-module.sh, or a deckhouseVersion test the declaration did not produce); regenerating it would drop the legacy branch")

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

// A file whose objects are all under `when`, deleted: the render cannot miss them (D4), the text can.
func TestSync_MissingFileWithConditionalObjectsIsReported(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)

	const rel = "templates/cainjector/rbac-for-us.yaml"

	file := model.File(rel)
	require.NotNil(t, file)

	for _, o := range file.Objects {
		require.NotEmpty(t, o.When, "the fixture's cainjector objects are conditional")
	}

	// Rendered as with the condition false: none of the cainjector objects, no file on disk.
	store := renderedFrom(t, model, func(o *generate.Object) bool { return o.When == "" })

	errorList := runSync(t, modulePath, store)
	got := texts(errorList)
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], "templates/cainjector/rbac-for-us.yaml does not match rbac.yaml: the file does not exist, and objects the declaration puts in it are absent from the render (objects under `when` included")

	for _, fix := range errorList.GetFixes() {
		fix()
	}

	assert.Empty(t, errorList.GetErrors())

	written, err := os.ReadFile(filepath.Join(modulePath, rel))
	require.NoError(t, err)
	assert.Equal(t, generate.RenderFile(*file), string(written))

	// With the file in place and the condition still false, nothing is reported.
	assert.Empty(t, texts(runSync(t, modulePath, store)))
}

// A generated file that also holds an object the declaration does not produce is never rewritten:
// the generator writes the whole file, and the foreign object would vanish with it.
func TestSync_FileWithForeignObjectsIsNotRegenerated(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	const rel = "templates/cainjector/rbac-for-us.yaml"

	// The render: the cainjector file lacks a declared rule and carries a controller ClusterRole
	// of its own that nobody declared.
	store := renderedFrom(t, model, func(o *generate.Object) bool {
		if o.Kind == "ClusterRole" && o.Name == "d8:cert-manager:cainjector" {
			o.Rules = o.Rules[:1]
		}

		return true
	})
	putObject(t, store, rel, generate.Object{
		Kind: "ClusterRole", Name: "d8:cert-manager:cainjector:requester", Class: generate.ClassDeclared,
		Rules: []generate.Rule{{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}}}},
	})

	before, err := os.ReadFile(filepath.Join(modulePath, rel))
	require.NoError(t, err)

	errorList := runSync(t, modulePath, store)
	for _, fix := range errorList.GetFixes() {
		fix()
	}

	remaining := errorList.GetErrors()
	require.Len(t, remaining, 1)
	require.Error(t, remaining[0].FixError)
	assert.Contains(t, remaining[0].FixError.Error(), "templates/cainjector/rbac-for-us.yaml also holds objects the declaration does not produce: ClusterRole/d8:cert-manager:cainjector:requester; regenerating the file would drop them")

	after, err := os.ReadFile(filepath.Join(modulePath, rel))
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "the file is left alone")
}

// A rendered object the generator produces under another name is replaced, not foreign: a binding
// with the same roleRef and subjects, a role with the same rules.
func TestSync_RenamedObjectsAreNotForeign(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	const rel = "templates/rbac-to-us.yaml"

	// A file of contract 1 lists no owned objects; only there does a rename apply.
	asContractOne(t, filepath.Join(modulePath, rel))

	// The render still carries the old names a hand-written module gave the metrics access: the
	// Role and its RoleBinding, with the same rules, roleRef and subjects.
	store := renderedFrom(t, model, func(o *generate.Object) bool {
		if o.Name == "access-to-cert-manager" && (o.Kind == "Role" || o.Kind == "RoleBinding") {
			o.Name = "access-to-cert-manager-prometheus-metrics"

			if o.Kind == "RoleBinding" {
				o.RoleRefName = "access-to-cert-manager-prometheus-metrics"
			}
		}

		return true
	})

	errorList := runSync(t, modulePath, store)
	got := texts(errorList)
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], "is declared but absent from the render")

	for _, fix := range errorList.GetFixes() {
		fix()
	}

	assert.Empty(t, errorList.GetErrors(), "the renamed bindings are replaced, so the file is regenerated")

	written, err := os.ReadFile(filepath.Join(modulePath, rel))
	require.NoError(t, err)
	assert.Equal(t, generate.RenderFile(*model.File(rel)), string(written))
}

// An rbac.yaml of the earlier, never consumed shape is named for what it is.
func TestSync_OldShapeFileIsNamed(t *testing.T) {
	modulePath := syncModuleDir(t)
	require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), []byte("crds:\n  - certificates\n"), 0o600))

	got := texts(runSync(t, modulePath, storage.NewUnstructuredObjectStore()))
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], "rbac.yaml is not a declaration (no apiVersion): an rbac.yaml of an earlier shape that nothing reads; delete it and run `dmt lint --linter rbac --fix`")
}

// An object the declaration puts in another file is left where it renders: the fix does not move
// objects between files (review of #479, findings 15, 23, 25 and 29).
func TestSync_MisplacedObjectIsNamed(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	// The edit capability renders from view.yaml; the declaration puts it in edit.yaml.
	store := renderedFrom(t, model, func(o *generate.Object) bool { return o.Name != "d8:namespace-capability:cert-manager:edit" })
	edit := *model.File("templates/rbacv2/use/edit.yaml")
	putObject(t, store, "templates/rbacv2/use/view.yaml", edit.Objects[0])

	viewPath := filepath.Join(modulePath, "templates/rbacv2/use/view.yaml")
	editPath := filepath.Join(modulePath, "templates/rbacv2/use/edit.yaml")

	require.NoError(t, os.WriteFile(viewPath, []byte(generate.Header()+"\n# stale\n"), 0o600))
	require.NoError(t, os.WriteFile(editPath, []byte(generate.Header()+"\n# stale\n"), 0o600))

	errorList := runSync(t, modulePath, store)
	for _, fix := range errorList.GetFixes() {
		fix()
	}

	var messages []string

	for _, e := range errorList.GetErrors() {
		if e.FixError != nil {
			messages = append(messages, e.FixError.Error())
		}
	}

	joined := strings.Join(messages, "\n")
	assert.Contains(t, joined, "ClusterRole/d8:namespace-capability:cert-manager:edit (the declaration now puts it in templates/rbacv2/use/edit.yaml; the fix does not move objects between files")
	assert.Contains(t, joined, "templates/rbacv2/use/edit.yaml would produce objects that still render from another file: ClusterRole/d8:namespace-capability:cert-manager:edit (renders from templates/rbacv2/use/view.yaml)",
		"the target is not written either, so the object never renders twice")

	for _, path := range []string{viewPath, editPath} {
		kept, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, generate.Header()+"\n# stale\n", string(kept), path+" is left alone")
	}
}

// Under --matrix the first declaration is written from the union of every variant's render.
func TestSync_BootstrapUnitesRenderVariants(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	require.NoError(t, os.Remove(rbacyaml.Path(modulePath)))

	// Variant B rendered with the cainjector disabled, variant A with it enabled; B lints first.
	variantB := renderedFrom(t, model, func(o *generate.Object) bool { return o.When == "" })
	variantA := renderedFrom(t, model, nil)

	listB := runSync(t, modulePath, variantB)
	listA := runSync(t, modulePath, variantA)

	for _, list := range []*errors.LintRuleErrorsList{listB, listA} {
		for _, fix := range list.GetFixes() {
			fix()
		}
	}

	written, err := rbacyaml.Load(modulePath)
	require.NoError(t, err)
	require.Len(t, written.ServiceAccounts, 1, "the cainjector account, seen only by variant A, is in the declaration")
	assert.Equal(t, "cainjector", written.ServiceAccounts[0].Name)
}

// A generated file the declaration produces nothing for any more is an orphan: --fix deletes it.
// One that also holds an object outside the owned classes, or serves both models, stays.
func TestSync_OrphanGeneratedFileIsDeleted(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)
	store := renderedFrom(t, model, nil)

	// The legacy section leaves the declaration; the render still has the roles from the file.
	decl, err := rbacyaml.Load(modulePath)
	require.NoError(t, err)

	for i := range decl.Resources {
		decl.Resources[i].Legacy = nil
	}

	raw, err := yaml.Marshal(decl)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), raw, 0o600))

	const rel = "templates/user-authz-cluster-roles.yaml"

	errorList := runSync(t, modulePath, store)
	got := texts(errorList)
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], "The file carries the generator header and the declaration produces nothing for it; `dmt lint --linter rbac --fix` deletes it")

	for _, fix := range errorList.GetFixes() {
		fix()
	}

	assert.Empty(t, errorList.GetErrors())

	_, err = os.Stat(filepath.Join(modulePath, rel))
	assert.True(t, os.IsNotExist(err), "the orphan is gone")

	// The same file with a foreign object beside the legacy roles is not deleted.
	resetFixState()
	writeGenerated(t, modulePath, model)
	putObject(t, store, rel, generate.Object{
		Kind: "ClusterRole", Name: "d8:cert-manager:something-else", Class: generate.ClassDeclared,
		Rules: []generate.Rule{{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}}}},
	})

	errorList = runSync(t, modulePath, store)
	got = texts(errorList)
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], "the file also holds ClusterRole/d8:cert-manager:something-else, which the declaration does not describe. Only a person can close this")
	assert.Empty(t, errorList.GetFixes())
}

// exclude-rules.sync silences an object's findings without forgetting the object: a declared
// object that is excluded is neither compared nor reported as absent.
func TestSync_ExcludedObjectIsSilentButKnown(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	const capability = "d8:namespace-capability:cert-manager:view"

	exclude := pkg.KindRuleExclude{Kind: "ClusterRole", Name: capability}

	// The capability rendered with a rule the declaration does not name.
	store := renderedFrom(t, model, func(o *generate.Object) bool {
		if o.Kind == "ClusterRole" && o.Name == capability {
			o.Rules = append(o.Rules, generate.Rule{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}}})
		}

		return true
	})
	got := texts(runSync(t, modulePath, store, exclude))
	assert.Empty(t, got, "got: %v", got)

	// The capability is not rendered at all.
	store = renderedFrom(t, model, func(o *generate.Object) bool { return o.Name != capability })
	got = texts(runSync(t, modulePath, store, exclude))
	assert.Empty(t, got, "got: %v", got)

	// Without the exclusion the same render is a finding.
	got = texts(runSync(t, modulePath, store))
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], "is declared but absent from the render")
}

// A ServiceAccount is compared like every other declared object: a token the render mounts but
// the declaration does not is a divergence, since the regeneration would take it away.
func TestSync_ServiceAccountAutomountIsCompared(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	yes := true
	store := renderedFrom(t, model, func(o *generate.Object) bool {
		if o.Kind == "ServiceAccount" {
			o.AutomountToken = &yes
		}

		return true
	})

	got := texts(runSync(t, modulePath, store))
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], "ServiceAccount/cainjector: automountServiceAccountToken is true in the render, the declaration produces false")
}

// A rendered binding under another name is a rename only when it points at the role the produced
// binding replaces. One that binds the same subjects to cluster-admin is a foreign object, and
// the file it lives in is not regenerated.
func TestSync_BindingToAnotherRoleIsNotARename(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	const rel = "templates/rbac-to-us.yaml"

	store := renderedFrom(t, model, func(o *generate.Object) bool {
		if o.Kind == "RoleBinding" && o.Name == "access-to-cert-manager" {
			o.Name = "access-to-cert-manager-prometheus-metrics"
			o.RoleRefKind = "ClusterRole"
			o.RoleRefName = "cluster-admin"
		}

		return true
	})

	before, err := os.ReadFile(filepath.Join(modulePath, rel))
	require.NoError(t, err)

	errorList := runSync(t, modulePath, store)
	for _, fix := range errorList.GetFixes() {
		fix()
	}

	remaining := errorList.GetErrors()
	require.Len(t, remaining, 1, "got: %v", texts(errorList))
	require.Error(t, remaining[0].FixError)
	assert.Contains(t, remaining[0].FixError.Error(), "also holds objects the declaration does not produce: d8-cert-manager/RoleBinding/access-to-cert-manager-prometheus-metrics")

	after, err := os.ReadFile(filepath.Join(modulePath, rel))
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "the file is left alone")
}

// The orphan fix judges the union of render variants: an object another variant placed in the
// file keeps the file, and a header that vanished between lint and fix keeps it too.
func TestSync_OrphanDeletionRefusedByOtherVariantsAndByHand(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)
	store := renderedFrom(t, model, nil)

	decl, err := rbacyaml.Load(modulePath)
	require.NoError(t, err)

	for i := range decl.Resources {
		decl.Resources[i].Legacy = nil
	}

	raw, err := yaml.Marshal(decl)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), raw, 0o600))

	const rel = "templates/user-authz-cluster-roles.yaml"

	fullPath := filepath.Join(modulePath, rel)

	t.Run("another variant holds a foreign object", func(t *testing.T) {
		errorList := runSync(t, modulePath, store)
		fixes := errorList.GetFixes()
		require.Len(t, fixes, 1)

		// What the run under other values found in the same file.
		recordForeignObjects(fullPath, []string{"ClusterRole/d8:cert-manager:only-under-other-values"})

		fixes[0]()

		remaining := errorList.GetErrors()
		require.Len(t, remaining, 1)
		require.Error(t, remaining[0].FixError)
		assert.Contains(t, remaining[0].FixError.Error(), "also holds objects the declaration does not describe (ClusterRole/d8:cert-manager:only-under-other-values), some only under other values; it is not deleted")

		_, err := os.Stat(fullPath)
		require.NoError(t, err, "the file stays")
	})

	t.Run("the header left the file before the fix ran", func(t *testing.T) {
		resetFixState()

		errorList := runSync(t, modulePath, store)
		fixes := errorList.GetFixes()
		require.Len(t, fixes, 1)

		content, err := os.ReadFile(fullPath)
		require.NoError(t, err)

		_, rest, _ := strings.Cut(string(content), "\n")
		require.NoError(t, os.WriteFile(fullPath, []byte(rest), 0o600))

		fixes[0]()

		remaining := errorList.GetErrors()
		require.Len(t, remaining, 1)
		require.Error(t, remaining[0].FixError)
		assert.Contains(t, remaining[0].FixError.Error(), "is not the generator's to delete any more")

		_, err = os.Stat(fullPath)
		require.NoError(t, err, "the file stays")
	})
}

// A `when` that tests the platform version is the declaration's own; the produced file carries
// the same action, so it is not mistaken for the migration gate and is regenerated.
func TestSync_WhenOnDeckhouseVersionIsNotAGate(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)

	decl, err := rbacyaml.Load(modulePath)
	require.NoError(t, err)

	decl.Resources[0].When = `semverCompare ">= 1.80" .Values.global.deckhouseVersion`

	raw, err := yaml.Marshal(decl)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), raw, 0o600))

	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	var rel string

	for _, f := range model.Files {
		if strings.Contains(generate.RenderFile(f), "deckhouseVersion") {
			rel = f.Path
		}
	}

	require.NotEmpty(t, rel, "a file renders the version test")

	// The file is stale: a comment was appended by hand.
	fullPath := filepath.Join(modulePath, rel)
	content, err := os.ReadFile(fullPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(fullPath, append(content, []byte("# a stray edit\n")...), 0o600))

	errorList := runSync(t, modulePath, renderedFrom(t, model, nil))
	got := texts(errorList)
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], "does not match rbac.yaml")

	for _, fix := range errorList.GetFixes() {
		fix()
	}

	assert.Empty(t, errorList.GetErrors(), "the file is regenerated, not mistaken for a gated template")

	after, err := os.ReadFile(fullPath)
	require.NoError(t, err)
	assert.Equal(t, generate.RenderFile(*model.File(rel)), string(after))
}

// asContractOne rewrites a generated file the way contract 1 wrote it: the header without the list
// of owned objects.
func asContractOne(t *testing.T, path string) {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	kept := lines[:1]
	kept[0] = strings.Replace(kept[0], "contract "+rbaccontract.ContractVersion+".", "contract 1.", 1)

	for _, l := range lines[1:] {
		if !strings.HasPrefix(l, "# dmt:owns ") {
			kept = append(kept, l)
		}
	}

	require.NoError(t, os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o600))
}

// A generated file may carry objects of other kinds; the fix never drops them (review of #479,
// finding 2): neither on a regeneration nor when the declaration stops producing the file.
func TestSync_NonRBACObjectsInGeneratedFilesAreKept(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	const rel = "templates/cainjector/rbac-for-us.yaml"

	store := renderedFrom(t, model, func(o *generate.Object) bool {
		if o.Kind == "ClusterRole" && o.Name == "d8:cert-manager:cainjector" {
			o.Rules = o.Rules[:1]
		}

		return true
	})
	putConfigMap(t, store, rel, "cainjector-extra")

	before, err := os.ReadFile(filepath.Join(modulePath, rel))
	require.NoError(t, err)

	errorList := runSync(t, modulePath, store)
	for _, fix := range errorList.GetFixes() {
		fix()
	}

	remaining := errorList.GetErrors()
	require.Len(t, remaining, 1, "got: %v", texts(errorList))
	require.Error(t, remaining[0].FixError)
	assert.Contains(t, remaining[0].FixError.Error(), "also holds objects the declaration does not produce: d8-cert-manager/ConfigMap/cainjector-extra")

	after, err := os.ReadFile(filepath.Join(modulePath, rel))
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after))
}

// An object the generator wrote earlier and the declaration no longer names leaves the file with
// the regeneration and is named in the finding, even when the file holds other objects (review of
// #479, finding 4): dropping the legacy Admin level rewrites the legacy file.
func TestSync_DroppedLegacyLevelIsRemoved(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)
	store := renderedFrom(t, model, nil)

	decl, err := rbacyaml.Load(modulePath)
	require.NoError(t, err)

	for i := range decl.Resources {
		delete(decl.Resources[i].Legacy, "Admin")
	}

	raw, err := yaml.Marshal(decl)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), raw, 0o600))

	const rel = "templates/user-authz-cluster-roles.yaml"

	errorList := runSync(t, modulePath, store)
	got := texts(errorList)
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], "ClusterRole/d8:user-authz:cert-manager:admin")

	for _, fix := range errorList.GetFixes() {
		fix()
	}

	assert.Empty(t, errorList.GetErrors(), "the fix applies")

	written, err := os.ReadFile(filepath.Join(modulePath, rel))
	require.NoError(t, err)
	assert.NotContains(t, string(written), "d8:user-authz:cert-manager:admin")
	assert.Contains(t, string(written), "d8:user-authz:cert-manager:user")
}

// A hand-written role with the same rules as a produced one is a duplicate, not an old name, when
// the produced one is rendered too (review of #479, finding 3): the fix refuses instead of dropping it.
func TestSync_DuplicateOfARenderedObjectIsNotARename(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	const rel = "templates/rbac-to-us.yaml"

	asContractOne(t, filepath.Join(modulePath, rel))

	store := renderedFrom(t, model, nil)

	var produced generate.Object

	for _, o := range model.File(rel).Objects {
		if o.Kind == "Role" {
			produced = o
		}
	}

	duplicate := produced
	duplicate.Name = "extra-reader-bound-elsewhere"
	putObject(t, store, rel, duplicate)

	errorList := runSync(t, modulePath, store)
	for _, fix := range errorList.GetFixes() {
		fix()
	}

	remaining := errorList.GetErrors()
	require.Len(t, remaining, 1, "got: %v", texts(errorList))
	require.Error(t, remaining[0].FixError)
	assert.Contains(t, remaining[0].FixError.Error(), "d8-cert-manager/Role/extra-reader-bound-elsewhere")
}

func putConfigMap(t *testing.T, store *storage.UnstructuredObjectStore, path, name string) {
	t.Helper()

	cm := &corev1.ConfigMap{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "d8-cert-manager", Labels: map[string]string{"heritage": "deckhouse", "module": syncModule}}}
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
	require.NoError(t, err)
	require.NoError(t, store.Put("/module/"+path, path, content, []byte("d8-cert-manager/ConfigMap/"+name)))
}

// The access level of a legacy role and an aggregationRule on a declared role are compared too
// (review of #479, finding 5): a render that differs there is a divergence, not silence.
func TestSync_AccessLevelAndAggregationAreCompared(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	store := renderedFrom(t, model, func(o *generate.Object) bool {
		if o.Name == "d8:user-authz:cert-manager:user" {
			o.Annotations = map[string]string{rbaccontract.AccessLevelAnnotation: "SuperAdmin"}
		}

		return true
	})

	got := texts(runSync(t, modulePath, store))
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], `ClusterRole/d8:user-authz:cert-manager:user: the user-authz.deckhouse.io/access-level annotation is "SuperAdmin" in the render, the declaration produces "User"`)
}

// A written declaration with TODO in it keeps the bootstrap finding and the non-zero exit: the
// file exists, the decisions do not (review of #479, finding 9).
func TestSync_BootstrapWithOpenDecisionsKeepsTheFinding(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	require.NoError(t, os.Remove(rbacyaml.Path(modulePath)))
	require.NoError(t, os.WriteFile(filepath.Join(modulePath, "crds", "extra.yaml"), []byte(crdYAML("cert-manager.io", "nobodies", "Namespaced")), 0o600))

	errorList := runSync(t, modulePath, renderedFrom(t, model, nil))
	for _, fix := range errorList.GetFixes() {
		fix()
	}

	remaining := errorList.GetErrors()
	require.Len(t, remaining, 1)
	require.Error(t, remaining[0].FixError)
	assert.Contains(t, remaining[0].FixError.Error(), "rbac.yaml is written; 1 TODO in it are decisions only a person can make")

	_, err := os.Stat(rbacyaml.Path(modulePath))
	require.NoError(t, err, "the file is written all the same")
}

// A module directory in an edition overlay gets no declaration of its own (review of #479,
// finding 9): nothing is reported and nothing is written.
func TestSync_NoBootstrapInAnOverlay(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	root := t.TempDir()
	base := filepath.Join(root, "modules", "101-cert-manager")
	overlay := filepath.Join(root, "ee", "se-plus", "modules", "101-cert-manager")

	require.NoError(t, os.MkdirAll(base, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(overlay), 0o755))

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	require.NoError(t, os.Remove(rbacyaml.Path(modulePath)))
	require.NoError(t, os.Rename(modulePath, overlay))

	errorList := runSync(t, overlay, renderedFrom(t, model, nil))
	assert.Empty(t, texts(errorList))

	_, err := os.Stat(rbacyaml.Path(overlay))
	assert.True(t, os.IsNotExist(err))
}

// A template the tolerant render skipped is neither compared nor regenerated: its objects were
// never seen (review of #479, finding 13k).
func TestSync_DroppedTemplateIsNotRegenerated(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	const rel = "templates/rbac-to-us.yaml"

	// The render skipped the whole template, so none of its objects is in the storage.
	dropped := map[string]struct{}{}
	for _, o := range model.File(rel).Objects {
		dropped[o.Identity()] = struct{}{}
	}

	store := renderedFrom(t, model, func(o *generate.Object) bool {
		_, gone := dropped[o.Identity()]

		return !gone
	})
	store.MarkDropped(rel, "required value missing")

	errorList := runSync(t, modulePath, store)
	got := texts(errorList)
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], rel+" failed to render in this run (required value missing); nothing in it is compared or regenerated")
	assert.Empty(t, errorList.GetFixes())
}

// A module.yaml that does not parse stops the rule instead of generating without subsystems
// (review of #479, finding 13k).
func TestSync_BrokenModuleYAMLStops(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	require.NoError(t, os.WriteFile(filepath.Join(modulePath, "module.yaml"), []byte("name: [broken\n"), 0o600))

	errorList := runSync(t, modulePath, renderedFrom(t, model, nil))
	got := texts(errorList)
	require.Len(t, got, 1, "got: %v", got)
	assert.Contains(t, got[0], "parse module.yaml")
	assert.Empty(t, errorList.GetFixes())
}

// A generated file whose objects are all under a false condition is found on disk and deleted
// when the declaration drops them (review of #479, finding 16).
func TestSync_OrphanWithEveryObjectUnderWhenIsFound(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	const rel = "templates/cainjector/rbac-for-us.yaml"

	// Default values: the cainjector account (under when) does not render.
	store := renderedFrom(t, model, func(o *generate.Object) bool { return o.When == "" })

	decl, err := rbacyaml.Load(modulePath)
	require.NoError(t, err)

	decl.ServiceAccounts = nil
	raw, err := yaml.Marshal(decl)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), raw, 0o600))

	errorList := runSync(t, modulePath, store)
	assert.Contains(t, strings.Join(texts(errorList), "\n"), rel+" does not match rbac.yaml")

	for _, fix := range errorList.GetFixes() {
		fix()
	}

	_, err = os.Stat(filepath.Join(modulePath, rel))
	assert.True(t, os.IsNotExist(err), "the orphan is deleted")
}

// A template one render variant could not render is not rewritten by another variant's fix
// (review of #479, finding 17).
func TestSync_TemplateDroppedInAnotherVariantIsNotRewritten(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	const rel = "templates/rbac-to-us.yaml"

	fullPath := filepath.Join(modulePath, rel)
	require.NoError(t, os.WriteFile(fullPath, []byte(generate.Header()+"\n# stale\n"), 0o600))

	// The variant that could not render it.
	failing := renderedFrom(t, model, nil)
	failing.MarkDropped(rel, "required value missing")
	runSync(t, modulePath, failing)

	// The variant that renders it asks for a regeneration.
	errorList := runSync(t, modulePath, renderedFrom(t, model, nil))
	for _, fix := range errorList.GetFixes() {
		fix()
	}

	var messages []string

	for _, e := range errorList.GetErrors() {
		if e.FixError != nil {
			messages = append(messages, e.FixError.Error())
		}
	}

	assert.Contains(t, strings.Join(messages, "\n"), rel+" failed to render under some values (required value missing)")

	kept, err := os.ReadFile(fullPath)
	require.NoError(t, err)
	assert.Equal(t, generate.Header()+"\n# stale\n", string(kept))
}

// A contract 1 file lists no owned objects, and a name is no proof of ownership (review of #479,
// finding 26): an object the declaration dropped is refused there. One --fix with the declaration
// unchanged brings the file to contract 2; then the drop applies.
func TestSync_ContractOneNeedsAnUpgradeBeforeADrop(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)
	store := renderedFrom(t, model, nil)

	const rel = "templates/rbac-to-us.yaml"

	path := filepath.Join(modulePath, rel)
	asContractOne(t, path)

	dropPrometheus := func() {
		decl, err := rbacyaml.Load(modulePath)
		require.NoError(t, err)

		decl.PrometheusAccess = nil
		raw, err := yaml.Marshal(decl)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), raw, 0o600))
	}

	original, err := os.ReadFile(rbacyaml.Path(modulePath))
	require.NoError(t, err)

	t.Run("dropped in the same run: refused", func(t *testing.T) {
		resetFixState()
		dropPrometheus()

		errorList := runSync(t, modulePath, store)
		for _, fix := range errorList.GetFixes() {
			fix()
		}

		var messages []string

		for _, e := range errorList.GetErrors() {
			if e.FixError != nil {
				messages = append(messages, e.FixError.Error())
			}
		}

		assert.Contains(t, strings.Join(messages, "\n"), "d8-cert-manager/Role/access-to-cert-manager")
	})

	t.Run("upgraded first, dropped next: removed", func(t *testing.T) {
		resetFixState()
		require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), original, 0o600))
		asContractOne(t, path)

		upgrade := runSync(t, modulePath, store)
		for _, fix := range upgrade.GetFixes() {
			fix()
		}

		require.Empty(t, upgrade.GetErrors(), "the upgrade to contract 2 applies")

		resetFixState()
		dropPrometheus()

		errorList := runSync(t, modulePath, store)
		for _, fix := range errorList.GetFixes() {
			fix()
		}

		assert.Empty(t, errorList.GetErrors())

		if written, err := os.ReadFile(path); err == nil {
			assert.NotContains(t, string(written), "name: access-to-cert-manager\n")
		}
	})
}

// A generated file found only on disk -- nothing in it rendered -- is not deleted when an object it
// holds is now declared elsewhere, nor when it holds an object the generator did not write
// (review of #479, findings 23 and 24).
func TestSync_UnrenderedOrphanIsJudgedByItsText(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	const rel = "templates/cainjector/rbac-for-us.yaml"

	fullPath := filepath.Join(modulePath, rel)

	// Default values: the cainjector account (under when) does not render.
	store := renderedFrom(t, model, func(o *generate.Object) bool { return o.When == "" })

	decl, err := rbacyaml.Load(modulePath)
	require.NoError(t, err)

	t.Run("the account moves to the root rbac-for-us.yaml: refused", func(t *testing.T) {
		resetFixState()

		moved := *decl
		moved.ServiceAccounts = append([]rbacyaml.ServiceAccount(nil), decl.ServiceAccounts...)
		moved.ServiceAccounts[0].Path = ""
		raw, err := yaml.Marshal(&moved)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), raw, 0o600))

		errorList := runSync(t, modulePath, store)
		for _, fix := range errorList.GetFixes() {
			fix()
		}

		_, err = os.Stat(fullPath)
		require.NoError(t, err, "the file with the account stays")
	})

	t.Run("a hand-added ConfigMap under the same condition: refused", func(t *testing.T) {
		resetFixState()

		dropped := *decl
		dropped.ServiceAccounts = nil
		raw, err := yaml.Marshal(&dropped)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), raw, 0o600))

		content, err := os.ReadFile(fullPath)
		require.NoError(t, err)

		withHand := strings.Replace(string(content), "{{- end }}\n", "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cainjector-extra\n  namespace: d8-cert-manager\n{{- end }}\n", 1)
		require.NoError(t, os.WriteFile(fullPath, []byte(withHand), 0o600))

		errorList := runSync(t, modulePath, store)
		for _, fix := range errorList.GetFixes() {
			fix()
		}

		var messages []string

		for _, e := range errorList.GetErrors() {
			if e.FixError != nil {
				messages = append(messages, e.FixError.Error())
			}
		}

		assert.Contains(t, strings.Join(messages, "\n"), "d8-cert-manager/ConfigMap/cainjector-extra")

		kept, err := os.ReadFile(fullPath)
		require.NoError(t, err)
		assert.Equal(t, withHand, string(kept))
	})
}

// The disk scan skips the generator's _<file>.generated asides (review of #479, finding 27).
func TestGeneratedTemplates_SkipsAsides(t *testing.T) {
	modulePath := t.TempDir()
	dir := filepath.Join(modulePath, "templates", "rbacv2", "use")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "view.yaml"), []byte(generate.Header()+"\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "_edit.yaml.generated"), []byte(generate.Header()+"\n"), 0o600))

	assert.Equal(t, []string{"templates/rbacv2/use/view.yaml"}, generatedTemplates(modulePath))
}

// A hand-added object named the way the generator names things, in a contract 1 file, is refused
// rather than removed (review of #479, finding 26).
func TestSync_ContractOneHandAddedGeneratorNamedIsForeign(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	const rel = "templates/cainjector/rbac-for-us.yaml"

	asContractOne(t, filepath.Join(modulePath, rel))

	store := renderedFrom(t, model, nil)
	putObject(t, store, rel, generate.Object{Kind: "ClusterRole", Name: "d8:cert-manager:hand-extra", Class: generate.ClassDeclared,
		Rules: []generate.Rule{{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}}}}})

	errorList := runSync(t, modulePath, store)
	for _, fix := range errorList.GetFixes() {
		fix()
	}

	var messages []string

	for _, e := range errorList.GetErrors() {
		if e.FixError != nil {
			messages = append(messages, e.FixError.Error())
		}
	}

	assert.Contains(t, strings.Join(messages, "\n"), "ClusterRole/d8:cert-manager:hand-extra")
}
