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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbacyaml"
)

const conditionalTemplate = `{{- if .Values.m.internal.enabled }}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: plain
  namespace: d8-m
{{- if .Values.m.extra }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:m:nested
rules: []
{{- else }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:m:otherwise
rules:
{{- if .Values.m.more }}
- apiGroups: [""]
  resources: [pods]
  verbs: [get]
{{- end }}
{{- end }}
{{- end }}
{{- range $name := .Values.m.names }}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ $name }}
{{- end }}
{{- $ns := .Values.m.ns }}
{{- if $ns }}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: by-variable
{{- end }}
---
{{ include "helm_lib_kube_rbac_proxy" . }}
`

// The template blocks around an object become its `when`, or the reason the declaration cannot
// carry them (regression hunt, B1 and B6).
func TestTemplateDocs(t *testing.T) {
	docs := TemplateDocs(conditionalTemplate)

	byName := map[string]Doc{}
	for _, d := range docs {
		byName[d.Kind+"/"+d.Name] = d
	}

	assert.Equal(t, ".Values.m.internal.enabled", byName["ServiceAccount/plain"].When)
	assert.Equal(t, "d8-m", byName["ServiceAccount/plain"].Namespace)
	assert.Equal(t, "and (.Values.m.internal.enabled) (.Values.m.extra)", byName["ClusterRole/d8:m:nested"].When)
	assert.Equal(t, "and (.Values.m.internal.enabled) not (.Values.m.extra)", byName["ClusterRole/d8:m:otherwise"].When)
	assert.True(t, byName["ClusterRole/d8:m:otherwise"].Partial, "a rule under its own block")
	assert.False(t, byName["ClusterRole/d8:m:nested"].Partial)
	assert.Contains(t, byName["ServiceAccount/"].Unmanageable, "{{ range }}")
	assert.True(t, strings.HasPrefix(byName["ServiceAccount/by-variable"].When, "TODO: $ns"), byName["ServiceAccount/by-variable"].When)

	var library []Doc

	for _, d := range docs {
		if d.Library {
			library = append(library, d)
		}
	}

	require.Len(t, library, 1)
	assert.Empty(t, library[0].When)

	// The written condition is a valid `when`.
	decl := &rbacyaml.Declaration{APIVersion: rbacyaml.APIVersionV1Alpha1, ServiceAccounts: []rbacyaml.ServiceAccount{{Name: "m", When: byName["ClusterRole/d8:m:otherwise"].When}}}
	assert.Empty(t, rbacyaml.Validate(decl, nil))
}

func TestLocate(t *testing.T) {
	docs := TemplateDocs(conditionalTemplate)

	d, ok := Locate(docs, Object{Kind: "ClusterRole", Name: "d8:m:nested"})
	require.True(t, ok)
	assert.Equal(t, "and (.Values.m.internal.enabled) (.Values.m.extra)", d.When)

	// A name the template computes matches the templated document of the kind.
	d, ok = Locate(docs, Object{Kind: "ServiceAccount", Name: "from-values"})
	require.True(t, ok)
	assert.Contains(t, d.Unmanageable, "range")

	// An object of a kind the text does not hold came from the include.
	d, ok = Locate(docs, Object{Kind: "ClusterRoleBinding", Name: "d8:m:rbac-proxy"})
	require.True(t, ok)
	assert.True(t, d.Library)

	_, ok = Locate(nil, Object{Kind: "Role", Name: "x"})
	assert.False(t, ok)
}

// The conditions reach the declaration: an account and its objects under one condition, an access
// entry, the scraper gate; a mismatch is a TODO (regression hunt, B1, B9 and B10).
func TestBuild_TemplateConditions(t *testing.T) {
	labels := map[string]string{"module": "m"}
	sa := []rbacv1.Subject{{Kind: "ServiceAccount", Name: "m", Namespace: "d8-m"}}
	scraper := []rbacv1.Subject{{Kind: "User", Name: "d8-monitoring:scraper"}, {Kind: "ServiceAccount", Name: "prometheus", Namespace: "d8-monitoring"}}
	nodes := []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"get"}}}
	metrics := []rbacv1.PolicyRule{{APIGroups: []string{"apps"}, Resources: []string{"deployments/prometheus-metrics"}, ResourceNames: []string{"m"}, Verbs: []string{"get"}}}
	gate := `.Values.global.enabledModules | has "prometheus"`

	got := Build(Input{Module: "m", Namespace: "d8-m", Objects: []Object{
		{Kind: "ServiceAccount", Name: "m", Path: "templates/rbac-for-us.yaml", Labels: labels, When: ".Values.m.on", Located: true},
		{Kind: "ClusterRole", Name: "d8:m:m", Path: "templates/rbac-for-us.yaml", Labels: labels, Rules: nodes, When: ".Values.m.on", Located: true},
		{Kind: "ClusterRoleBinding", Name: "d8:m:m", Path: "templates/rbac-for-us.yaml", Labels: labels, When: ".Values.m.on", Located: true,
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "d8:m:m"}, Subjects: sa},
		{Kind: "ClusterRole", Name: "d8:m:reader", Path: "templates/rbac-for-us.yaml", Labels: labels, Rules: nodes, When: ".Values.m.reader", Located: true},
		{Kind: "ClusterRoleBinding", Name: "d8:m:reader", Path: "templates/rbac-for-us.yaml", Labels: labels, Located: true,
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "d8:m:reader"}, Subjects: []rbacv1.Subject{{Kind: "Group", Name: "g"}}},
		{Kind: "ClusterRole", Name: "d8:m:empty", Path: "templates/rbac-for-us.yaml", Labels: labels, Located: true},
		{Kind: "ClusterRoleBinding", Name: "d8:m:empty", Path: "templates/rbac-for-us.yaml", Labels: labels, Located: true,
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "d8:m:empty"}, Subjects: []rbacv1.Subject{{Kind: "Group", Name: "g"}}},
		{Kind: "Role", Name: "access-to-m-prometheus-metrics", Namespace: "d8-m", Path: "templates/rbac-to-us.yaml", Labels: labels, Rules: metrics, Located: true},
		{Kind: "RoleBinding", Name: "access-to-m-prometheus-metrics", Namespace: "d8-m", Path: "templates/rbac-to-us.yaml", Labels: labels, When: gate, Located: true,
			RoleRef: rbacv1.RoleRef{Kind: "Role", Name: "access-to-m-prometheus-metrics"}, Subjects: scraper},
		{Kind: "ServiceAccount", Name: "looped", Path: "templates/rbac-for-us.yaml", Labels: labels, Unmanageable: "rendered inside {{ range }}, which the declaration cannot express", Located: true},
	}})

	require.Len(t, got.Decl.ServiceAccounts, 1)
	assert.Equal(t, ".Values.m.on", got.Decl.ServiceAccounts[0].When)

	require.Len(t, got.Decl.Access, 1)
	assert.Equal(t, "reader", got.Decl.Access[0].Name)
	assert.True(t, strings.HasPrefix(got.Decl.Access[0].When, "TODO: ClusterRole d8:m:reader renders under `.Values.m.reader`"), got.Decl.Access[0].When)

	require.NotNil(t, got.Decl.PrometheusAccess)
	assert.Equal(t, gate, got.Decl.PrometheusAccess.When)
	assert.NotContains(t, strings.Join(got.Notes, "\n"), "whether it gated the scraper binding")

	unmanaged := strings.Join(got.Unmanaged, "\n")
	assert.Contains(t, unmanaged, "ServiceAccount/looped (templates/rbac-for-us.yaml): rendered inside {{ range }}")
	assert.Contains(t, unmanaged, "ClusterRole/d8:m:empty (templates/rbac-for-us.yaml): has no rules")
	assert.Contains(t, unmanaged, "ClusterRoleBinding/d8:m:empty (templates/rbac-for-us.yaml): binds d8:m:empty, which is not a plain ClusterRole of this module the declaration describes")
}

// An account whose role renders under another condition than the account holds a TODO.
func TestBuild_AccountObjectsUnderAnotherCondition(t *testing.T) {
	labels := map[string]string{"module": "m"}
	sa := []rbacv1.Subject{{Kind: "ServiceAccount", Name: "m", Namespace: "d8-m"}}

	got := Build(Input{Module: "m", Namespace: "d8-m", Objects: []Object{
		{Kind: "ServiceAccount", Name: "m", Path: "templates/rbac-for-us.yaml", Labels: labels, Located: true},
		{Kind: "ClusterRoleBinding", Name: "d8:m:m:rbac-proxy", Path: "templates/rbac-for-us.yaml", Labels: labels, When: ".Values.m.proxy", Located: true,
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "d8:rbac-proxy"}, Subjects: sa},
	}})

	require.Len(t, got.Decl.ServiceAccounts, 1)
	assert.Equal(t, "TODO: the account renders under `no condition`, but ClusterRoleBinding d8:m:m:rbac-proxy renders under `.Values.m.proxy`; the declaration puts all of them under one condition", got.Decl.ServiceAccounts[0].When)
}

func TestUnwrap(t *testing.T) {
	assert.Equal(t, `.Values.global.enabledModules | has "prometheus"`, unwrap(`(.Values.global.enabledModules | has "prometheus")`))
	assert.Equal(t, `(a) (b)`, unwrap(`(a) (b)`))
	assert.Equal(t, `a`, unwrap(`((a))`))
	assert.Equal(t, `.Values.x`, unwrap(`.Values.x`))

	_, ok := Locate(TemplateDocs("{{- if (.Values.a) }}\n---\nkind: Role\nmetadata:\n  name: r\n{{- if .Values.b }}\n---\nkind: Role\nmetadata:\n  name: r\n{{- end }}\n{{- end }}\n"), Object{Kind: "Role", Name: "r"})
	assert.False(t, ok, "two documents of one name under different conditions are not one answer")
}
