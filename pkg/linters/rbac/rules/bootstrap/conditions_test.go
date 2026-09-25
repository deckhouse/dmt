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
	"io"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbaccontract"
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
	assert.Equal(t, "and (.Values.m.internal.enabled) (not (.Values.m.extra))", byName["ClusterRole/d8:m:otherwise"].When)
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

// The conditions bootstrap writes render: every argument of and is parenthesized, a negation
// included, and a condition over several lines is joined (regression hunt 2, B1 and B2).
func TestTemplateDocs_ConditionsRender(t *testing.T) {
	text := "{{- if .Values.a }}\n---\nkind: Role\nmetadata:\n  name: first\n{{- else if .Values.b }}\n---\nkind: Role\nmetadata:\n  name: second\n" +
		"{{- else }}\n---\nkind: Role\nmetadata:\n  name: third\n{{- end }}\n" +
		"{{- if and\n    .Values.c\n    (not .Values.d) }}\n---\nkind: Role\nmetadata:\n  name: fourth\n{{- end }}\n"

	want := map[string]string{
		"first":  ".Values.a",
		"second": "and (not (.Values.a)) (.Values.b)",
		"third":  "and (not (.Values.a)) (not (.Values.b))",
		"fourth": "and .Values.c (not .Values.d)",
	}

	values := map[string]any{"Values": map[string]any{"a": false, "b": true, "c": true, "d": false}}

	for _, d := range TemplateDocs(text) {
		require.Contains(t, want, d.Name)
		assert.Equal(t, want[d.Name], d.When, d.Name)

		tpl, err := template.New(d.Name).Parse("{{ if " + d.When + " }}yes{{ end }}")
		require.NoError(t, err, d.When)
		require.NoError(t, tpl.Execute(io.Discard, values), d.When)
	}
}

// What the scanner reads besides: a quoted }} in a condition, a commented-out object, an include
// with comments beside it, a kind or name with a comment or quotes, a computed name, a block
// that stays open into the next document (regression hunt 2, B3, B4, B5, B8, B9).
func TestTemplateDocs_Shapes(t *testing.T) {
	text := `{{- if eq .Values.x "}}" }}
---
kind: "Role" # the role
metadata:
  name: quoted # a comment
{{- end }}
{{/*
---
kind: ClusterRole
metadata:
  name: commented
*/}}
---
{{ include "helm_lib_csi_controller_rbac" . }}
# =========================================
---
kind: ServiceAccount
metadata:
  name: {{ .Chart.Name }}-extra
---
kind: ServiceAccount
metadata:
  name: a
{{- if .Values.b }}
---
kind: ServiceAccount
metadata:
  name: b
{{- end }}
`
	docs := TemplateDocs(text)

	byName := map[string]Doc{}
	library := 0

	for _, d := range docs {
		byName[d.Kind+"/"+d.Name] = d
		if d.Library {
			library++
		}
	}

	assert.True(t, strings.HasPrefix(byName["Role/quoted"].When, `TODO: eq .Values.x "}}" -- the template condition holds a template delimiter`), "read to the end, not cut at the quoted }}: %s", byName["Role/quoted"].When)
	assert.NotContains(t, byName, "ClusterRole/commented")
	assert.Equal(t, 1, library, "an include beside comments is still the library's document")
	assert.False(t, byName["ServiceAccount/a"].Partial, "the if after it belongs to the next document")
	assert.Equal(t, ".Values.b", byName["ServiceAccount/b"].When)

	d, ok := Locate(docs, Object{Kind: "ServiceAccount", Name: "cert-manager-extra"})
	require.True(t, ok)
	assert.Empty(t, d.Name)
	assert.NotNil(t, d.NamePattern)

	withoutLibrary := make([]Doc, 0, len(docs))
	for _, d := range docs {
		if !d.Library {
			withoutLibrary = append(withoutLibrary, d)
		}
	}

	_, ok = Locate(withoutLibrary, Object{Kind: "ServiceAccount", Name: "unrelated"})
	assert.False(t, ok, "a computed name matches only what it can produce")
}

// A note quoting a condition over several lines stays a comment, and the file parses
// (regression hunt 2, B2); an account outside the module namespace is listed once (B10).
func TestMarshal_MultilineNoteAndSingleListing(t *testing.T) {
	got := Build(Input{Module: "m", Namespace: "d8-m", Objects: []Object{
		{Kind: "ServiceAccount", Name: "elsewhere", Namespace: "kube-system", Path: "templates/rbac-for-us.yaml", Labels: map[string]string{"module": "m"}},
	}, Unrendered: []string{"Role/r (templates/x.yaml, under `and\n  (include \"a\" .)\n  .Values.b`)"}})

	assert.Len(t, got.Unmanaged, 1, "got: %v", got.Unmanaged)

	content, err := Marshal(got)
	require.NoError(t, err)

	_, err = rbacyaml.Parse(content)
	require.NoError(t, err, string(content))

	for _, line := range strings.Split(strings.TrimSpace(strings.SplitN(string(content), "apiVersion:", 2)[0]), "\n") {
		assert.True(t, strings.HasPrefix(line, "#"), "a header line that is no comment: %q", line)
	}
}

// The template is read with text/template/parse: metadata at any indentation or in flow style,
// string literals kept as written, an if without a space, a define (review of #479, finding 35).
func TestTemplateDocs_Parser(t *testing.T) {
	text := `{{- if eq .Values.m.mode "a  b" }}
---
kind: Role
metadata:
    name: four-spaces
    namespace: d8-m
{{- end }}
{{if(.Values.m.flow)}}
---
kind: Role
metadata: {name: flow, namespace: d8-m}
{{end}}
---
kind: ServiceAccount
metadata:
  labels:
    name: not-the-name
{{- define "m.helper" }}
---
kind: Role
metadata:
  name: defined
{{- end }}
---
{{ include "helm_lib_csi_controller_rbac" . }}
`
	docs := TemplateDocs(text)

	byName := map[string]Doc{}
	for _, d := range docs {
		byName[d.Kind+"/"+d.Name] = d
	}

	assert.Equal(t, `eq .Values.m.mode "a  b"`, byName["Role/four-spaces"].When, "the literal keeps its two spaces")
	assert.Equal(t, "d8-m", byName["Role/four-spaces"].Namespace)
	assert.Equal(t, "(.Values.m.flow)", byName["Role/flow"].When)
	assert.Equal(t, "d8-m", byName["Role/flow"].Namespace)
	assert.Contains(t, byName["Role/defined"].Unmanageable, "define")

	sa, ok := byName["ServiceAccount/"]
	require.True(t, ok)
	assert.Nil(t, sa.NamePattern, "a name nested under labels is no name")

	// A document without a readable name matches nothing: the object the include renders is the
	// library's, not that document's.
	d, ok := Locate(docs, Object{Kind: "ServiceAccount", Name: "csi"})
	require.True(t, ok)
	assert.True(t, d.Library)

	// A template that does not parse yields nothing rather than a guess.
	assert.Empty(t, TemplateDocs("{{ if }"))
}

// A capability or a legacy role under a condition leaves a TODO reason on the resources it grants:
// resources[] have no `when`, and a regeneration would grant them for every value (review of #479,
// finding 33).
func TestBuild_ConditionalCapabilityIsATODO(t *testing.T) {
	labels := map[string]string{"module": "m", rbaccontract.LabelKind: rbaccontract.KindCapability}

	got := Build(Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}, CRDs: map[string]string{"x.io/things": "Namespaced", "x.io/others": "Namespaced"}, Objects: []Object{
		{Kind: "ClusterRole", Name: "d8:namespace-capability:m:view", Path: "templates/rbacv2/use/view.yaml", Labels: labels, Located: true, When: ".Values.m.extra",
			Rules: []rbacv1.PolicyRule{{APIGroups: []string{"x.io"}, Resources: []string{"things"}, Verbs: []string{"get"}}}},
		{Kind: "ClusterRole", Name: "d8:namespace-capability:m:edit", Path: "templates/rbacv2/use/edit.yaml", Labels: labels, Located: true,
			Rules: []rbacv1.PolicyRule{{APIGroups: []string{"x.io"}, Resources: []string{"others"}, Verbs: []string{"create"}}}},
	}})

	byKey := map[string]rbacyaml.Resource{}
	for _, r := range got.Decl.Resources {
		byKey[r.Key()] = r
	}

	assert.True(t, strings.HasPrefix(byKey["x.io/things"].Reason, "TODO: ClusterRole d8:namespace-capability:m:view renders under `.Values.m.extra`"), byKey["x.io/things"].Reason)
	assert.Empty(t, byKey["x.io/others"].Reason)
}

// Several library documents are one answer only when their conditions agree: an include under a
// condition beside one without gives its objects no condition otherwise (review of #480).
func TestLocate_LibraryDocumentsMustAgree(t *testing.T) {
	docs := TemplateDocs(`{{ include "helm_lib_cloud_provider_user_authz_cluster_roles" . }}
---
{{- if .Values.x }}
{{ include "helm_lib_other_roles" . }}
{{- end }}
`)

	library := 0

	for _, d := range docs {
		if d.Library {
			library++
		}
	}

	require.Equal(t, 2, library)

	// Disagreeing, they still say a library renders the object, its condition unknown.
	d, ok := Locate(docs, Object{Kind: "ClusterRole", Name: "d8:m:user"})
	require.True(t, ok)
	assert.True(t, d.Library)
	assert.Equal(t, "rendered by includes of named templates under different conditions (no condition; `.Values.x`), so its condition is unknown", d.Unmanageable)
	assert.True(t, strings.HasPrefix(d.When, "TODO: "), d.When)

	agreeing := TemplateDocs("{{ include \"a\" . }}\n---\n{{ include \"b\" . }}\n")
	d, ok = Locate(agreeing, Object{Kind: "ClusterRole", Name: "d8:m:user"})
	require.True(t, ok)
	assert.True(t, d.Library)
	assert.Empty(t, d.When)
	assert.Empty(t, d.Unmanageable)
}
