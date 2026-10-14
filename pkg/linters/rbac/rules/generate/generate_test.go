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

package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbaccontract"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbacyaml"
)

// updateGolden rewrites the expected files from the current output: UPDATE_GOLDEN=1 go test ./...
// Review the diff before committing it -- the golden files are the contract of the generator.
var updateGolden = os.Getenv("UPDATE_GOLDEN") == "1"

func certManagerInput(t *testing.T) Input {
	t.Helper()

	decl, err := rbacyaml.Load("testdata/cert-manager")
	require.NoError(t, err)
	require.Empty(t, rbacyaml.Validate(decl, rbacyaml.CRDScopes{
		"cert-manager.io/certificates":        rbacyaml.ScopeNamespaced,
		"cert-manager.io/certificaterequests": rbacyaml.ScopeNamespaced,
		"cert-manager.io/issuers":             rbacyaml.ScopeNamespaced,
		"cert-manager.io/clusterissuers":      rbacyaml.ScopeCluster,
		"acme.cert-manager.io/orders":         rbacyaml.ScopeNamespaced,
		"acme.cert-manager.io/challenges":     rbacyaml.ScopeNamespaced,
	}))

	return Input{Module: "cert-manager", Namespace: "d8-cert-manager", ChartName: "cert-manager", Subsystems: []string{"security"}, Decl: decl}
}

func TestBuild_CertManagerModel(t *testing.T) {
	model, err := Build(certManagerInput(t))
	require.NoError(t, err)

	assert.Equal(t, []string{
		"templates/cainjector/rbac-for-us.yaml",
		"templates/rbac-for-us.yaml",
		"templates/rbac-to-us.yaml",
		"templates/rbacv2/manage/edit.yaml",
		"templates/rbacv2/manage/view.yaml",
		"templates/rbacv2/use/admin.yaml",
		"templates/rbacv2/use/edit.yaml",
		"templates/rbacv2/use/view.yaml",
		"templates/user-authz-cluster-roles.yaml",
	}, model.Paths())

	// The system view capability always carries the ModuleConfig rule, aggregates into every
	// subsystem of module.yaml and names the module namespace.
	view := model.File("templates/rbacv2/manage/view.yaml").Objects[0]
	assert.Equal(t, "d8:system-capability:cert-manager:view", view.Name)
	assert.Equal(t, []LineageLevel{{Lineage: "security", Level: "viewer"}}, view.AggregationLabels())
	assert.Equal(t, "d8-cert-manager", view.Labels["rbac.deckhouse.io/namespace"])
	assert.Equal(t, "system-capability.cert-manager.view", view.Labels["rbac.deckhouse.io/capability"])
	require.Len(t, view.Rules, 2)
	assert.Equal(t, []string{"clusterissuers"}, view.Rules[0].Resources)
	assert.Equal(t, []string{"moduleconfigs"}, view.Rules[1].Resources)
	assert.Equal(t, []string{"cert-manager"}, view.Rules[1].ResourceNames)
	assert.Equal(t, "Module cert-manager: view configuration", view.Annotations["en.meta.deckhouse.io/title"])

	// The admin capability takes its texts from the declaration.
	admin := model.File("templates/rbacv2/use/admin.yaml").Objects[0]
	assert.Equal(t, "Модуль cert-manager: администрирование", admin.Annotations["ru.meta.deckhouse.io/title"])
	assert.Equal(t, []LineageLevel{{Lineage: "namespace", Level: "admin"}}, admin.AggregationLabels())

	// A rule under when keeps its condition; the rest of the file does not.
	useView := model.File("templates/rbacv2/use/view.yaml").Objects[0]

	conditional := make([]string, 0, 1)

	for _, r := range useView.Rules {
		if r.When != "" {
			conditional = append(conditional, r.Resources[0]+"@"+r.When)
		}
	}

	assert.Equal(t, []string{"challenges@.Values.certManager.internal.acmeEnabled"}, conditional)

	// Legacy roles: one per level in enum order, kebab-case names.
	legacy := model.File("templates/user-authz-cluster-roles.yaml")

	names := make([]string, 0, len(legacy.Objects))
	for _, o := range legacy.Objects {
		names = append(names, o.Name+"="+o.Annotations["user-authz.deckhouse.io/access-level"])
	}

	assert.Equal(t, []string{
		"d8:user-authz:cert-manager:user=User",
		"d8:user-authz:cert-manager:editor=Editor",
		"d8:user-authz:cert-manager:admin=Admin",
		"d8:user-authz:cert-manager:cluster-editor=ClusterEditor",
	}, names)

	// The ServiceAccount file: every object under the account's condition, names as the placement
	// rule expects, the foreign-namespace binding in its namespace.
	sa := model.File("templates/cainjector/rbac-for-us.yaml")

	ids := make([]string, 0, len(sa.Objects))
	for _, o := range sa.Objects {
		ids = append(ids, o.Identity()+"@"+o.When)
	}

	assert.Equal(t, []string{
		"d8-cert-manager/ServiceAccount/cainjector@.Values.certManager.internal.enableCAInjector",
		"ClusterRole/d8:cert-manager:cainjector@.Values.certManager.internal.enableCAInjector",
		"ClusterRoleBinding/d8:cert-manager:cainjector@.Values.certManager.internal.enableCAInjector",
		"d8-cert-manager/Role/cainjector@.Values.certManager.internal.enableCAInjector",
		"d8-cert-manager/RoleBinding/cainjector@.Values.certManager.internal.enableCAInjector",
		"ClusterRoleBinding/d8:cert-manager:cainjector:rbac-proxy@.Values.certManager.internal.enableCAInjector",
		"kube-system/RoleBinding/d8:cert-manager:cainjector:extension-apiserver-authentication-reader@.Values.certManager.internal.enableCAInjector",
	}, ids)

	// Access: cluster rules land in rbac-for-us.yaml, namespace rules and the metrics access in rbac-to-us.yaml.
	forUs := make([]string, 0, 2)
	for _, o := range model.File("templates/rbac-for-us.yaml").Objects {
		forUs = append(forUs, o.Identity())
	}

	toUs := make([]string, 0, 4)
	for _, o := range model.File("templates/rbac-to-us.yaml").Objects {
		toUs = append(toUs, o.Identity())
	}

	assert.Equal(t, []string{"ClusterRole/d8:cert-manager:admin-kubeconfig", "ClusterRoleBinding/d8:cert-manager:admin-kubeconfig"}, forUs)
	assert.Equal(t, []string{
		"d8-cert-manager/Role/access-to-cert-manager", "d8-cert-manager/RoleBinding/access-to-cert-manager",
		"d8-cert-manager/Role/access-to-cert-manager-auth", "d8-cert-manager/RoleBinding/access-to-cert-manager-auth",
	}, toUs)
}

// The system capabilities aggregate into the subsystems of module.yaml only: a subsystems list the
// declaration still carries is not read (review of #480, finding 13).
func TestBuild_SubsystemsOfModuleYAMLAndNamespaceLabel(t *testing.T) {
	in := certManagerInput(t)
	in.Subsystems = []string{"network", "managed-services"}
	in.Decl.Subsystems = []string{"storage"}
	in.Namespace = "default"
	// Accounts of component directories cannot live in default (README, limits).
	in.Decl.ServiceAccounts = nil
	in.Decl.Normalize()

	model, err := Build(in)
	require.NoError(t, err)

	edit := model.File("templates/rbacv2/manage/edit.yaml").Objects[0]
	assert.Equal(t, []LineageLevel{{Lineage: "managed-services", Level: "manager"}, {Lineage: "network", Level: "manager"}}, edit.AggregationLabels())
	_, hasNamespaceLabel := edit.Labels["rbac.deckhouse.io/namespace"]
	assert.False(t, hasNamespaceLabel, "the namespace label is set only for a d8- namespace")
	assert.Equal(t, []string{"create", "delete", "patch", "update"}, edit.Rules[len(edit.Rules)-1].Verbs, "the edit ModuleConfig rule has no read verbs")
}

// observerInput is the fixture of the names a module keeps: prometheusAccess as a list with path and
// name, a bindRoles entry with the binding name the module gave it.
func observerInput(t *testing.T) Input {
	t.Helper()

	decl, err := rbacyaml.Load(filepath.Join("testdata", "observer"))
	require.NoError(t, err)
	require.Empty(t, rbacyaml.Validate(decl, nil))

	return Input{Module: "observer", Namespace: "d8-observer", ChartName: "observer", Decl: decl}
}

func TestRender_GoldenAndIdempotent(t *testing.T) {
	for fixture, input := range map[string]func(*testing.T) Input{"cert-manager": certManagerInput, "observer": observerInput} {
		t.Run(fixture, func(t *testing.T) {
			model, err := Build(input(t))
			require.NoError(t, err)

			first := Render(model)
			second := Render(model)
			assert.Equal(t, first, second, "rendering is a pure function of the model")

			paths := make([]string, 0, len(first))

			for _, f := range first {
				paths = append(paths, f.Path)
				golden := filepath.Join("testdata", fixture, "expected", f.Path)

				if updateGolden {
					require.NoError(t, os.MkdirAll(filepath.Dir(golden), 0o755))
					require.NoError(t, os.WriteFile(golden, []byte(f.Content), 0o600))
				}

				want, err := os.ReadFile(golden)
				require.NoError(t, err, "missing golden file %s (run with UPDATE_GOLDEN=1)", golden)
				assert.Equal(t, string(want), f.Content, "generated %s differs from the golden file", f.Path)
				assert.NotContains(t, f.Content, "Generated by dmt", "the templates carry no header")
			}

			// A golden file the generator no longer writes is a stale contract.
			var stale []string

			require.NoError(t, filepath.WalkDir(filepath.Join("testdata", fixture, "expected"), func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}

				rel, err := filepath.Rel(filepath.Join("testdata", fixture, "expected"), path)
				if err == nil && !slices.Contains(paths, filepath.ToSlash(rel)) {
					stale = append(stale, rel)
				}

				return err
			}))
			assert.Empty(t, stale, "golden files the generator does not write")
		})
	}
}

// The module namespace is written as d8-{{ $.Chart.Name }}, as the hand-written templates write it,
// when the chart is named after the module and the module lives in d8-<module>: in metadata.namespace
// and in a ServiceAccount subject. Every other namespace, and every namespace of a chart named
// otherwise, is written as it is.
func TestRender_ChartNamespace(t *testing.T) {
	render := func(t *testing.T, in Input) string {
		t.Helper()

		model, err := Build(in)
		require.NoError(t, err)

		var all strings.Builder
		for _, f := range Render(model) {
			all.WriteString(f.Content)
		}

		return all.String()
	}

	t.Run("the chart is named after the module", func(t *testing.T) {
		text := render(t, observerInput(t))

		assert.NotContains(t, text, "d8-observer", "the module namespace is never written literally")
		assert.Contains(t, text, "  name: controller\n  namespace: d8-{{ $.Chart.Name }}\n", "the account")
		assert.Contains(t, text, "- kind: ServiceAccount\n  name: controller\n  namespace: d8-{{ $.Chart.Name }}\n", "the account as a subject")
		assert.Contains(t, text, "- kind: ServiceAccount\n  name: reader\n  namespace: d8-{{ $.Chart.Name }}\n", "an access subject of the module namespace")
		assert.Contains(t, text, "  namespace: d8-log-shipper\n", "a namespace of the platform stays literal")
		assert.Contains(t, text, "  namespace: kube-system\n")
		assert.Contains(t, text, "- kind: ServiceAccount\n  name: grafana\n  namespace: d8-monitoring\n")
		assert.Contains(t, text, "- kind: ServiceAccount\n  name: prometheus\n  namespace: d8-monitoring\n")
	})

	for name, tweak := range map[string]func(in *Input){
		"the chart is named otherwise":          func(in *Input) { in.ChartName = "observer-chart" },
		"the chart name is not known":           func(in *Input) { in.ChartName = "" },
		"the module lives in another namespace": func(in *Input) { in.Namespace = "d8-system" },
	} {
		t.Run(name, func(t *testing.T) {
			in := observerInput(t)
			tweak(&in)

			text := render(t, in)
			assert.NotContains(t, text, "{{ $.Chart.Name }}")
			assert.Contains(t, text, "  namespace: "+in.Namespace+"\n")
		})
	}
}

// prometheusAccess entries keep the names and directories of the scrape access: access-to-<module>
// at the root without path and name, access-to-<module>-<name> at the root with a name,
// access-to-<directory>-<name> in templates/<directory>/rbac-to-us.yaml with both. Each Role is
// unconditional and each binding under its entry's when.
func TestBuild_PrometheusAccessEntries(t *testing.T) {
	model, err := Build(observerInput(t))
	require.NoError(t, err)

	ids := func(path string) []string {
		f := model.File(path)
		require.NotNil(t, f, path)

		out := make([]string, 0, len(f.Objects))
		for _, o := range f.Objects {
			out = append(out, o.Identity()+"@"+o.When)
		}

		return out
	}

	gate := `.Values.global.enabledModules | has "prometheus"`

	assert.Equal(t, []string{
		"d8-observer/Role/access-to-observer@",
		"d8-observer/RoleBinding/access-to-observer@" + gate,
		"d8-observer/Role/access-to-observer-metrics@",
		"d8-observer/RoleBinding/access-to-observer-metrics@",
		"d8-observer/Role/access-to-observer-http@",
		"d8-observer/RoleBinding/access-to-observer-http@",
	}, ids("templates/rbac-to-us.yaml"))

	assert.Equal(t, []string{
		"d8-observer/Role/access-to-dex-prometheus-metrics@",
		"d8-observer/RoleBinding/access-to-dex-prometheus-metrics@(" + gate + ")",
		"d8-observer/Role/access-to-dex-configmap-reader@",
		"d8-observer/RoleBinding/access-to-dex-configmap-reader-binding@",
	}, ids("templates/dex/rbac-to-us.yaml"), "the scrape access comes before the access entries of the file")

	assert.Equal(t, []string{
		"d8-observer/Role/access-to-cloud-data-discoverer-role@",
		"d8-observer/RoleBinding/access-to-cloud-data-discoverer-binding@",
	}, ids("templates/cloud-data-discoverer/rbac-to-us.yaml"), "bindingName names the binding")
	assert.Equal(t, "access-to-cloud-data-discoverer-role", model.File("templates/cloud-data-discoverer/rbac-to-us.yaml").Objects[1].RoleRefName)

	metrics := model.File("templates/rbac-to-us.yaml").Objects[2]
	assert.Equal(t, []string{"daemonsets/prometheus-metrics", "statefulsets/prometheus-metrics"}, []string{metrics.Rules[0].Resources[0], metrics.Rules[1].Resources[0]})

	dex := model.File("templates/dex/rbac-to-us.yaml").Objects[0]
	assert.Equal(t, map[string]string{"app": "dex"}, dex.Labels)

	// The order of the entries in the declaration does not change the files.
	in := observerInput(t)
	slices.Reverse(in.Decl.PrometheusAccess)

	reversed, err := Build(in)
	require.NoError(t, err)
	assert.Equal(t, Render(model), Render(reversed))
}

// access[] objects take roleName and bindingName as they are, the binding the role's name by default,
// and a name that already carries the prefix of its file is not prefixed again.
func TestBuild_AccessObjectNames(t *testing.T) {
	model, err := Build(observerInput(t))
	require.NoError(t, err)

	refs := func(path string) []string {
		f := model.File(path)
		require.NotNil(t, f, path)

		out := make([]string, 0, len(f.Objects))

		for _, o := range f.Objects {
			if strings.Contains(o.Name, "prometheus") || o.Kind == "ServiceAccount" || o.Name == "access-to-observer-metrics" || f.Path == "templates/controller/rbac-for-us.yaml" {
				continue
			}

			id := o.Identity()
			if o.RoleRefName != "" {
				id += " -> " + o.RoleRefName
			}

			out = append(out, id)
		}

		return out
	}

	assert.Equal(t, []string{
		"d8-observer/Role/access-to-observer",
		"d8-observer/RoleBinding/access-to-observer -> access-to-observer",
		"d8-observer/Role/access-to-observer-http",
		"d8-observer/RoleBinding/access-to-observer-http -> access-to-observer-http",
	}, refs("templates/rbac-to-us.yaml"), "the scrape access comes first; the access name is not prefixed again")
	assert.Equal(t, []string{
		"ClusterRole/d8:observer:accessible-namespaces-reader",
		"ClusterRoleBinding/d8:observer:accessible-namespaces-reader:system-authenticated -> d8:observer:accessible-namespaces-reader",
	}, refs("templates/rbac-for-us.yaml"))
	assert.Equal(t, []string{
		"d8-observer/Role/access-to-dex-configmap-reader",
		"d8-observer/RoleBinding/access-to-dex-configmap-reader-binding -> access-to-dex-configmap-reader",
	}, refs("templates/dex/rbac-to-us.yaml"))
}

// The first declarations wrote prometheusAccess as one mapping (modules/101-cert-manager/rbac.yaml in
// deckhouse): it reads as a list of one entry and produces the objects it always did.
func TestBuild_PrometheusAccessMappingForm(t *testing.T) {
	const head = "apiVersion: rbac.deckhouse.io/v1alpha1\n"

	mapping, err := rbacyaml.Parse([]byte(head + "prometheusAccess:\n  deployments: [cert-manager]\n  when: .Values.global.enabledModules | has \"prometheus\"\n"))
	require.NoError(t, err)

	list, err := rbacyaml.Parse([]byte(head + "prometheusAccess:\n  - deployments: [cert-manager]\n    when: .Values.global.enabledModules | has \"prometheus\"\n"))
	require.NoError(t, err)

	assert.Equal(t, mapping, list)

	model, err := Build(Input{Module: "cert-manager", Namespace: "d8-cert-manager", ChartName: "cert-manager", Decl: mapping})
	require.NoError(t, err)

	f := model.File("templates/rbac-to-us.yaml")
	require.NotNil(t, f)
	require.Len(t, f.Objects, 2)
	assert.Equal(t, "d8-cert-manager/Role/access-to-cert-manager", f.Objects[0].Identity())
	assert.Equal(t, "d8-cert-manager/RoleBinding/access-to-cert-manager", f.Objects[1].Identity())
	assert.Equal(t, `.Values.global.enabledModules | has "prometheus"`, f.Objects[1].When)
}

// A bindRoles entry with a binding name gets the binding under that name; without one, the name the
// placement rule wants.
func TestBuild_BindRolesBindingName(t *testing.T) {
	model, err := Build(observerInput(t))
	require.NoError(t, err)

	f := model.File("templates/controller/rbac-for-us.yaml")
	require.NotNil(t, f)

	var bindings []string

	for _, o := range f.Objects {
		if o.Kind == "RoleBinding" && o.Namespace != "d8-observer" {
			bindings = append(bindings, o.Identity()+" -> "+o.RoleRefName)
		}
	}

	assert.Equal(t, []string{
		"d8-log-shipper/RoleBinding/d8:observer:controller:log-shipper-secrets -> d8:observer:controller:log-shipper-secrets",
		"kube-system/RoleBinding/d8:observer:controller:extension-apiserver-authentication-reader -> extension-apiserver-authentication-reader",
	}, bindings)
}

// extraClusterRoles land in the account's file, bound unless said otherwise; the account's
// automountServiceAccountToken follows the declaration and defaults to false.
func TestBuild_ExtraClusterRolesAndAutomount(t *testing.T) {
	yes, no := true, false
	decl := &rbacyaml.Declaration{APIVersion: rbacyaml.APIVersionV1Alpha1, ServiceAccounts: []rbacyaml.ServiceAccount{
		{Name: "webhook", Path: "webhook", AutomountToken: &yes, ExtraClusterRoles: []rbacyaml.ExtraClusterRole{
			{Name: "requester", Bind: &no, Rules: []rbacyaml.PolicyRule{{APIGroups: []string{"admission.cert-manager.io"}, Resources: []string{"certificates"}, Verbs: []string{"create"}}}},
			{Name: "approve", Rules: []rbacyaml.PolicyRule{{APIGroups: []string{"cert-manager.io"}, Resources: []string{"signers"}, Verbs: []string{"approve"}}}},
			{Name: "d8:cert-manager:legacy-name", Rules: []rbacyaml.PolicyRule{{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}}}},
		}},
		{Name: "controller"},
	}}

	model, err := Build(Input{Module: "cert-manager", Namespace: "d8-cert-manager", Subsystems: []string{"security"}, Decl: decl})
	require.NoError(t, err)

	file := model.File("templates/webhook/rbac-for-us.yaml")
	require.NotNil(t, file)

	names := map[string]Object{}
	for _, o := range file.Objects {
		names[o.Kind+"/"+o.Name] = o
	}

	assert.Contains(t, names, "ClusterRole/d8:cert-manager:webhook:requester")
	assert.NotContains(t, names, "ClusterRoleBinding/d8:cert-manager:webhook:requester", "bind: false leaves the role unbound")
	assert.Contains(t, names, "ClusterRole/d8:cert-manager:webhook:approve")
	assert.Contains(t, names, "ClusterRoleBinding/d8:cert-manager:webhook:approve")
	assert.Equal(t, "d8:cert-manager:webhook:approve", names["ClusterRoleBinding/d8:cert-manager:webhook:approve"].RoleRefName)
	assert.Contains(t, names, "ClusterRole/d8:cert-manager:legacy-name", "a full d8: name is kept as given")
	assert.True(t, *names["ServiceAccount/webhook"].AutomountToken)

	root := model.File("templates/rbac-for-us.yaml")
	require.NotNil(t, root)

	for _, o := range root.Objects {
		if o.Kind == "ServiceAccount" && o.Name == "controller" {
			assert.False(t, *o.AutomountToken, "unset means false")
		}
	}
}

// What the declaration alone cannot know is wrong, the generator refuses against the module.
func TestBuild_RefusesWhatTheModuleCannotCarry(t *testing.T) {
	base := func() *rbacyaml.Declaration {
		return &rbacyaml.Declaration{APIVersion: rbacyaml.APIVersionV1Alpha1, Resources: []rbacyaml.Resource{
			{Group: "x.io", Resource: "things", Scope: "Cluster", System: map[string][]string{"viewer": {"get"}}},
		}}
	}

	t.Run("system levels without any subsystem", func(t *testing.T) {
		_, err := Build(Input{Module: "m", Namespace: "d8-m", Decl: base()})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "system levels are declared but the module aggregates into no subsystem")

		_, err = Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Decl: base()})
		require.NoError(t, err, "module.yaml subsystems suffice")

		decl := base()
		decl.Subsystems = []string{"storage"}
		_, err = Build(Input{Module: "m", Namespace: "d8-m", Decl: decl})
		require.Error(t, err, "a subsystems list of the declaration is not read")
		assert.Contains(t, err.Error(), "module.yaml declares none, so declare the module's subsystems in module.yaml")
	})

	t.Run("an account whose name does not follow its directory", func(t *testing.T) {
		decl := base()
		decl.ServiceAccounts = []rbacyaml.ServiceAccount{{Name: "helper", Path: "cainjector"}}
		_, err := Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Decl: decl})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `the placement rule wants the account named "cainjector" after its directory`)

		decl.ServiceAccounts = []rbacyaml.ServiceAccount{{Name: "cainjector", Path: "cainjector"}, {Name: "webhook", Path: "webhook"}, {Name: "anything", Path: ""}}
		_, err = Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Decl: decl})
		require.NoError(t, err)

		// The module name in front is the platform namespaces' form only (regression hunt 2, A1).
		decl.ServiceAccounts = []rbacyaml.ServiceAccount{{Name: "m-cainjector", Path: "cainjector"}}
		_, err = Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Decl: decl})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `only in a namespace of the platform`)

		_, err = Build(Input{Module: "m", Namespace: "d8-system", Subsystems: []string{"security"}, Decl: decl})
		require.NoError(t, err)

		decl.ServiceAccounts = []rbacyaml.ServiceAccount{{Name: "dir", Path: "some/nested/dir"}}
		_, err = Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Decl: decl})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `wants the account named "some-nested-dir"`)

		// templates/<a>/<b>/rbac-for-us.yaml: the placement rule joins the directories.
		decl.ServiceAccounts = []rbacyaml.ServiceAccount{{Name: "some-nested-dir", Path: "some/nested/dir"}}
		_, err = Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Decl: decl})
		require.NoError(t, err)

		// In default and kube-system the placement rule wants d8-<module>-<dir>, which the generator
		// does not accept: refused with the limitation named (review of #479, finding 43).
		decl.ServiceAccounts = []rbacyaml.ServiceAccount{{Name: "control-plane-proxy", Path: "control-plane-proxy"}}
		for _, ns := range []string{"kube-system", "default"} {
			_, err = Build(Input{Module: "m", Namespace: ns, Subsystems: []string{"security"}, Decl: decl})
			require.Error(t, err, ns)
			assert.Contains(t, err.Error(), `the placement rule wants the account named "d8-m-control-plane-proxy", which the declaration does not accept yet (a known limitation)`)
		}
	})

	t.Run("namespace access in a directory", func(t *testing.T) {
		decl := base()
		decl.Access = []rbacyaml.Access{{Name: "reader", Path: "cainjector", Subjects: []rbacyaml.Subject{{Kind: "Group", Name: "g"}},
			NamespaceRules: []rbacyaml.PolicyRule{{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}}}}}
		model, err := Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Decl: decl})
		require.NoError(t, err)

		file := model.File("templates/cainjector/rbac-to-us.yaml")
		require.NotNil(t, file)
		assert.Equal(t, "access-to-cainjector-reader", file.Objects[0].Name, "the name the placement rule wants in the directory")
	})

	t.Run("a capability marker longer than a label value", func(t *testing.T) {
		// With the levels alone, only the module name can push the marker
		// namespace-capability.<module>.superadmin past 63 characters: at 32 characters it does.
		decl := &rbacyaml.Declaration{APIVersion: rbacyaml.APIVersionV1Alpha1,
			Resources:    []rbacyaml.Resource{{Group: "x.io", Resource: "things", Scope: "Namespaced", Namespace: map[string][]string{"superadmin": {"get"}}}},
			Capabilities: map[string]rbacyaml.Capability{"namespace.superadmin": {Title: rbacyaml.LocalizedText{EN: "t", RU: "т"}, Description: rbacyaml.LocalizedText{EN: "d", RU: "д"}}},
		}
		_, err := Build(Input{Module: "a-module-name-of-thirty-two-char", Namespace: "d8-m", Decl: decl})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "a label value holds 63")

		_, err = Build(Input{Module: "a-module-name-of-thirtyone-char", Namespace: "d8-m", Decl: decl})
		require.NoError(t, err)
	})
}

// A capability with an action of its own is named after it and aggregates into the level the
// declaration gives it; the labels of the module go on it beside those of the role model.
func TestBuild_CapabilityOfItsOwnAction(t *testing.T) {
	const agent = "m.deckhouse.io/aggregate-to-agent"

	decl := &rbacyaml.Declaration{APIVersion: rbacyaml.APIVersionV1Alpha1,
		Resources: []rbacyaml.Resource{
			{Group: "x.io", Resource: "snapshots", Scope: "Namespaced", Namespace: map[string][]string{"viewer": {"get"}, "download_snapshots": {"create"}}},
			{Group: "x.io", Resource: "volumes", Scope: "Cluster", System: map[string][]string{"transfer": {"update"}}},
		},
		Capabilities: map[string]rbacyaml.Capability{
			"namespace.download_snapshots": {Level: "admin", Labels: map[string]string{agent: "true"},
				Title: rbacyaml.LocalizedText{EN: "t", RU: "т"}, Description: rbacyaml.LocalizedText{EN: "d", RU: "д"}},
			"namespace.view": {Labels: map[string]string{agent: "true"}},
			"system.transfer": {Level: "manager",
				Title: rbacyaml.LocalizedText{EN: "t2", RU: "т2"}, Description: rbacyaml.LocalizedText{EN: "d2", RU: "д2"}},
		},
	}
	require.Empty(t, rbacyaml.Validate(decl, nil))

	model, err := Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"storage"}, Decl: decl})
	require.NoError(t, err)

	own := model.File("templates/rbacv2/use/download_snapshots.yaml")
	require.NotNil(t, own)
	require.Len(t, own.Objects, 1)
	assert.Equal(t, "d8:namespace-capability:m:download_snapshots", own.Objects[0].Name)
	assert.Equal(t, map[string]string{
		rbaccontract.LabelKind:                        rbaccontract.KindCapability,
		rbaccontract.LabelScope:                       rbaccontract.LineageNamespace,
		rbaccontract.LabelCapability:                  "namespace-capability.m.download_snapshots",
		"rbac.deckhouse.io/aggregate-to-namespace-as": "admin",
		agent: "true",
	}, own.Objects[0].Labels)
	assert.Equal(t, "t", own.Objects[0].Annotations[rbaccontract.AnnotationTitleEN])
	assert.Contains(t, RenderFile(*own), `"`+agent+`" "true"`, "the label of the module is written with the others")

	view := model.File("templates/rbacv2/use/view.yaml")
	require.NotNil(t, view)
	assert.Equal(t, "true", view.Objects[0].Labels[agent])
	assert.Equal(t, "viewer", view.Objects[0].Labels["rbac.deckhouse.io/aggregate-to-namespace-as"])

	transfer := model.File("templates/rbacv2/manage/transfer.yaml")
	require.NotNil(t, transfer)
	assert.Equal(t, "d8:system-capability:m:transfer", transfer.Objects[0].Name)
	assert.Equal(t, "manager", transfer.Objects[0].Labels["rbac.deckhouse.io/aggregate-to-storage-as"])
	assert.NotContains(t, transfer.Objects[0].Labels, agent)
}

func TestYAMLScalar(t *testing.T) {
	for in, want := range map[string]string{
		"d8:cert-manager:cainjector": "d8:cert-manager:cainjector",
		"/metrics":                   "/metrics",
		"*":                          `"*"`,
		"*foo":                       `"*foo"`,
		"pods/*":                     "pods/*",
		"pods/log":                   "pods/log",
		"cert-manager.io":            "cert-manager.io",
		"":                           `""`,
		"no":                         `"no"`,
		"Yes":                        `"Yes"`,
		"off":                        `"off"`,
		"null":                       `"null"`,
		"123":                        `"123"`,
		"foo:":                       `"foo:"`,
		"a: b":                       `"a: b"`,
		"-lead":                      `"-lead"`,
		"a #b":                       `"a #b"`,
	} {
		assert.Equal(t, want, yamlScalar(in), "input %q", in)
	}
}

// Every value the generator writes must read back as itself: a wildcard in a service account's
// rules renders a file that parses and carries the "*".
func TestRender_WildcardRoundTrip(t *testing.T) {
	decl := &rbacyaml.Declaration{APIVersion: rbacyaml.APIVersionV1Alpha1,
		Resources: []rbacyaml.Resource{{Group: "x.io", Resource: "things", Scope: "Cluster", System: map[string][]string{"viewer": {"get"}}}},
		ServiceAccounts: []rbacyaml.ServiceAccount{{Name: "worker", ClusterRules: []rbacyaml.PolicyRule{
			{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}},
		}}},
	}

	model, err := Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Decl: decl})
	require.NoError(t, err)

	file := model.File("templates/rbac-for-us.yaml")
	require.NotNil(t, file)

	for _, doc := range strings.Split(RenderFile(*file), "\n---\n") {
		if !strings.HasPrefix(doc, "apiVersion: rbac.authorization.k8s.io/v1\nkind: ClusterRole\n") {
			continue
		}

		var role struct {
			Rules []struct {
				APIGroups []string `json:"apiGroups"`
				Verbs     []string `json:"verbs"`
			} `json:"rules"`
		}
		require.NoError(t, sigsyaml.Unmarshal([]byte(helmTemplateLines.ReplaceAllString(doc, "")), &role), doc)
		require.Len(t, role.Rules, 1)
		assert.Equal(t, []string{"*"}, role.Rules[0].APIGroups)
		assert.Equal(t, []string{"*"}, role.Rules[0].Verbs)
	}
}

// helmTemplateLines drops the Helm actions of a generated document so it parses as plain YAML.
var helmTemplateLines = regexp.MustCompile(`(?m)^.*\{\{.*\}\}.*$`)

// A ServiceAccount and an access entry of one name map to the same ClusterRole in two templates;
// Helm would refuse the release, so the model does (review of #479, finding 10).
func TestBuild_RefusesNamesCollidingAcrossFiles(t *testing.T) {
	decl := &rbacyaml.Declaration{APIVersion: rbacyaml.APIVersionV1Alpha1,
		Resources:       []rbacyaml.Resource{{Group: "x.io", Resource: "things", Scope: "Cluster", System: map[string][]string{"viewer": {"get"}}}},
		ServiceAccounts: []rbacyaml.ServiceAccount{{Name: "webhook", Path: "webhook", ClusterRules: []rbacyaml.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}}}}},
		Access: []rbacyaml.Access{{Name: "webhook", Subjects: []rbacyaml.Subject{{Kind: "Group", Name: "g"}},
			ClusterRules: []rbacyaml.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"list"}}}}},
	}

	_, err := Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Decl: decl})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "would both hold ClusterRole/d8:m:webhook")
}

// Two declared roles whose names differ only by the separator produce one binding name; the
// model refuses instead of writing a file with two objects of one name.
func TestBuild_RefusesDuplicateGeneratedNames(t *testing.T) {
	decl := &rbacyaml.Declaration{APIVersion: rbacyaml.APIVersionV1Alpha1,
		Resources:       []rbacyaml.Resource{{Group: "x.io", Resource: "things", Scope: "Cluster", System: map[string][]string{"viewer": {"get"}}}},
		ServiceAccounts: []rbacyaml.ServiceAccount{{Name: "worker", BindClusterRoles: []string{"d8:a:b", "d8:a-b"}}},
	}

	_, err := Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Decl: decl})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "would hold two objects named ClusterRoleBinding/d8:m:worker:a-b")
}

// prometheusAccess.when gates the binding to the scraper only; the Role stays unconditional, as
// the modules write it today.
func TestBuild_PrometheusAccessWhen(t *testing.T) {
	decl := &rbacyaml.Declaration{APIVersion: rbacyaml.APIVersionV1Alpha1,
		Resources:        []rbacyaml.Resource{{Group: "x.io", Resource: "things", Scope: "Cluster", System: map[string][]string{"viewer": {"get"}}}},
		PrometheusAccess: rbacyaml.PrometheusAccessList{{Deployments: []string{"m"}, When: `.Values.global.enabledModules | has "prometheus"`}},
	}

	model, err := Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Decl: decl})
	require.NoError(t, err)

	file := model.File("templates/rbac-to-us.yaml")
	require.NotNil(t, file)
	require.Len(t, file.Objects, 2)
	assert.Equal(t, "Role", file.Objects[0].Kind)
	assert.Empty(t, file.Objects[0].When)
	assert.Equal(t, "RoleBinding", file.Objects[1].Kind)
	assert.Equal(t, `.Values.global.enabledModules | has "prometheus"`, file.Objects[1].When)

	rendered := RenderFile(*file)
	assert.Equal(t, 1, strings.Count(rendered, `{{- if .Values.global.enabledModules | has "prometheus" }}`))
	assert.Less(t, strings.Index(rendered, "kind: Role\n"), strings.Index(rendered, "{{- if"), "the Role comes before the gate")
}

// A capability whose every rule is conditional is conditional as a whole: it is not rendered with
// an empty rules list when the conditions do not hold (review of #479, finding 11).
func TestBuild_AllRulesUnderWhenLiftTheCondition(t *testing.T) {
	declWith := func(whens ...string) *rbacyaml.Declaration {
		d := &rbacyaml.Declaration{APIVersion: rbacyaml.APIVersionV1Alpha1}
		for i, w := range whens {
			d.Resources = append(d.Resources, rbacyaml.Resource{Group: "x.io", Resource: fmt.Sprintf("things%d", i), Scope: "Namespaced", When: w,
				Namespace: map[string][]string{"viewer": {"get"}}})
		}

		return d
	}

	capability := func(t *testing.T, decl *rbacyaml.Declaration) Object {
		t.Helper()

		model, err := Build(Input{Module: "m", Namespace: "d8-m", Decl: decl})
		require.NoError(t, err)

		f := model.File("templates/rbacv2/use/view.yaml")
		require.NotNil(t, f)

		return f.Objects[0]
	}

	one := capability(t, declWith(".Values.m.a", ".Values.m.a"))
	assert.Equal(t, ".Values.m.a", one.When)
	assert.Empty(t, one.Rules[0].When, "a shared condition leaves the rules")

	two := capability(t, declWith(".Values.m.a", ".Values.m.b"))
	assert.Equal(t, "or (.Values.m.a) (.Values.m.b)", two.When)
	assert.Equal(t, ".Values.m.a", two.Rules[0].When, "different conditions stay on their rules")

	mixed := capability(t, declWith(".Values.m.a", ""))
	assert.Empty(t, mixed.When, "an unconditional rule keeps the role unconditional")
}

// Without a subsystem a system capability would aggregate into no role: none is written, view and
// edit included (review of #480).
func TestBuild_NoSubsystemsNoSystemCapabilities(t *testing.T) {
	decl := &rbacyaml.Declaration{APIVersion: rbacyaml.APIVersionV1Alpha1,
		Resources: []rbacyaml.Resource{{Group: "x.io", Resource: "things", Scope: "Namespaced", Namespace: map[string][]string{"viewer": {"get"}}}},
	}

	model, err := Build(Input{Module: "m", Namespace: "d8-m", Decl: decl})
	require.NoError(t, err)
	assert.Nil(t, model.File("templates/rbacv2/manage/view.yaml"))
	assert.Nil(t, model.File("templates/rbacv2/manage/edit.yaml"))
	assert.NotNil(t, model.File("templates/rbacv2/use/view.yaml"))
}
