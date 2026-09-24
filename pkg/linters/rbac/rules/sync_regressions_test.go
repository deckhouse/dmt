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

// Regression probes from the fourth review of deckhouse/dmt#479: each reproduces a way a --fix could
// lose, duplicate or misreport an object.
package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/bootstrap"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/generate"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbacyaml"
)

func probeFixMessages(list *errors.LintRuleErrorsList) string {
	var out []string

	for _, e := range list.GetErrors() {
		if e.FixError != nil {
			out = append(out, e.FixError.Error())
		}
	}

	return strings.Join(out, "\n")
}

// P1: a hand-added object under the file's false condition that textDocuments cannot see is
// dropped by the regeneration (no declaration change needed: the text edit is the divergence).
func TestSyncRegression_TextDocumentsBlindSpots(t *testing.T) {
	const rel = "templates/cainjector/rbac-for-us.yaml"

	const cm = "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cainjector-extra\n  namespace: d8-cert-manager\n"

	for name, mutate := range map[string]func(string) string{
		"control: plain ConfigMap document (refused today)": func(s string) string {
			return strings.Replace(s, "{{- end }}\n", "---\n"+cm+"{{- end }}\n", 1)
		},
		"document is an include": func(s string) string {
			return strings.Replace(s, "{{- end }}\n", "---\n{{ include \"cainjector-extra\" . }}\n{{- end }}\n", 1)
		},
		"kind line carries a comment": func(s string) string {
			return strings.Replace(s, "{{- end }}\n", "---\n"+strings.Replace(cm, "kind: ConfigMap\n", "kind: ConfigMap # hand-added\n", 1)+"{{- end }}\n", 1)
		},
		"separator carries a comment": func(s string) string {
			return strings.Replace(s, "{{- end }}\n", "--- # hand-added\n"+cm+"{{- end }}\n", 1)
		},
		"object before the first separator": func(s string) string {
			return strings.Replace(s, "{{- if .Values.certManager.internal.enableCAInjector }}\n", "{{- if .Values.certManager.internal.enableCAInjector }}\n"+cm, 1)
		},
		"sound: range with templated name": func(s string) string {
			return strings.Replace(s, "{{- end }}\n", "{{- range .Values.x }}\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cainjector-extra-{{ . }}\n{{- end }}\n{{- end }}\n", 1)
		},
		"sound: no metadata.name, no namespace, quoted": func(s string) string {
			return strings.Replace(s, "{{- end }}\n", "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  labels: {x: cainjector-extra}\n---\nkind: Role\nmetadata:\n  name: 'cainjector'\n{{- end }}\n", 1)
		},
		"CRLF line endings": func(s string) string {
			s = strings.Replace(s, "{{- end }}\n", "---\n"+cm+"{{- end }}\n", 1)
			return strings.ReplaceAll(s, "\n", "\r\n")
		},
	} {
		t.Run(name, func(t *testing.T) {
			resetFixState()
			t.Cleanup(resetFixState)

			modulePath := syncModuleDir(t)
			model := syncModel(t, modulePath)
			writeGenerated(t, modulePath, model)

			fullPath := filepath.Join(modulePath, rel)
			content, err := os.ReadFile(fullPath)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(fullPath, []byte(mutate(string(content))), 0o600))

			// Default values: the cainjector block does not render.
			store := renderedFrom(t, model, func(o *generate.Object) bool { return o.When == "" })

			list := runSync(t, modulePath, store)
			for _, fix := range list.GetFixes() {
				fix()
			}

			after, err := os.ReadFile(fullPath)
			require.NoError(t, err)
			assert.Contains(t, string(after), "cainjector-extra", "the hand-added object must survive --fix; fix errors: %s", probeFixMessages(list))
		})
	}
}

// P1b: the same blind spot on the orphan path deletes the whole file.
func TestSyncRegression_OrphanDeletesIncludeDocument(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	const rel = "templates/cainjector/rbac-for-us.yaml"

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	fullPath := filepath.Join(modulePath, rel)
	content, err := os.ReadFile(fullPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(fullPath, []byte(strings.Replace(string(content), "{{- end }}\n", "---\n{{ include \"cainjector-extra\" . }}\n{{- end }}\n", 1)), 0o600))

	decl, err := rbacyaml.Load(modulePath)
	require.NoError(t, err)

	decl.ServiceAccounts = nil
	raw, err := yaml.Marshal(decl)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), raw, 0o600))

	store := renderedFrom(t, model, func(o *generate.Object) bool { return o.When == "" })

	list := runSync(t, modulePath, store)
	for _, fix := range list.GetFixes() {
		fix()
	}

	_, err = os.Stat(fullPath)
	assert.NoError(t, err, "a file holding a hand-added include must not be deleted")
}

// P5: an object under a false `when` that the declaration moves to another file is refused in
// its source but written into its target: after --fix both templates define it.
func TestSyncRegression_UnrenderedMoveWritesTwice(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	decl, err := rbacyaml.Load(modulePath)
	require.NoError(t, err)

	decl.ServiceAccounts[0].Path = ""
	raw, err := yaml.Marshal(decl)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), raw, 0o600))

	// Default values: the cainjector account (under when) renders from nowhere.
	store := renderedFrom(t, model, func(o *generate.Object) bool { return o.When == "" })

	list := runSync(t, modulePath, store)
	t.Logf("findings:\n%s", strings.Join(texts(list), "\n"))

	for _, fix := range list.GetFixes() {
		fix()
	}

	t.Logf("fix errors:\n%s", probeFixMessages(list))

	const sa = "kind: ServiceAccount\nmetadata:\n  name: cainjector\n"

	source, err := os.ReadFile(filepath.Join(modulePath, "templates/cainjector/rbac-for-us.yaml"))
	require.NoError(t, err)

	target, err := os.ReadFile(filepath.Join(modulePath, "templates/rbac-for-us.yaml"))
	require.NoError(t, err)

	inSource, inTarget := strings.Contains(string(source), sa), strings.Contains(string(target), sa)
	assert.False(t, inSource && inTarget, "the account is defined in both templates after --fix")
}

// P6: a contract 1 capability file the declaration drops is announced as deleted by --fix, and
// the fix refuses: the text parse does not recognize the capability label the generator writes.
func TestSyncRegression_ContractOneCapabilityOrphanRefused(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	const rel = "templates/rbacv2/use/admin.yaml"

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)
	asContractOne(t, filepath.Join(modulePath, rel))

	store := renderedFrom(t, model, nil)

	decl, err := rbacyaml.Load(modulePath)
	require.NoError(t, err)

	for i := range decl.Resources {
		if decl.Resources[i].Resource == "issuers" {
			delete(decl.Resources[i].Namespace, "admin")
		}
	}

	delete(decl.Capabilities, "namespace.admin")

	raw, err := yaml.Marshal(decl)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), raw, 0o600))
	require.Nil(t, syncModel(t, modulePath).File(rel), "the declaration no longer produces the file")

	list := runSync(t, modulePath, store)
	joined := strings.Join(texts(list), "\n")
	require.Contains(t, joined, rel+" does not match rbac.yaml")
	t.Logf("findings:\n%s", joined)

	for _, fix := range list.GetFixes() {
		fix()
	}

	t.Logf("fix errors:\n%s", probeFixMessages(list))

	_, err = os.Stat(filepath.Join(modulePath, rel))
	assert.True(t, os.IsNotExist(err), "the finding says --fix deletes the file")
}

// P7: a `when` on deckhouseVersion that the declaration drops turns the generated file into a
// "gated" one: the fix refuses with a false reason.
func TestSyncRegression_DroppedVersionWhenLooksLikeAGate(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)

	decl, err := rbacyaml.Load(modulePath)
	require.NoError(t, err)

	decl.Resources[0].When = `semverCompare ">= 1.80" .Values.global.deckhouseVersion`
	raw, err := yaml.Marshal(decl)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), raw, 0o600))

	writeGenerated(t, modulePath, syncModel(t, modulePath))

	decl.Resources[0].When = ""
	raw, err = yaml.Marshal(decl)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), raw, 0o600))

	model := syncModel(t, modulePath)

	list := runSync(t, modulePath, renderedFrom(t, model, nil))
	for _, fix := range list.GetFixes() {
		fix()
	}

	msgs := probeFixMessages(list)
	t.Logf("fix errors:\n%s", msgs)
	assert.NotContains(t, msgs, "renders one of two role models", "the file never had a gate")
}

// P8: in a contract 1 file, a legacy role excluded from sync (exclude-rules.sync) is dropped by
// a regeneration, and the removal is neither reported nor logged.
func TestSyncRegression_ExcludedLegacyRoleDroppedFromContractOne(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	const rel = "templates/user-authz-cluster-roles.yaml"

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	fullPath := filepath.Join(modulePath, rel)
	asContractOne(t, fullPath)

	extra := generate.Object{Kind: "ClusterRole", Name: "d8:user-authz:cert-manager:kept-by-hand", Class: generate.ClassLegacy,
		Annotations: map[string]string{"user-authz.deckhouse.io/access-level": "User"},
		Rules:       []generate.Rule{{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}}}}}

	content, err := os.ReadFile(fullPath)
	require.NoError(t, err)

	doc := "---\napiVersion: rbac.authorization.k8s.io/v1\nkind: ClusterRole\nmetadata:\n  name: d8:user-authz:cert-manager:kept-by-hand\n  annotations:\n    user-authz.deckhouse.io/access-level: \"User\"\nrules:\n- apiGroups: [\"\"]\n  resources: [pods]\n  verbs: [get]\n"
	require.NoError(t, os.WriteFile(fullPath, append(content, []byte(doc)...), 0o600))

	store := renderedFrom(t, model, nil)
	putObject(t, store, rel, extra)

	list := runSync(t, modulePath, store, pkg.KindRuleExclude{Kind: "ClusterRole", Name: extra.Name})
	joined := strings.Join(texts(list), "\n")
	t.Logf("findings:\n%s", joined)

	for _, fix := range list.GetFixes() {
		fix()
	}

	t.Logf("fix errors:\n%s", probeFixMessages(list))

	after, err := os.ReadFile(fullPath)
	require.NoError(t, err)
	assert.Contains(t, string(after), "kept-by-hand", "an object excluded from sync is dropped by the fix without a word")
}

// P9: a regeneration that changes rights in ways other than "is in the render but not declared"
// logs no removal: on a --fix run the fixed finding is not printed, so the only trace is an Info line.
func TestSyncRegression_RightsChangesMissingFromRemovalLog(t *testing.T) {
	for name, tc := range map[string]struct {
		rel   string
		tweak func(o *generate.Object) bool
		after func(store map[string]any)
		name  string
	}{
		"automount token taken away": {
			rel: "templates/cainjector/rbac-for-us.yaml",
			tweak: func(o *generate.Object) bool {
				if o.Kind == "ServiceAccount" && o.Name == "cainjector" {
					yes := true
					o.AutomountToken = &yes
				}

				return true
			},
		},
		"legacy access level lowered": {
			rel: "templates/user-authz-cluster-roles.yaml",
			tweak: func(o *generate.Object) bool {
				if o.Name == "d8:user-authz:cert-manager:user" {
					o.Annotations = map[string]string{"user-authz.deckhouse.io/access-level": "Admin"}
				}

				return true
			},
		},
		"binding repointed": {
			rel: "templates/rbac-for-us.yaml",
			tweak: func(o *generate.Object) bool {
				if o.Kind == "ClusterRoleBinding" && o.Name == "d8:cert-manager:admin-kubeconfig" {
					o.RoleRefName = "cluster-admin"
				}

				return true
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			resetFixState()
			t.Cleanup(resetFixState)

			modulePath := syncModuleDir(t)
			model := syncModel(t, modulePath)
			writeGenerated(t, modulePath, model)

			// The template on disk is stale (a hand edit that the render shows).
			fullPath := filepath.Join(modulePath, tc.rel)
			content, err := os.ReadFile(fullPath)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(fullPath, append(content, []byte("# stale\n")...), 0o600))

			list := runSync(t, modulePath, renderedFrom(t, model, tc.tweak))
			t.Logf("findings:\n%s", strings.Join(texts(list), "\n"))

			assert.NotEmpty(t, recordedRemovals(fullPath), "the rights change is not among the removals the fix logs")
		})
	}
}

// An old copy that keeps rendering beside the object that replaced it is reported in its own file:
// the Prometheus Role bootstrap folds into access-to-<module> stays in the nested template
// otherwise, and the grant outlives its removal from the declaration.
func TestSyncRegression_ReplacedCopyIsReported(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	store := renderedFrom(t, model, nil)

	for _, o := range model.File("templates/rbac-to-us.yaml").Objects {
		old := o
		old.Name = "access-to-cert-manager-prometheus-metrics"

		if old.Kind == "RoleBinding" {
			old.RoleRefName = "access-to-cert-manager-prometheus-metrics"
		}

		putObject(t, store, "templates/cert-manager/rbac-to-us.yaml", old)
	}

	got := strings.Join(texts(runSync(t, modulePath, store)), "\n")
	assert.Contains(t, got, "templates/cert-manager/rbac-to-us.yaml does not match rbac.yaml: d8-cert-manager/Role/access-to-cert-manager-prometheus-metrics grants what d8-cert-manager/Role/access-to-cert-manager grants, and both render")
	assert.Contains(t, got, "d8-cert-manager/RoleBinding/access-to-cert-manager-prometheus-metrics binds access-to-cert-manager-prometheus-metrics, the old copy of d8-cert-manager/Role/access-to-cert-manager")
}

// What the linter would refuse in a written declaration is named by the fix that wrote it
// (regression hunt, B10).
func TestSyncRegression_WrittenProblems(t *testing.T) {
	in := bootstrap.Input{Module: "m", Namespace: "d8-m", Subsystems: []string{"security"}}

	assert.Empty(t, writtenProblems([]byte("apiVersion: rbac.deckhouse.io/v1alpha1\n"), nil, in))
	assert.Contains(t, writtenProblems([]byte("apiVersion: rbac.deckhouse.io/v1alpha1\nserviceAccounts:\n  - name: x\n    path: a/b\n"), nil, in),
		"one directory under templates/ only")
	assert.Contains(t, writtenProblems([]byte("apiVersion: rbac.deckhouse.io/v1alpha1\nserviceAccounts:\n  - name: x\n    when: .Values.x }}\n"), nil, in), "template delimiter")
	assert.Empty(t, writtenProblems([]byte("apiVersion: rbac.deckhouse.io/v1alpha1\nserviceAccounts:\n  - name: x\n    when: \"TODO: decide\"\n"), nil, in), "a TODO is counted on its own")
}

// An include inside an object's document, after its kind line, is someone else's too: the fix
// neither deletes the file (the account dropped) nor regenerates it without the include (nothing
// changed) (review of #479, finding 31).
func TestSyncRegression_IncludeInsideTheDocument(t *testing.T) {
	const rel = "templates/cainjector/rbac-for-us.yaml"

	inject := func(t *testing.T, modulePath string) string {
		t.Helper()

		fullPath := filepath.Join(modulePath, rel)
		content, err := os.ReadFile(fullPath)
		require.NoError(t, err)

		i := strings.LastIndex(string(content), "{{- end }}\n")
		require.GreaterOrEqual(t, i, 0)

		patched := string(content[:i]) + "{{- if .Values.handExtra }}\n{{ include \"cainjector-extra\" . }}\n{{- end }}\n" + string(content[i:])
		require.NoError(t, os.WriteFile(fullPath, []byte(patched), 0o600))

		return patched
	}

	t.Run("the account dropped: the file is not deleted", func(t *testing.T) {
		resetFixState()
		t.Cleanup(resetFixState)

		modulePath := syncModuleDir(t)
		model := syncModel(t, modulePath)
		writeGenerated(t, modulePath, model)
		inject(t, modulePath)

		decl, err := rbacyaml.Load(modulePath)
		require.NoError(t, err)

		decl.ServiceAccounts = nil
		raw, err := yaml.Marshal(decl)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(rbacyaml.Path(modulePath), raw, 0o600))

		list := runSync(t, modulePath, renderedFrom(t, model, nil))
		for _, fix := range list.GetFixes() {
			fix()
		}

		_, err = os.Stat(filepath.Join(modulePath, rel))
		assert.NoError(t, err, "a file holding a hand-added include must not be deleted")
	})

	t.Run("nothing changed: the include is not dropped", func(t *testing.T) {
		resetFixState()
		t.Cleanup(resetFixState)

		modulePath := syncModuleDir(t)
		model := syncModel(t, modulePath)
		writeGenerated(t, modulePath, model)
		patched := inject(t, modulePath)

		list := runSync(t, modulePath, renderedFrom(t, model, nil))
		for _, fix := range list.GetFixes() {
			fix()
		}

		got, err := os.ReadFile(filepath.Join(modulePath, rel))
		require.NoError(t, err)
		assert.Equal(t, patched, string(got))
	})
}

// The generator's labels line is recognized only in the shapes the generator writes: an include
// or labels from the values appended to it are someone else's (review of #479, finding 31).
func TestLabelsLineRe(t *testing.T) {
	for line, want := range map[string]bool{
		`  {{- include "helm_lib_module_labels" (list .) | nindent 2 }}`:                                      true,
		`  {{- include "helm_lib_module_labels" (list . (dict "app" "cainjector")) | nindent 2 }}`:            true,
		`  {{- include "helm_lib_module_labels" (list . (dict "a" "x \" y" "b" "z")) | nindent 2 }}`:          true,
		`  {{- include "helm_lib_module_labels" (list .) | nindent 2 }}{{ include "x" . | nindent 2 }}`:       false,
		`  {{- include "helm_lib_module_labels" (list . .Values.m.labels) | nindent 2 }}`:                     false,
		`  {{- include "helm_lib_module_labels" (list . (dict "app" .Values.m.app)) | nindent 2 }}`:           false,
		`  {{- include "helm_lib_module_labels" (list . (dict "app" "a")) | nindent 2 }} {{ include "x" . }}`: false,
	} {
		assert.Equal(t, want, labelsLineRe.MatchString(line), line)
	}
}

// An include appended to the generator's labels line is not dropped by a regeneration.
func TestSyncRegression_IncludeOnTheLabelsLine(t *testing.T) {
	resetFixState()
	t.Cleanup(resetFixState)

	const rel = "templates/cainjector/rbac-for-us.yaml"

	modulePath := syncModuleDir(t)
	model := syncModel(t, modulePath)
	writeGenerated(t, modulePath, model)

	fullPath := filepath.Join(modulePath, rel)
	content, err := os.ReadFile(fullPath)
	require.NoError(t, err)

	patched := strings.Replace(string(content), "| nindent 2 }}\n", "| nindent 2 }}{{ include \"extra-labels\" . | nindent 2 }}\n", 1)
	require.NotEqual(t, string(content), patched)
	require.NoError(t, os.WriteFile(fullPath, []byte(patched), 0o600))

	list := runSync(t, modulePath, renderedFrom(t, model, nil))
	for _, fix := range list.GetFixes() {
		fix()
	}

	got, err := os.ReadFile(fullPath)
	require.NoError(t, err)
	assert.Equal(t, patched, string(got))
}

// An object the template renders through an include of a named template is the library's; one
// with a document of its own, literal or with a computed name, is the module's (review of #479,
// finding 32).
func TestRenderedByInclude(t *testing.T) {
	text := `{{- include "helm_lib_csi_controller_rbac" . }}
# ==========
---
kind: ClusterRole
metadata:
  name: d8:csi-vsphere:csi
---
kind: ServiceAccount
metadata:
  name: {{ .Chart.Name }}-extra
`
	assert.True(t, renderedByInclude(text, "ServiceAccount", "csi"), "no document of its own: the include renders it")
	assert.True(t, renderedByInclude(text, "Role", "csi:controller:external-provisioner"))
	assert.False(t, renderedByInclude(text, "ClusterRole", "d8:csi-vsphere:csi"), "a literal document of its own")
	assert.False(t, renderedByInclude(text, "ServiceAccount", "csi-vsphere-extra"), "a document of its kind with a computed name")
	assert.False(t, renderedByInclude("---\nkind: Role\nmetadata:\n  name: r\n", "Role", "other"), "no include: not the library's")
}

// An object a {{ range }} renders is found by the stdlib template parser; one beside the range is
// not (review of #479, finding 39).
func TestRenderedInRange(t *testing.T) {
	text := `{{- range $version := .Values.istio.internal.versions }}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: istiod-{{ $version | replace "." "x" }}
{{- end }}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: operator
{{- if .Values.x }}
---
kind: Role
metadata:
  name: conditional
{{- end }}
`
	assert.True(t, renderedInRange(text, "ServiceAccount", "istiod-1x25"))
	assert.False(t, renderedInRange(text, "ServiceAccount", "operator"))
	assert.False(t, renderedInRange(text, "Role", "conditional"), "an if is no range")
	assert.False(t, renderedInRange("{{ if }", "Role", "x"), "a template that does not parse tells nothing")
}

// A template with a document that only includes a named template holds library objects (review of
// #479, finding 42).
func TestHoldsLibraryDocument(t *testing.T) {
	assert.True(t, holdsLibraryDocument("{{- include \"helm_lib_csi_controller_rbac\" . }}\n---\nkind: ClusterRole\nmetadata:\n  name: d8:m:csi\n"))
	assert.False(t, holdsLibraryDocument("---\nkind: ClusterRole\nmetadata:\n  name: d8:m:csi\n  {{- include \"helm_lib_module_labels\" (list .) | nindent 2 }}\n"))
}
