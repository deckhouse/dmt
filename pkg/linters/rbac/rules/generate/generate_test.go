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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	return Input{Module: "cert-manager", Namespace: "d8-cert-manager", Subsystems: []string{"security"}, Decl: decl}
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

func TestBuild_SubsystemsOverrideAndNamespaceLabel(t *testing.T) {
	in := certManagerInput(t)
	in.Decl.Subsystems = []string{"networking", "kubernetes"}
	in.Namespace = "default"
	in.Decl.Normalize()

	model, err := Build(in)
	require.NoError(t, err)

	edit := model.File("templates/rbacv2/manage/edit.yaml").Objects[0]
	assert.Equal(t, []LineageLevel{{Lineage: "kubernetes", Level: "manager"}, {Lineage: "networking", Level: "manager"}}, edit.AggregationLabels())
	_, hasNamespaceLabel := edit.Labels["rbac.deckhouse.io/namespace"]
	assert.False(t, hasNamespaceLabel, "the namespace label is set only for a d8- namespace")
	assert.Equal(t, []string{"create", "delete", "patch", "update"}, edit.Rules[len(edit.Rules)-1].Verbs, "the edit ModuleConfig rule has no read verbs")
}

func TestRender_GoldenAndIdempotent(t *testing.T) {
	model, err := Build(certManagerInput(t))
	require.NoError(t, err)

	first := Render(model)
	second := Render(model)
	assert.Equal(t, first, second, "rendering is a pure function of the model")

	for _, f := range first {
		golden := filepath.Join("testdata", "cert-manager", "expected", f.Path)

		if updateGolden {
			require.NoError(t, os.MkdirAll(filepath.Dir(golden), 0o755))
			require.NoError(t, os.WriteFile(golden, []byte(f.Content), 0o600))
		}

		want, err := os.ReadFile(golden)
		require.NoError(t, err, "missing golden file %s (run with UPDATE_GOLDEN=1)", golden)
		assert.Equal(t, string(want), f.Content, "generated %s differs from the golden file", f.Path)

		generated, version := ParseHeader(f.Content)
		assert.True(t, generated)
		assert.Equal(t, "1", version)
	}
}

func TestParseHeader(t *testing.T) {
	assert.True(t, func() bool { g, _ := ParseHeader(Header() + "\n---\n"); return g }())

	generated, version := ParseHeader("# Generated by dmt (rbac/sync) from rbac.yaml, contract 0. Edit rbac.yaml and run \"dmt lint --linter rbac --fix\", or remove this line to maintain the file by hand.\n")
	assert.True(t, generated)
	assert.Equal(t, "0", version)

	generated, _ = ParseHeader("---\napiVersion: v1\n")
	assert.False(t, generated, "a file without the header is maintained by hand")
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
		require.NoError(t, err, "the declaration's own subsystems suffice")
	})

	t.Run("an account whose name does not follow its directory", func(t *testing.T) {
		decl := base()
		decl.ServiceAccounts = []rbacyaml.ServiceAccount{{Name: "helper", Path: "cainjector"}}
		_, err := Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Decl: decl})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `the placement rule wants the account named "cainjector" or "m-cainjector"`)

		decl.ServiceAccounts = []rbacyaml.ServiceAccount{{Name: "m-cainjector", Path: "cainjector"}, {Name: "webhook", Path: "webhook"}, {Name: "anything", Path: ""}}
		_, err = Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Decl: decl})
		require.NoError(t, err)

		decl.ServiceAccounts = []rbacyaml.ServiceAccount{{Name: "dir", Path: "some/nested/dir"}}
		_, err = Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Decl: decl})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "one directory under templates/ only")
	})

	t.Run("a capability marker longer than a label value", func(t *testing.T) {
		// The levels are a fixed set, so only the module name can push the marker
		// namespace-capability.<module>.superadmin past 63 characters: at 32 characters it does.
		decl := &rbacyaml.Declaration{APIVersion: rbacyaml.APIVersionV1Alpha1,
			Resources:    []rbacyaml.Resource{{Group: "x.io", Resource: "things", Scope: "Namespaced", Namespace: map[string][]string{"superadmin": {"get"}}}},
			Capabilities: map[string]rbacyaml.CapabilityText{"namespace.superadmin": {Title: rbacyaml.LocalizedText{EN: "t", RU: "т"}, Description: rbacyaml.LocalizedText{EN: "d", RU: "д"}}},
		}
		_, err := Build(Input{Module: "a-module-name-of-thirty-two-char", Namespace: "d8-m", Decl: decl})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "a label value holds 63")

		_, err = Build(Input{Module: "a-module-name-of-thirtyone-char", Namespace: "d8-m", Decl: decl})
		require.NoError(t, err)
	})
}

// A generated file that acquired CRLF line endings is still the generator's.
func TestParseHeader_CRLF(t *testing.T) {
	generated, version := ParseHeader(Header() + "\r\n---\r\n")
	assert.True(t, generated)
	assert.Equal(t, "1", version)
}
