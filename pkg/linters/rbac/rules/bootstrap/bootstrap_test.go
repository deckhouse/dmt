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

package bootstrap

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/generate"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbaccontract"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbacyaml"
)

// objectsOf simulates the render of a model: every object as the chart would produce it, with the
// module labels helm_lib adds and the module namespace stripped, as dmt renders it.
func objectsOf(model *generate.Model, module, namespace string) []Object {
	out := make([]Object, 0, len(model.Files))

	for _, file := range model.Files {
		for _, o := range file.Objects {
			labels := map[string]string{"heritage": "deckhouse", "module": module}
			for k, v := range o.Labels {
				labels[k] = v
			}

			ns := o.Namespace
			if ns == namespace {
				ns = ""
			}

			obj := Object{Kind: o.Kind, Name: o.Name, Namespace: ns, Path: file.Path, Labels: labels, Annotations: o.Annotations, Automount: o.AutomountToken}

			for _, r := range o.Rules {
				obj.Rules = append(obj.Rules, rbacv1.PolicyRule{APIGroups: r.APIGroups, Resources: r.Resources, ResourceNames: r.ResourceNames, NonResourceURLs: r.NonResourceURLs, Verbs: r.Verbs})
			}

			if o.RoleRefKind != "" {
				obj.RoleRef = rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: o.RoleRefKind, Name: o.RoleRefName}
			}

			for _, s := range o.Subjects {
				obj.Subjects = append(obj.Subjects, rbacv1.Subject{Kind: s.Kind, Name: s.Name, Namespace: s.Namespace})
			}

			out = append(out, obj)
		}
	}

	return out
}

var certManagerCRDs = map[string]string{
	"cert-manager.io/certificates": "Namespaced", "cert-manager.io/certificaterequests": "Namespaced", "cert-manager.io/issuers": "Namespaced",
	"cert-manager.io/clusterissuers": "Cluster", "acme.cert-manager.io/orders": "Namespaced", "acme.cert-manager.io/challenges": "Namespaced",
}

// The declaration the generator renders comes back from its render: what the fixture declares is
// what the importer writes, up to what the render cannot show (when, reasons).
func TestBuild_RoundTripOnTheCertManagerFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "generate", "testdata", "cert-manager", "rbac.yaml"))
	require.NoError(t, err)

	want, err := rbacyaml.Parse(raw)
	require.NoError(t, err)

	model, err := generate.Build(generate.Input{Module: "cert-manager", Namespace: "d8-cert-manager", Subsystems: []string{"security"}, Decl: want})
	require.NoError(t, err)

	got := Build(Input{Module: "cert-manager", Namespace: "d8-cert-manager", Subsystems: []string{"security"}, Objects: objectsOf(model, "cert-manager", "d8-cert-manager"), CRDs: certManagerCRDs})
	require.Empty(t, got.Unmanaged, "everything the generator wrote is described again")

	// resources: same keys and levels
	keyOf := func(r rbacyaml.Resource) string { return r.Group + "/" + r.Resource }

	wantRes := map[string]rbacyaml.Resource{}
	for _, r := range want.Resources {
		wantRes[keyOf(r)] = r
	}

	gotRes := map[string]rbacyaml.Resource{}
	for _, r := range got.Decl.Resources {
		gotRes[keyOf(r)] = r
	}

	for k, w := range wantRes {
		g, ok := gotRes[k]
		require.True(t, ok, "resource %s missing", k)
		assert.Equal(t, w.NoAccess != "", g.NoAccess != "", "%s: denied", k)

		if _, backed := certManagerCRDs[k]; backed {
			assert.Empty(t, g.Scope, "%s: the CRD carries the scope, the entry does not repeat it", k)
		}

		assert.Equal(t, w.Namespace, g.Namespace, "%s: namespace levels", k)
		assert.Equal(t, w.System, g.System, "%s: system levels", k)
		assert.Equal(t, w.Legacy, g.Legacy, "%s: legacy levels", k)
	}

	assert.Len(t, gotRes, len(wantRes))

	// capabilities texts for the non-conventional level
	assert.Equal(t, want.Capabilities, got.Decl.Capabilities)

	// service accounts
	byName := func(list []rbacyaml.ServiceAccount) map[string]rbacyaml.ServiceAccount {
		m := map[string]rbacyaml.ServiceAccount{}
		for _, sa := range list {
			m[sa.Name] = sa
		}

		return m
	}
	wantSA, gotSA := byName(want.ServiceAccounts), byName(got.Decl.ServiceAccounts)
	require.Len(t, gotSA, len(wantSA))

	for name, w := range wantSA {
		g := gotSA[name]
		assert.Equal(t, w.Path, g.Path, "%s: path", name)
		assert.Equal(t, w.ClusterRules, g.ClusterRules, "%s: clusterRules", name)
		assert.Equal(t, w.NamespaceRules, g.NamespaceRules, "%s: namespaceRules", name)
		assert.ElementsMatch(t, w.BindClusterRoles, g.BindClusterRoles, "%s: bindClusterRoles", name)
		assert.ElementsMatch(t, w.BindRoles, g.BindRoles, "%s: bindRoles", name)
		assert.Equal(t, len(w.ExtraClusterRoles), len(g.ExtraClusterRoles), "%s: extraClusterRoles", name)
		assert.Nil(t, g.AutomountToken, "%s: the generator wrote automount false, so the import leaves it unset", name)
	}

	// access and prometheus
	wantAccess := map[string]rbacyaml.Access{}
	for _, a := range want.Access {
		wantAccess[a.Name] = a
	}

	for _, a := range got.Decl.Access {
		w, ok := wantAccess[a.Name]
		require.True(t, ok, "access %s unexpected", a.Name)
		assert.Equal(t, w.ClusterRules, a.ClusterRules)
		assert.Equal(t, w.NamespaceRules, a.NamespaceRules)
		assert.ElementsMatch(t, w.Subjects, a.Subjects)
	}

	assert.Len(t, got.Decl.Access, len(want.Access))
	require.NotNil(t, got.Decl.PrometheusAccess)
	assert.ElementsMatch(t, want.PrometheusAccess.Deployments, got.Decl.PrometheusAccess.Deployments)

	// nothing to rename: the generator's names come back as themselves
	for _, n := range got.Notes {
		assert.NotContains(t, n, "will be named", "note: %s", n)
	}

	// and the file it writes parses and validates
	content, err := Marshal(got)
	require.NoError(t, err)
	again, err := rbacyaml.Parse(content)
	require.NoError(t, err)
	assert.Empty(t, rbacyaml.Validate(again, rbacyaml.CRDScopes(certManagerCRDs)), "the written declaration validates")
}

// What the importer cannot decide is a TODO or a note, and objects outside the format stay listed.
// A grant limited to resourceNames cannot be kept on a capability. When every grant of the
// resource is limited, the entry is left undecided rather than widened; when one grant is
// unrestricted, the levels stand.
func TestBuild_ResourceNamesAreNotWidened(t *testing.T) {
	labels := map[string]string{"module": "m", "rbac.deckhouse.io/kind": "capability", "rbac.deckhouse.io/scope": "namespace", "rbac.deckhouse.io/aggregate-to-namespace-as": "viewer"}
	objects := []Object{
		{Kind: "ClusterRole", Name: "d8:namespace-capability:m:view", Path: "templates/rbacv2/use/view.yaml", Labels: labels,
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"configmaps"}, ResourceNames: []string{"m-config"}, Verbs: []string{"get"}},
				{APIGroups: []string{""}, Resources: []string{"secrets"}, ResourceNames: []string{"m-token"}, Verbs: []string{"get"}},
				{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"list"}},
			}},
	}

	got := Build(Input{Module: "m", Namespace: "d8-m", Objects: objects})

	byKey := map[string]rbacyaml.Resource{}
	for _, r := range got.Decl.Resources {
		byKey[r.Group+"/"+r.Resource] = r
	}

	require.Contains(t, byKey, "/configmaps")
	assert.Contains(t, byKey["/configmaps"].NoAccess, "TODO")
	assert.Empty(t, byKey["/configmaps"].Namespace)
	assert.Equal(t, "Namespaced", byKey["/configmaps"].Scope)

	require.Contains(t, byKey, "/secrets")
	assert.Empty(t, byKey["/secrets"].NoAccess)
	assert.Equal(t, []string{"list"}, byKey["/secrets"].Namespace["viewer"], "get was limited to m-token and is not widened")

	notes := strings.Join(got.Notes, "\n")
	assert.Contains(t, notes, "/configmaps: every grant carried resourceNames")
	assert.Contains(t, notes, "/secrets: grants limited to resourceNames are not carried over, the format would grant them on every object: namespace/viewer: get on [m-token]")
}

// An unrestricted grant at one level does not carry a restricted grant at another level with it
// (review of #479, finding 8).
func TestBuild_ResourceNamesOfOneLevelDoNotRideOnAnother(t *testing.T) {
	capability := func(name, level string, rules ...rbacv1.PolicyRule) Object {
		return Object{Kind: "ClusterRole", Name: name, Path: "templates/rbacv2/use/x.yaml", Rules: rules,
			Labels: map[string]string{"module": "m", "rbac.deckhouse.io/kind": "capability", "rbac.deckhouse.io/scope": "namespace", "rbac.deckhouse.io/aggregate-to-namespace-as": level}}
	}

	got := Build(Input{Module: "m", Namespace: "d8-m", Objects: []Object{
		capability("d8:namespace-capability:m:view", "viewer", rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"list"}}),
		capability("d8:namespace-capability:m:edit", "manager", rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"secrets"}, ResourceNames: []string{"m-config"}, Verbs: []string{"get", "update", "delete"}}),
	}})

	var secrets rbacyaml.Resource

	for _, r := range got.Decl.Resources {
		if r.Resource == "secrets" {
			secrets = r
		}
	}

	assert.Equal(t, []string{"list"}, secrets.Namespace["viewer"])
	assert.NotContains(t, secrets.Namespace, "manager", "the manager grant named m-config only")
	assert.Contains(t, strings.Join(got.Notes, "\n"), "namespace/manager: get,update,delete on [m-config]")
}

// The lint path fills the input from a map; the result must not depend on that order.
func TestBuild_IsIndependentOfInputOrder(t *testing.T) {
	yes := true
	objects := []Object{
		{Kind: "ClusterRole", Name: "d8:namespace-capability:m:view", Path: "templates/rbacv2/use/view.yaml",
			Labels: map[string]string{"module": "m", "rbac.deckhouse.io/kind": "capability", "rbac.deckhouse.io/scope": "namespace", "rbac.deckhouse.io/aggregate-to-namespace-as": "viewer"},
			Rules:  []rbacv1.PolicyRule{{APIGroups: []string{"trivy.deckhouse.io"}, Resources: []string{"vulnerabilityreports"}, Verbs: []string{"get"}}}},
		{Kind: "ClusterRole", Name: "d8:use:capability:module:m:view", Path: "templates/rbacv2/use/old.yaml", Labels: map[string]string{"module": "m", "rbac.deckhouse.io/kind": "use"}},
		{Kind: "ClusterRole", Name: "d8:namespace-capability:kubernetes:view_logs", Path: "templates/rbacv2/global/x.yaml", Labels: map[string]string{"module": "m", "rbac.deckhouse.io/kind": "capability"}},
		{Kind: "ServiceAccount", Name: "webhook", Path: "templates/webhook/rbac-for-us.yaml", Labels: map[string]string{"module": "m", "app": "webhook"}, Automount: &yes},
		{Kind: "ClusterRole", Name: "d8:m:webhook:requester", Path: "templates/webhook/rbac-for-us.yaml", Labels: map[string]string{"module": "m"}, Rules: []rbacv1.PolicyRule{{APIGroups: []string{"x"}, Resources: []string{"y"}, Verbs: []string{"create"}}}},
	}

	forward := Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Objects: objects, CRDs: map[string]string{"deckhouse.io/things": "Namespaced"}})

	reversed := make([]Object, 0, len(objects))
	for i := len(objects) - 1; i >= 0; i-- {
		reversed = append(reversed, objects[i])
	}

	backward := Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Objects: reversed, CRDs: map[string]string{"deckhouse.io/things": "Namespaced"}})

	assert.Equal(t, forward, backward)
}

func TestBuild_TODOsAndUnmanaged(t *testing.T) {
	yes := true
	objects := []Object{
		// a capability granting a resource without a CRD in the module and a well-known core one
		{Kind: "ClusterRole", Name: "d8:namespace-capability:m:view", Path: "templates/rbacv2/use/view.yaml",
			Labels: map[string]string{"module": "m", "rbac.deckhouse.io/kind": "capability", "rbac.deckhouse.io/scope": "namespace", "rbac.deckhouse.io/aggregate-to-namespace-as": "viewer"},
			Rules:  []rbacv1.PolicyRule{{APIGroups: []string{"trivy.deckhouse.io"}, Resources: []string{"vulnerabilityreports"}, Verbs: []string{"get"}}, {APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}}}},
		// a legacy scheme capability and a platform capability
		{Kind: "ClusterRole", Name: "d8:use:capability:module:m:view", Path: "templates/rbacv2/use/old.yaml", Labels: map[string]string{"module": "m", "rbac.deckhouse.io/kind": "use"}},
		{Kind: "ClusterRole", Name: "d8:namespace-capability:kubernetes:view_logs", Path: "templates/rbacv2/global/x.yaml", Labels: map[string]string{"module": "m", "rbac.deckhouse.io/kind": "capability"}},
		// an account that mounts its token, with an unbound role in its file
		{Kind: "ServiceAccount", Name: "webhook", Path: "templates/webhook/rbac-for-us.yaml", Labels: map[string]string{"module": "m", "app": "webhook"}, Automount: &yes},
		{Kind: "ClusterRole", Name: "d8:m:webhook:requester", Path: "templates/webhook/rbac-for-us.yaml", Labels: map[string]string{"module": "m"}, Rules: []rbacv1.PolicyRule{{APIGroups: []string{"x"}, Resources: []string{"y"}, Verbs: []string{"create"}}}},
	}

	got := Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Objects: objects, CRDs: map[string]string{"deckhouse.io/things": "Namespaced"}})

	sort.Strings(got.Notes)

	joined := ""
	for _, n := range got.Notes {
		joined += n + "\n"
	}

	assert.Contains(t, joined, "ServiceAccount webhook mounted its token")

	byKey := map[string]rbacyaml.Resource{}
	for _, r := range got.Decl.Resources {
		byKey[r.Group+"/"+r.Resource] = r
	}

	assert.Equal(t, "Namespaced", byKey["/pods"].Scope, "a well-known core resource gets its scope")
	assert.Equal(t, "TODO: Namespaced or Cluster (Cluster drops the namespace levels)", byKey["trivy.deckhouse.io/vulnerabilityreports"].Scope, "an unknown one is a TODO value for the author (review of #479, reply to finding 9)")
	assert.Contains(t, byKey["deckhouse.io/things"].NoAccess, "TODO", "a CRD nobody grants is an undecided entry")

	require.Len(t, got.Decl.ServiceAccounts, 1)
	sa := got.Decl.ServiceAccounts[0]
	assert.Equal(t, "webhook", sa.Path)
	require.NotNil(t, sa.AutomountToken)
	assert.True(t, *sa.AutomountToken)
	require.Len(t, sa.ExtraClusterRoles, 1)
	assert.Equal(t, "requester", sa.ExtraClusterRoles[0].Name)
	assert.False(t, sa.ExtraClusterRoles[0].IsBound())

	require.Len(t, got.Unmanaged, 2)
	assert.Contains(t, got.Unmanaged[0], "d8:namespace-capability:kubernetes:view_logs")
	assert.Contains(t, got.Unmanaged[1], "d8:use:capability:module:m:view")
}

// A system capability's moduleconfigs rule other than the one the generator adds is named, not
// dropped in silence; a ServiceAccount's RoleBinding to a ClusterRole stays hand-written instead of
// becoming a binding to a Role of that name (review of #479, findings 13c and 13d).
func TestBuild_WhatTheFormatCannotHoldIsNamed(t *testing.T) {
	capability := func(action, level string, rules ...rbacv1.PolicyRule) Object {
		return Object{Kind: "ClusterRole", Name: "d8:system-capability:m:" + action, Path: "templates/rbacv2/manage/" + action + ".yaml", Rules: rules,
			Labels: map[string]string{"module": "m", "rbac.deckhouse.io/kind": "capability", "rbac.deckhouse.io/scope": "system", "rbac.deckhouse.io/aggregate-to-security-as": level}}
	}
	moduleConfigs := func(names []string, verbs ...string) rbacv1.PolicyRule {
		return rbacv1.PolicyRule{APIGroups: []string{"deckhouse.io"}, Resources: []string{"moduleconfigs"}, ResourceNames: names, Verbs: verbs}
	}

	got := Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Objects: []Object{
		capability("view", "viewer", moduleConfigs([]string{"m"}, "get", "list", "watch")),
		capability("superadmin", "superadmin", moduleConfigs([]string{"m"}, "delete")),
		capability("edit", "manager", moduleConfigs([]string{"other"}, "update")),
		{Kind: "ServiceAccount", Name: "worker", Path: "templates/worker/rbac-for-us.yaml", Labels: map[string]string{"module": "m"}},
		{Kind: "RoleBinding", Name: "worker-view", Namespace: "d8-m", Path: "templates/worker/rbac-for-us.yaml", Labels: map[string]string{"module": "m"},
			RoleRef:  rbacv1.RoleRef{Kind: "ClusterRole", Name: "view"},
			Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "worker", Namespace: "d8-m"}}},
	}})

	notes := strings.Join(got.Notes, "\n")
	assert.NotContains(t, notes, "system/viewer: a moduleconfigs rule", "the generator's own rule is not a note")
	assert.Contains(t, notes, "system/superadmin: a moduleconfigs rule the generator does not produce (delete on [m])")
	assert.Contains(t, notes, "system/manager: a moduleconfigs rule the generator does not produce (update on [other])")

	require.Len(t, got.Decl.ServiceAccounts, 1)
	assert.Empty(t, got.Decl.ServiceAccounts[0].BindRoles)
	assert.Contains(t, strings.Join(got.Unmanaged, "\n"), "a RoleBinding to the ClusterRole view, which bindRoles cannot express")
}

// A role granting "*" verbs or API groups stays out of the declaration with a note, instead of
// being written in a shape the validation refuses (review of #479, finding 21); a grant on every
// ModuleConfig is an ordinary system entry (review of #479, reply to finding 13d).
func TestBuild_WildcardRolesAndEveryModuleConfig(t *testing.T) {
	got := Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, Objects: []Object{
		{Kind: "ClusterRole", Name: "d8:user-authz:m:super-admin", Path: "templates/user-authz-cluster-roles.yaml",
			Annotations: map[string]string{"user-authz.deckhouse.io/access-level": "SuperAdmin"},
			Rules:       []rbacv1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}}}},
		{Kind: "ClusterRole", Name: "d8:system-capability:m:view", Path: "templates/rbacv2/manage/view.yaml",
			Labels: map[string]string{"module": "m", "rbac.deckhouse.io/kind": "capability", "rbac.deckhouse.io/scope": "system", "rbac.deckhouse.io/aggregate-to-security-as": "viewer"},
			Rules:  []rbacv1.PolicyRule{{APIGroups: []string{"deckhouse.io"}, Resources: []string{"moduleconfigs"}, Verbs: []string{"get", "list", "watch"}}}},
	}})

	unmanaged := strings.Join(got.Unmanaged, "\n")
	assert.Contains(t, unmanaged, "d8:user-authz:m:super-admin")
	assert.Contains(t, unmanaged, "user-authz does not aggregate SuperAdmin")

	var moduleConfigs rbacyaml.Resource

	for _, r := range got.Decl.Resources {
		assert.NotEqual(t, "*", r.Group, "no wildcard entry is written")

		if r.Resource == "moduleconfigs" {
			moduleConfigs = r
		}
	}

	assert.Equal(t, []string{"get", "list", "watch"}, moduleConfigs.System["viewer"], "the grant on every ModuleConfig is kept")
}

// An aggregated ClusterRole of a ServiceAccount is not an extra role without rules: the account
// keeps its binding by name and the role stays hand-written (found while checking finding 21 on
// node-manager).
func TestBuild_AggregatedRoleOfAnAccountStaysHandWritten(t *testing.T) {
	labels := map[string]string{"module": "m"}
	got := Build(Input{Module: "m", Namespace: "d8-m", Objects: []Object{
		{Kind: "ServiceAccount", Name: "capi", Path: "templates/capi/rbac-for-us.yaml", Labels: labels},
		{Kind: "ClusterRole", Name: "d8:m:capi:aggregated", Path: "templates/capi/rbac-for-us.yaml", Labels: labels, Aggregated: true},
		{Kind: "ClusterRoleBinding", Name: "d8:m:capi:aggregated", Path: "templates/capi/rbac-for-us.yaml", Labels: labels,
			RoleRef:  rbacv1.RoleRef{Kind: "ClusterRole", Name: "d8:m:capi:aggregated"},
			Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "capi", Namespace: "d8-m"}}},
	}})

	require.Len(t, got.Decl.ServiceAccounts, 1)
	assert.Empty(t, got.Decl.ServiceAccounts[0].ExtraClusterRoles)
	assert.Equal(t, []string{"d8:m:capi:aggregated"}, got.Decl.ServiceAccounts[0].BindClusterRoles)
	assert.Contains(t, strings.Join(got.Unmanaged, "\n"), "ClusterRole/d8:m:capi:aggregated")
	assert.Empty(t, rbacyaml.Validate(got.Decl, nil), "the written declaration validates")
}

// Two bindings of one account to one role fold into the one binding the generator names, instead of
// producing a declaration that does not generate (review of #479, finding 28: node-manager).
func TestBuild_RepeatedBindingOfAnAccountFolds(t *testing.T) {
	labels := map[string]string{"module": "m"}
	subject := []rbacv1.Subject{{Kind: "ServiceAccount", Name: "autoscaler", Namespace: "d8-m"}}

	got := Build(Input{Module: "m", Namespace: "d8-m", Objects: []Object{
		{Kind: "ServiceAccount", Name: "autoscaler", Path: "templates/autoscaler/rbac-for-us.yaml", Labels: labels},
		{Kind: "ClusterRoleBinding", Name: "d8:m:autoscaler:rbac-proxy", Path: "templates/autoscaler/rbac-for-us.yaml", Labels: labels,
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "d8:rbac-proxy"}, Subjects: subject},
		{Kind: "ClusterRoleBinding", Name: "d8:m:autoscaler-mcm:rbac-proxy", Path: "templates/autoscaler/rbac-for-us.yaml", Labels: labels,
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "d8:rbac-proxy"}, Subjects: subject},
	}})

	require.Len(t, got.Decl.ServiceAccounts, 1)
	assert.Equal(t, []string{"d8:rbac-proxy"}, got.Decl.ServiceAccounts[0].BindClusterRoles)
	assert.Regexp(t, `ClusterRoleBinding d8:m:autoscaler(-mcm)?:rbac-proxy binds autoscaler to d8:rbac-proxy again; it folds into d8:m:autoscaler:rbac-proxy`, strings.Join(got.Notes, "\n"))
	assert.Empty(t, rbacyaml.Validate(got.Decl, nil))
}

// Annotations of an account and of its roles survive the import, apart from Helm's and the
// generator's own; a nested access Role keeps its entry name (regression hunt, B7 and B8).
func TestBuild_AnnotationsAndNestedAccessNames(t *testing.T) {
	labels := map[string]string{"module": "m"}
	subject := []rbacv1.Subject{{Kind: "ServiceAccount", Name: "m", Namespace: "d8-m"}}
	rules := []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"get"}}}

	got := Build(Input{Module: "m", Namespace: "d8-m", Objects: []Object{
		{Kind: "ServiceAccount", Name: "m", Path: "templates/rbac-for-us.yaml", Labels: labels,
			Annotations: map[string]string{"helm.sh/resource-policy": "keep", "meta.helm.sh/release-name": "m"}},
		{Kind: "ClusterRole", Name: "d8:m:m", Path: "templates/rbac-for-us.yaml", Labels: labels, Rules: rules,
			Annotations: map[string]string{"werf.io/deploy-on": "pre-install"}},
		{Kind: "ClusterRoleBinding", Name: "d8:m:m", Path: "templates/rbac-for-us.yaml", Labels: labels,
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "d8:m:m"}, Subjects: subject,
			Annotations: map[string]string{"werf.io/deploy-on": "pre-install"}},
		{Kind: "Role", Name: "access-to-webhook-reader", Namespace: "d8-m", Path: "templates/webhook/rbac-to-us.yaml", Labels: labels,
			Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}}}},
		{Kind: "RoleBinding", Name: "access-to-webhook-reader", Namespace: "d8-m", Path: "templates/webhook/rbac-to-us.yaml", Labels: labels,
			RoleRef: rbacv1.RoleRef{Kind: "Role", Name: "access-to-webhook-reader"}, Subjects: []rbacv1.Subject{{Kind: "Group", Name: "g"}}},
	}})

	require.Len(t, got.Decl.ServiceAccounts, 1)
	assert.Equal(t, map[string]string{"helm.sh/resource-policy": "keep"}, got.Decl.ServiceAccounts[0].Annotations)
	assert.Equal(t, map[string]string{"werf.io/deploy-on": "pre-install"}, got.Decl.ServiceAccounts[0].RBACAnnotations)

	require.Len(t, got.Decl.Access, 1)
	assert.Equal(t, "reader", got.Decl.Access[0].Name)
	assert.Equal(t, "webhook", got.Decl.Access[0].Path)
	assert.NotContains(t, strings.Join(got.Notes, "\n"), "will be named", "the generator writes the names the module already has")
	assert.Empty(t, rbacyaml.Validate(got.Decl, nil))
}

// Labels and annotations of an access entry and of the scrape access survive the import
// (regression hunt 2, A2 and A4).
func TestBuild_AccessMetadataImported(t *testing.T) {
	labels := map[string]string{"module": "m", "heritage": "deckhouse", "app": "capi"}
	hook := map[string]string{"werf.io/deploy-on": "pre-install"}
	nodes := []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"get"}}}

	got := Build(Input{Module: "m", Namespace: "d8-m", Objects: []Object{
		{Kind: "ClusterRole", Name: "d8:m:manager", Path: "templates/rbac-for-us.yaml", Labels: labels, Annotations: hook, Rules: nodes},
		{Kind: "ClusterRoleBinding", Name: "d8:m:manager", Path: "templates/rbac-for-us.yaml", Labels: labels, Annotations: hook,
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "d8:m:manager"}, Subjects: []rbacv1.Subject{{Kind: "Group", Name: "g"}}},
		{Kind: "ServiceAccount", Name: "m", Path: "templates/rbac-for-us.yaml", Labels: map[string]string{"module": "m", "app.kubernetes.io/part-of": "gatekeeper"}},
	}})

	require.Len(t, got.Decl.Access, 1)
	assert.Equal(t, map[string]string{"app": "capi"}, got.Decl.Access[0].Labels)
	assert.Equal(t, hook, got.Decl.Access[0].Annotations)

	require.Len(t, got.Decl.ServiceAccounts, 1)
	assert.Equal(t, map[string]string{"app.kubernetes.io/part-of": "gatekeeper"}, got.Decl.ServiceAccounts[0].Labels)
}

// What a library renders stays hand-written, apart from the legacy roles and capabilities sync
// owns by class; a role without rules stays hand-written and its binding binds a hand-written
// role (review of #479, findings 32 and 39).
func TestBuild_LibraryAndEmptyRolesAreSetAside(t *testing.T) {
	labels := map[string]string{"module": "m"}
	sa := []rbacv1.Subject{{Kind: "ServiceAccount", Name: "m", Namespace: "d8-m"}}

	got := Build(Input{Module: "m", Namespace: "d8-m", Objects: []Object{
		{Kind: "ServiceAccount", Name: "csi", Path: "templates/csi/rbac-for-us.yaml", Labels: labels, Unmanageable: "rendered by an include of a named template (helm_lib or another chart), which owns it", Located: true, Library: true},
		{Kind: "ClusterRole", Name: "d8:m:csi:controller", Path: "templates/csi/rbac-for-us.yaml", Labels: labels, Unmanageable: "rendered by an include of a named template (helm_lib or another chart), which owns it", Located: true, Library: true,
			Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"get"}}}},
		{Kind: "ClusterRole", Name: "d8:m:user", Path: "templates/user-authz-cluster-roles.yaml", Labels: labels, Unmanageable: "rendered by an include of a named template (helm_lib or another chart), which owns it", Located: true, Library: true,
			Annotations: map[string]string{rbaccontract.AccessLevelAnnotation: "User"},
			Rules:       []rbacv1.PolicyRule{{APIGroups: []string{"x.io"}, Resources: []string{"things"}, Verbs: []string{"get"}}}},
		{Kind: "ServiceAccount", Name: "m", Path: "templates/rbac-for-us.yaml", Labels: labels},
		{Kind: "ClusterRole", Name: "d8:m:m:iop:istiod-1x25", Path: "templates/rbac-for-us.yaml", Labels: labels},
		{Kind: "ClusterRoleBinding", Name: "d8:m:m:iop:istiod-1x25", Path: "templates/rbac-for-us.yaml", Labels: labels,
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "d8:m:m:iop:istiod-1x25"}, Subjects: sa},
	}, CRDs: map[string]string{"x.io/things": "Namespaced"}})

	unmanaged := strings.Join(got.Unmanaged, "\n")
	assert.Contains(t, unmanaged, "ServiceAccount/csi (templates/csi/rbac-for-us.yaml): rendered by an include of a named template")
	assert.NotContains(t, unmanaged, "ClusterRole/d8:m:user", "a legacy role a library renders is sync's")
	assert.Contains(t, unmanaged, "ClusterRole/d8:m:csi:controller (templates/csi/rbac-for-us.yaml): rendered by an include")
	assert.Contains(t, unmanaged, "ClusterRole/d8:m:m:iop:istiod-1x25 (templates/rbac-for-us.yaml): has no rules")
	assert.Equal(t, 3, strings.Count(unmanaged, "\n")+1, "got: %s", unmanaged)

	require.Len(t, got.Decl.Resources, 1, "the legacy role a library renders is imported")
	assert.Contains(t, got.Decl.Resources[0].Legacy, "User")

	require.Len(t, got.Decl.ServiceAccounts, 1)
	assert.Empty(t, got.Decl.ServiceAccounts[0].ExtraClusterRoles, "no entry without rules")
	assert.Equal(t, []string{"d8:m:m:iop:istiod-1x25"}, got.Decl.ServiceAccounts[0].BindClusterRoles)
	assert.Empty(t, rbacyaml.Validate(got.Decl, rbacyaml.CRDScopes{"x.io/things": "Namespaced"}))
}

// What the format still does not carry is noted per object: the labels and annotations of a
// legacy role or a capability, and the condition of an object only some render variants showed
// when the template did not give it (review of #479, finding 40). An account carries its own.
func TestBuild_DroppedMetadataAndConditionsAreNoted(t *testing.T) {
	labels := map[string]string{"module": "m", "heritage": "deckhouse"}

	got := Build(Input{Module: "m", Namespace: "d8-m", Objects: []Object{
		{Kind: "ServiceAccount", Name: "m", Namespace: "d8-m", Path: "templates/rbac-for-us.yaml", Labels: labels, Located: true,
			Annotations: map[string]string{"helm.sh/resource-policy": "keep", "meta.helm.sh/release-name": "m"}},
		{Kind: "ClusterRole", Name: "d8:m:user", Path: "templates/user-authz-cluster-roles.yaml", Located: true,
			Labels:      map[string]string{"module": "m", "gatekeeper.sh/system": "yes"},
			Annotations: map[string]string{rbaccontract.AccessLevelAnnotation: "User", "werf.io/deploy-on": "pre-install"},
			Rules:       []rbacv1.PolicyRule{{APIGroups: []string{"x.io"}, Resources: []string{"things"}, Verbs: []string{"get"}}}},
		{Kind: "ClusterRole", Name: "d8:m:admin", Path: "templates/user-authz-cluster-roles.yaml", Located: true, When: ".Values.m.on",
			Labels:      map[string]string{"module": "m"},
			Annotations: map[string]string{rbaccontract.AccessLevelAnnotation: "Admin"},
			Rules:       []rbacv1.PolicyRule{{APIGroups: []string{"x.io"}, Resources: []string{"things"}, Verbs: []string{"delete"}}}},
	}, CRDs: map[string]string{"x.io/things": "Namespaced"}, Partial: []string{"ClusterRole//d8:m:user", "ClusterRole//d8:m:admin"}})

	notes := strings.Join(got.Notes, "\n")
	assert.Contains(t, notes, "ClusterRole d8:m:user (templates/user-authz-cluster-roles.yaml) carries what the format does not describe (label gatekeeper.sh/system, annotation werf.io/deploy-on)")
	// A legacy role only some variants rendered, its condition unknown, leaves a TODO reason
	// (review of #479, finding 41).
	require.Len(t, got.Decl.Resources, 1)
	assert.Contains(t, got.Decl.Resources[0].Reason, "ClusterRole d8:m:user renders only under some of the linted values")
	assert.NotContains(t, got.Decl.Resources[0].Reason, "ClusterRole d8:m:admin renders only under some", "its condition was read from the template")
	assert.NotContains(t, notes, "ServiceAccount m (templates/rbac-for-us.yaml) carries", "an account carries its annotations")

	require.Len(t, got.Decl.ServiceAccounts, 1)
	assert.Equal(t, map[string]string{"helm.sh/resource-policy": "keep"}, got.Decl.ServiceAccounts[0].Annotations)
}

// An object of the module in a file that also holds what a helm_lib include renders stays
// hand-written: the generator writes the whole file (review of #479, finding 42). A scrape Role
// only some variants render stays hand-written too: prometheusAccess gates only the binding
// (finding 41).
func TestBuild_LibraryFileAndPartialScrapeAccess(t *testing.T) {
	labels := map[string]string{"module": "m"}
	metrics := []rbacv1.PolicyRule{{APIGroups: []string{"apps"}, Resources: []string{"deployments/prometheus-metrics"}, ResourceNames: []string{"m"}, Verbs: []string{"get"}}}

	got := Build(Input{Module: "m", Namespace: "d8-m", Objects: []Object{
		{Kind: "ClusterRole", Name: "d8:m:csi", Path: "templates/csi/rbac-for-us.yaml", Labels: labels, LibraryFile: true,
			Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"get"}}}},
		{Kind: "ClusterRoleBinding", Name: "d8:m:csi", Path: "templates/csi/rbac-for-us.yaml", Labels: labels, LibraryFile: true,
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "d8:m:csi"}, Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "csi", Namespace: "d8-m"}}},
		{Kind: "Role", Name: "access-to-m-prometheus-metrics", Namespace: "d8-m", Path: "templates/rbac-to-us.yaml", Labels: labels, Rules: metrics},
		{Kind: "RoleBinding", Name: "access-to-m-prometheus-metrics", Namespace: "d8-m", Path: "templates/rbac-to-us.yaml", Labels: labels,
			RoleRef: rbacv1.RoleRef{Kind: "Role", Name: "access-to-m-prometheus-metrics"}, Subjects: []rbacv1.Subject{{Kind: "User", Name: "d8-monitoring:scraper"}}},
	}, Partial: []string{"Role/d8-m/access-to-m-prometheus-metrics"}})

	unmanaged := strings.Join(got.Unmanaged, "\n")
	assert.Contains(t, unmanaged, "ClusterRole/d8:m:csi (templates/csi/rbac-for-us.yaml): shares templates/csi/rbac-for-us.yaml with objects a helm_lib include renders")
	assert.Contains(t, unmanaged, "ClusterRoleBinding/d8:m:csi (templates/csi/rbac-for-us.yaml): shares")
	assert.Contains(t, unmanaged, "d8-m/Role/access-to-m-prometheus-metrics (templates/rbac-to-us.yaml): renders only under some of the linted values")
	assert.Empty(t, got.Decl.Access)
	assert.Nil(t, got.Decl.PrometheusAccess)
}

// A capability only some variants rendered leaves a TODO reason on what it grants: resources[] have
// no `when` (review of #479, finding 41).
func TestBuild_PartialCapabilityIsATODO(t *testing.T) {
	labels := map[string]string{"module": "m", rbaccontract.LabelKind: rbaccontract.KindCapability}

	got := Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, CRDs: map[string]string{"x.io/things": "Namespaced"}, Objects: []Object{
		{Kind: "ClusterRole", Name: "d8:namespace-capability:m:view", Path: "templates/rbacv2/use/view.yaml", Labels: labels,
			Rules: []rbacv1.PolicyRule{{APIGroups: []string{"x.io"}, Resources: []string{"things"}, Verbs: []string{"get"}}}},
	}, Partial: []string{"ClusterRole//d8:namespace-capability:m:view"}})

	require.Len(t, got.Decl.Resources, 1)
	assert.True(t, strings.HasPrefix(got.Decl.Resources[0].Reason, "TODO: ClusterRole d8:namespace-capability:m:view renders only under some of the linted values"), got.Decl.Resources[0].Reason)
}

// A partially rendered object kept hand-written in a file the declaration also writes is named
// with the way out (review of #479, finding 48).
func TestBuild_PartialObjectBesideDeclaredOnes(t *testing.T) {
	labels := map[string]string{"module": "m"}
	nodes := []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"get"}}}

	got := Build(Input{Module: "m", Namespace: "d8-m", Objects: []Object{
		{Kind: "ServiceAccount", Name: "m", Namespace: "d8-m", Path: "templates/rbac-for-us.yaml", Labels: labels},
		{Kind: "ClusterRole", Name: "d8:m:supplement", Path: "templates/rbac-for-us.yaml", Labels: labels, Rules: nodes},
		{Kind: "ClusterRoleBinding", Name: "d8:m:supplement", Path: "templates/rbac-for-us.yaml", Labels: labels,
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "d8:m:supplement"}, Subjects: []rbacv1.Subject{{Kind: "Group", Name: "g"}}},
	}, Partial: []string{"ClusterRoleBinding//d8:m:supplement"}})

	notes := strings.Join(got.Notes, "\n")
	assert.Contains(t, notes, "ClusterRoleBinding d8:m:supplement stays hand-written in templates/rbac-for-us.yaml, which the declaration also writes: move it to the rbac-for-us.yaml of another component directory")
	assert.Contains(t, notes, "ClusterRole d8:m:supplement stays hand-written in templates/rbac-for-us.yaml")
}

// An account of a component directory in kube-system stays hand-written with what binds it: the
// generator refuses it, and the declaration would be refused whole (review of #479, finding 49).
func TestBuild_KubeSystemComponentAccountIsSetAside(t *testing.T) {
	labels := map[string]string{"module": "m"}
	sa := []rbacv1.Subject{{Kind: "ServiceAccount", Name: "proxy", Namespace: "kube-system"}}

	got := Build(Input{Module: "m", Namespace: "kube-system", Objects: []Object{
		{Kind: "ServiceAccount", Name: "proxy", Namespace: "kube-system", Path: "templates/proxy/rbac-for-us.yaml", Labels: labels},
		{Kind: "ClusterRole", Name: "d8:m:proxy", Path: "templates/proxy/rbac-for-us.yaml", Labels: labels,
			Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"get"}}}},
		{Kind: "ClusterRoleBinding", Name: "d8:m:proxy", Path: "templates/proxy/rbac-for-us.yaml", Labels: labels,
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "d8:m:proxy"}, Subjects: sa},
	}})

	assert.Empty(t, got.Decl.ServiceAccounts)
	assert.Empty(t, got.Decl.Access)
	assert.Len(t, got.Unmanaged, 3, "got: %v", got.Unmanaged)
	assert.Contains(t, strings.Join(got.Unmanaged, "\n"), `in kube-system the placement rule wants the account named "d8-m-proxy"`)
}

// When only an object of an account renders in some variants, the TODO does not invite narrowing
// the account itself (review of #479, finding 51).
func TestBuild_PartialObjectOfAnAlwaysRenderedAccount(t *testing.T) {
	labels := map[string]string{"module": "m"}

	got := Build(Input{Module: "m", Namespace: "d8-m", Objects: []Object{
		{Kind: "ServiceAccount", Name: "m", Namespace: "d8-m", Path: "templates/rbac-for-us.yaml", Labels: labels},
		{Kind: "ClusterRoleBinding", Name: "d8:m:m:rbac-proxy", Path: "templates/rbac-for-us.yaml", Labels: labels,
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "d8:rbac-proxy"}, Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "m", Namespace: "d8-m"}}},
	}, Partial: []string{"ClusterRoleBinding//d8:m:m:rbac-proxy"}})

	require.Len(t, got.Decl.ServiceAccounts, 1)
	assert.Contains(t, got.Decl.ServiceAccounts[0].When, "the account always")
}

// A Role two accounts bind is neither account's own: it is not absorbed into one account's
// namespaceRules, whatever order the accounts come in, and the hand-written binding of the other
// keeps pointing at a Role that stays (review of #479, finding 53).
func TestBuild_SharedRoleIsNoAccountsOwn(t *testing.T) {
	labels := map[string]string{"module": "m"}
	secrets := []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}}}

	for _, names := range [][2]string{{"a", "z"}, {"z", "a"}} {
		declared, other := names[0], names[1]

		got := Build(Input{Module: "m", Namespace: "d8-m", Objects: []Object{
			{Kind: "ServiceAccount", Name: declared, Namespace: "d8-m", Path: "templates/rbac-for-us.yaml", Labels: labels},
			{Kind: "ServiceAccount", Name: other, Namespace: "d8-other", Path: "templates/rbac-for-us.yaml", Labels: labels},
			{Kind: "Role", Name: "shared", Namespace: "d8-m", Path: "templates/rbac-for-us.yaml", Labels: labels, Rules: secrets},
			{Kind: "RoleBinding", Name: declared, Namespace: "d8-m", Path: "templates/rbac-for-us.yaml", Labels: labels,
				RoleRef: rbacv1.RoleRef{Kind: "Role", Name: "shared"}, Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: declared, Namespace: "d8-m"}}},
			{Kind: "RoleBinding", Name: other, Namespace: "d8-m", Path: "templates/rbac-for-us.yaml", Labels: labels,
				RoleRef: rbacv1.RoleRef{Kind: "Role", Name: "shared"}, Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: other, Namespace: "d8-other"}}},
		}})

		require.Len(t, got.Decl.ServiceAccounts, 1, names)
		assert.Empty(t, got.Decl.ServiceAccounts[0].NamespaceRules, "%v: the shared Role is not absorbed", names)
		assert.Contains(t, strings.Join(got.Unmanaged, "\n"), "d8-m/Role/shared", names)
	}
}

// Objects of one account that render in other variants than the account -- the cluster-autoscaler
// shape, two mutually exclusive sets -- get a TODO that says no single `when` holds them, rather
// than asking for one (review of #479, finding 52).
func TestBuild_AccountObjectsInOtherVariants(t *testing.T) {
	labels := map[string]string{"module": "m"}
	sa := []rbacv1.Subject{{Kind: "ServiceAccount", Name: "autoscaler", Namespace: "d8-m"}}

	got := Build(Input{Module: "m", Namespace: "d8-m", Objects: []Object{
		{Kind: "ServiceAccount", Name: "autoscaler", Namespace: "d8-m", Path: "templates/autoscaler/rbac-for-us.yaml", Labels: labels},
		{Kind: "ClusterRoleBinding", Name: "d8:m:autoscaler:plain", Path: "templates/autoscaler/rbac-for-us.yaml", Labels: labels,
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "d8:rbac-proxy"}, Subjects: sa},
		{Kind: "ClusterRoleBinding", Name: "d8:m:autoscaler:mcm", Path: "templates/autoscaler/rbac-for-us.yaml", Labels: labels,
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "d8:mcm"}, Subjects: sa},
	},
		Partial: []string{"ClusterRoleBinding//d8:m:autoscaler:plain", "ClusterRoleBinding//d8:m:autoscaler:mcm"},
		Variants: map[string]string{
			"ServiceAccount/d8-m/autoscaler":            "1,2,",
			"ClusterRoleBinding//d8:m:autoscaler:plain": "1,",
			"ClusterRoleBinding//d8:m:autoscaler:mcm":   "2,",
		}})

	require.Len(t, got.Decl.ServiceAccounts, 1)
	when := got.Decl.ServiceAccounts[0].When
	assert.True(t, strings.HasPrefix(when, "TODO: "), when)
	assert.Contains(t, when, "render in other variants than ServiceAccount autoscaler, so no single `when` holds")
	assert.Contains(t, when, "ClusterRoleBinding d8:m:autoscaler:plain")
	assert.Contains(t, when, "ClusterRoleBinding d8:m:autoscaler:mcm")
}
