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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/generate"
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

	assert.Contains(t, joined, "trivy.deckhouse.io/vulnerabilityreports: the module ships no CRD and the scope is not known; fill scope")
	assert.Contains(t, joined, "ServiceAccount webhook mounted its token")

	byKey := map[string]rbacyaml.Resource{}
	for _, r := range got.Decl.Resources {
		byKey[r.Group+"/"+r.Resource] = r
	}

	assert.Equal(t, "Namespaced", byKey["/pods"].Scope, "a well-known core resource gets its scope")
	assert.Equal(t, "", byKey["trivy.deckhouse.io/vulnerabilityreports"].Scope, "an unknown one is left for the author")
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
	assert.Contains(t, got.Unmanaged[0]+got.Unmanaged[1], "d8:use:capability:module:m:view")
	assert.Contains(t, got.Unmanaged[0]+got.Unmanaged[1], "d8:namespace-capability:kubernetes:view_logs")
}
