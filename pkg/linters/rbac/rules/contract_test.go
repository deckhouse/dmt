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
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbacyaml"
)

// rendered is one rendered object: the template it came from and its manifest.
type rendered struct {
	path string
	yaml string
}

func storeOf(t *testing.T, objects ...rendered) map[storage.ResourceIndex]storage.StoreObject {
	t.Helper()

	store := storage.NewUnstructuredObjectStore()

	for _, o := range objects {
		var content map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(o.yaml), &content))
		require.NoError(t, store.Put("/module/"+o.path, o.path, content, []byte(o.yaml)))
	}

	return store.Storage
}

func runContract(t *testing.T, modulePath string, objects ...rendered) []string {
	t.Helper()

	m := mocks.NewModuleMock(minimock.NewController(t))
	m.GetPathMock.Return(modulePath)
	// A legacy object from a gated template stops before the module name is needed.
	m.GetNameMock.Optional().Return("cert-manager")
	m.GetStorageMock.Return(storeOf(t, objects...))

	errorList := errors.NewLintRuleErrorsList()
	NewContractRule(nil, m, errorList).Check(context.Background())

	return texts(errorList)
}

// notUse is the expression every selector of a system or subsystem role carries: it leaves out the
// use capabilities of the scheme before DKP 1.78.
const notUse = "    matchExpressions:\n    - {key: rbac.deckhouse.io/kind, operator: NotIn, values: [use]}\n"

const i18n = `
    en.meta.deckhouse.io/title: "t"
    ru.meta.deckhouse.io/title: "т"
    en.meta.deckhouse.io/description: "d"
    ru.meta.deckhouse.io/description: "д"`

func clusterRole(name string, labels map[string]string, annotations, body string) string {
	var b strings.Builder

	b.WriteString("apiVersion: rbac.authorization.k8s.io/v1\nkind: ClusterRole\nmetadata:\n  name: \"" + name + "\"\n  labels:\n")

	for _, k := range slices.Sorted(maps.Keys(labels)) {
		b.WriteString("    " + k + ": \"" + labels[k] + "\"\n")
	}

	if annotations != "" {
		b.WriteString("  annotations:" + annotations + "\n")
	}

	b.WriteString(body)

	return b.String()
}

var (
	validCapability = clusterRole("d8:namespace-capability:cert-manager:view", map[string]string{"module": "cert-manager",
		"rbac.deckhouse.io/kind":                      "capability",
		"rbac.deckhouse.io/scope":                     "namespace",
		"rbac.deckhouse.io/capability":                "namespace-capability.cert-manager.view",
		"rbac.deckhouse.io/aggregate-to-namespace-as": "viewer",
	}, i18n, "rules:\n- apiGroups: [cert-manager.io]\n  resources: [certificates]\n  verbs: [get, list, watch]\n")

	validRole = clusterRole("d8:subsystem:network:viewer", map[string]string{"module": "cert-manager",
		"rbac.deckhouse.io/kind":      "role",
		"rbac.deckhouse.io/scope":     "subsystem",
		"rbac.deckhouse.io/subsystem": "network",
		"rbac.deckhouse.io/use-role":  "viewer",
	}, i18n, "aggregationRule:\n  clusterRoleSelectors:\n  - matchLabels:\n      rbac.deckhouse.io/aggregate-to-network-as: viewer\n"+notUse)
)

func TestContract_CleanObjectsAndOutOfScopeFiles(t *testing.T) {
	got := runContract(t, t.TempDir(),
		rendered{"templates/rbacv2/use/view.yaml", validCapability},
		rendered{"templates/rbacv2/global/subsystem/roles/network/viewer.yaml", validRole},
		// the compatibility aliases keep the old names on purpose and are outside the contract
		rendered{"templates/rbacv2-compat/aliases.yaml", clusterRole("d8:manage:networking:viewer", map[string]string{"module": "cert-manager", "rbac.deckhouse.io/kind": "role"}, "", "")},
		// a controller ClusterRole elsewhere is none of the contract's business
		rendered{"templates/rbac-for-us.yaml", clusterRole("d8:cert-manager:controller", nil, "", "rules: []\n")},
		// d8:dict is a helper outside the framework: only the prefix and the texts are required
		rendered{"templates/rbacv2/global/dict.yaml", clusterRole("d8:dict", nil, i18n, "rules: []\n")},
	)
	assert.Empty(t, got)
}

func TestContract_Findings(t *testing.T) {
	for name, tc := range map[string]struct {
		object   string
		wantErrs []string
	}{
		"name prefix": {
			object: clusterRole("namespace-capability:x:view", map[string]string{"module": "cert-manager",
				"rbac.deckhouse.io/kind": "capability", "rbac.deckhouse.io/scope": "namespace",
				"rbac.deckhouse.io/capability": "namespace-capability.x.view", "rbac.deckhouse.io/aggregate-to-namespace-as": "viewer",
			}, i18n, "rules:\n- apiGroups: [x.io]\n  resources: [ys]\n  verbs: [get]\n"),
			wantErrs: []string{
				`error: name "namespace-capability:x:view" must start with the d8: prefix`,
				`error: capability name "namespace-capability:x:view" must start with "d8:namespace-capability:" for scope "namespace"`,
			},
		},
		"missing i18n": {
			object: clusterRole("d8:namespace-capability:x:view", map[string]string{"module": "cert-manager",
				"rbac.deckhouse.io/kind": "capability", "rbac.deckhouse.io/scope": "namespace",
				"rbac.deckhouse.io/capability": "namespace-capability.x.view", "rbac.deckhouse.io/aggregate-to-namespace-as": "viewer",
			}, "\n    en.meta.deckhouse.io/title: \"t\"\n    en.meta.deckhouse.io/description: \"d\"", "rules:\n- apiGroups: [x.io]\n  resources: [ys]\n  verbs: [get]\n"),
			wantErrs: []string{
				"error: missing the ru.meta.deckhouse.io/title annotation: every RBACv2 role and capability carries localized en/ru title and description",
				"error: missing the ru.meta.deckhouse.io/description annotation: every RBACv2 role and capability carries localized en/ru title and description",
			},
		},
		"capability with aggregationRule and no marker": {
			object: clusterRole("d8:namespace-capability:x:view", map[string]string{"module": "cert-manager",
				"rbac.deckhouse.io/kind": "capability", "rbac.deckhouse.io/scope": "namespace",
				"rbac.deckhouse.io/aggregate-to-namespace-as": "viewer",
			}, i18n, "rules:\n- apiGroups: [x.io]\n  resources: [ys]\n  verbs: [get]\naggregationRule:\n  clusterRoleSelectors:\n  - matchLabels: {a: b}\n"),
			wantErrs: []string{
				`error: capability "d8:namespace-capability:x:view" must not define aggregationRule`,
				`error: capability "d8:namespace-capability:x:view" must carry the rbac.deckhouse.io/capability label`,
			},
		},
		"role with rules and a wrong selector": {
			object: clusterRole("d8:namespace:viewer", map[string]string{"module": "cert-manager",
				"rbac.deckhouse.io/kind": "role", "rbac.deckhouse.io/scope": "namespace",
			}, i18n, "rules:\n- apiGroups: [x.io]\n  resources: [ys]\n  verbs: [get]\naggregationRule:\n  clusterRoleSelectors:\n  - matchLabels:\n      rbac.deckhouse.io/kind: capability\n"),
			wantErrs: []string{
				`error: role "d8:namespace:viewer" must not define its own rules; move them into a capability`,
				`error: role "d8:namespace:viewer" aggregation selector uses non-aggregation label "rbac.deckhouse.io/kind"`,
			},
		},
		"delegatable on a system role, use-role missing": {
			object: clusterRole("d8:system:viewer", map[string]string{"module": "cert-manager",
				"rbac.deckhouse.io/kind": "role", "rbac.deckhouse.io/scope": "system", "rbac.deckhouse.io/delegatable": "true",
			}, i18n, "aggregationRule:\n  clusterRoleSelectors:\n  - matchLabels:\n      rbac.deckhouse.io/aggregate-to-system-as: viewer\n"+notUse),
			wantErrs: []string{
				`error: label rbac.deckhouse.io/use-role must carry a valid level, got ""`,
				"error: label rbac.deckhouse.io/delegatable is only allowed on namespace/project roles",
			},
		},
		"R29: level not of the lineage": {
			object: clusterRole("d8:system-capability:x:edit", map[string]string{"module": "cert-manager",
				"rbac.deckhouse.io/kind": "capability", "rbac.deckhouse.io/scope": "system",
				"rbac.deckhouse.io/capability": "system-capability.x.edit", "rbac.deckhouse.io/aggregate-to-system-as": "admin",
			}, i18n, "rules:\n- apiGroups: [x.io]\n  resources: [ys]\n  verbs: [get]\n"),
			wantErrs: []string{
				`error: aggregation label "rbac.deckhouse.io/aggregate-to-system-as" has invalid level "admin"; the system lineage has viewer, manager, superadmin`,
			},
		},
		"R29: a system role named with a namespace level": {
			object: clusterRole("d8:system:admin", map[string]string{"module": "cert-manager",
				"rbac.deckhouse.io/kind": "role", "rbac.deckhouse.io/scope": "system", "rbac.deckhouse.io/use-role": "admin",
			}, i18n, "aggregationRule:\n  clusterRoleSelectors:\n  - matchLabels:\n      rbac.deckhouse.io/aggregate-to-system-as: admin\n"+notUse),
			wantErrs: []string{
				`error: role name "d8:system:admin" has invalid level "admin"; the system lineage has viewer, manager, superadmin`,
				`error: role "d8:system:admin" aggregation selector has invalid level "admin"`,
			},
		},
		"review 6: a capability with wildcard verbs and groups": {
			object: clusterRole("d8:namespace-capability:x:view", map[string]string{"module": "cert-manager",
				"rbac.deckhouse.io/kind": "capability", "rbac.deckhouse.io/scope": "namespace",
				"rbac.deckhouse.io/capability": "namespace-capability.x.view", "rbac.deckhouse.io/aggregate-to-namespace-as": "viewer",
			}, i18n, "rules:\n- apiGroups: [\"*\"]\n  resources: [secrets]\n  verbs: [\"*\"]\n"),
			wantErrs: []string{
				`error: capability "d8:namespace-capability:x:view" grants verb "*" on *, secrets; list the verbs`,
				`error: capability "d8:namespace-capability:x:view" grants on every API group (apiGroups: ["*"]); name the groups`,
			},
		},
		"R21: the module label names another module": {
			object: clusterRole("d8:namespace-capability:x:view", map[string]string{"module": "other",
				"rbac.deckhouse.io/kind": "capability", "rbac.deckhouse.io/scope": "namespace",
				"rbac.deckhouse.io/capability": "namespace-capability.x.view", "rbac.deckhouse.io/aggregate-to-namespace-as": "viewer",
			}, i18n, "rules:\n- apiGroups: [x.io]\n  resources: [ys]\n  verbs: [get]\n"),
			wantErrs: []string{
				`error: label module must be the module name "cert-manager", got "other"`,
			},
		},
		"unknown lineage and bad scope label": {
			object: clusterRole("d8:namespace-capability:x:view", map[string]string{"module": "cert-manager",
				"rbac.deckhouse.io/kind": "capability", "rbac.deckhouse.io/scope": "tenant",
			}, i18n, "rules:\n- apiGroups: [x.io]\n  resources: [ys]\n  verbs: [get]\n"),
			wantErrs: []string{
				`error: label rbac.deckhouse.io/scope must be one of namespace/project/subsystem/system, got "tenant"`,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := runContract(t, t.TempDir(), rendered{"templates/rbacv2/x.yaml", tc.object})
			assert.ElementsMatch(t, tc.wantErrs, got)
		})
	}
}

// A system or subsystem role is bound cluster-wide, so each of its selectors leaves out the use
// capabilities of the scheme before DKP 1.78 with a NotIn expression, as the platform test requires; a
// namespace role is bound in a namespace and is not held to it.
func TestContract_SystemAndSubsystemRolesLeaveOutUse(t *testing.T) {
	role := func(name, scope, lineage, expressions string) string {
		labels := map[string]string{"module": "cert-manager", "rbac.deckhouse.io/kind": "role", "rbac.deckhouse.io/scope": scope}
		if scope != "namespace" {
			labels["rbac.deckhouse.io/use-role"] = "viewer"
		}

		if scope == "subsystem" {
			labels["rbac.deckhouse.io/subsystem"] = lineage
		}

		return clusterRole(name, labels, i18n,
			"aggregationRule:\n  clusterRoleSelectors:\n  - matchLabels:\n      rbac.deckhouse.io/aggregate-to-"+lineage+"-as: viewer\n"+expressions)
	}

	for name, tc := range map[string]struct {
		object  string
		wantErr string
	}{
		"subsystem role without the expression": {
			object:  role("d8:subsystem:network:viewer", "subsystem", "network", ""),
			wantErr: `error: role "d8:subsystem:network:viewer" aggregation selector must leave out rbac.deckhouse.io/kind "use" with a NotIn expression`,
		},
		"subsystem role leaving out use": {
			object: role("d8:subsystem:network:viewer", "subsystem", "network", notUse),
		},
		"subsystem role leaving out role and use": {
			object: role("d8:subsystem:network:viewer", "subsystem", "network",
				"    matchExpressions:\n    - {key: rbac.deckhouse.io/kind, operator: NotIn, values: [role, use]}\n"),
		},
		"subsystem role leaving out another kind": {
			object: role("d8:subsystem:network:viewer", "subsystem", "network",
				"    matchExpressions:\n    - {key: rbac.deckhouse.io/kind, operator: NotIn, values: [role]}\n"),
			wantErr: `error: role "d8:subsystem:network:viewer" aggregation selector must leave out rbac.deckhouse.io/kind "use" with a NotIn expression`,
		},
		"subsystem role selecting use": {
			object: role("d8:subsystem:network:viewer", "subsystem", "network",
				"    matchExpressions:\n    - {key: rbac.deckhouse.io/kind, operator: In, values: [use]}\n"),
			wantErr: `error: role "d8:subsystem:network:viewer" aggregation selector must leave out rbac.deckhouse.io/kind "use" with a NotIn expression`,
		},
		"system role without the expression": {
			object:  role("d8:system:viewer", "system", "system", ""),
			wantErr: `error: role "d8:system:viewer" aggregation selector must leave out rbac.deckhouse.io/kind "use" with a NotIn expression`,
		},
		"namespace role without the expression": {
			object: role("d8:namespace:viewer", "namespace", "namespace", ""),
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := runContract(t, t.TempDir(), rendered{"templates/rbacv2/roles/viewer.yaml", tc.object})
			if tc.wantErr == "" {
				assert.Empty(t, got)
				return
			}

			assert.Equal(t, []string{tc.wantErr}, got)
		})
	}
}

func TestContract_ClusterScopedResourceInNamespaceCapabilityIsAWarning(t *testing.T) {
	capability := clusterRole("d8:namespace-capability:cert-manager:view", map[string]string{"module": "cert-manager",
		"rbac.deckhouse.io/kind":                      "capability",
		"rbac.deckhouse.io/scope":                     "namespace",
		"rbac.deckhouse.io/capability":                "namespace-capability.cert-manager.view",
		"rbac.deckhouse.io/aggregate-to-namespace-as": "viewer",
	}, i18n, "rules:\n- apiGroups: [cert-manager.io]\n  resources: [certificates, clusterissuers]\n  verbs: [get]\n- apiGroups: [external.io]\n  resources: [globals, unknowns]\n  verbs: [get]\n")

	// Scope from the module's CRDs (clusterissuers) and from rbac.yaml (external.io/globals);
	// external.io/unknowns is known to nobody and is not judged.
	modulePath := writeModule(t, map[string]string{
		"crds/cm.yaml":    crdYAML("cert-manager.io", "certificates", "Namespaced") + "---\n" + crdYAML("cert-manager.io", "clusterissuers", "Cluster"),
		rbacyaml.Filename: "apiVersion: rbac.deckhouse.io/v1alpha1\nresources:\n  - {group: external.io, resource: globals, scope: Cluster, system: {viewer: [get]}}\n",
	})

	got := runContract(t, modulePath, rendered{"templates/rbacv2/use/view.yaml", capability})
	assert.ElementsMatch(t, []string{
		`warn: capability "d8:namespace-capability:cert-manager:view" grants cert-manager.io/clusterissuers, a cluster-scoped resource, in a namespace capability: bound through a RoleBinding the rule grants nothing; move it to a system capability`,
		`warn: capability "d8:namespace-capability:cert-manager:view" grants external.io/globals, a cluster-scoped resource, in a namespace capability: bound through a RoleBinding the rule grants nothing; move it to a system capability`,
	}, got)
}

// A module still on the manage/use scheme gets one finding per object, not the whole contract; a
// module that serves both schemes behind the version gate gets none for the legacy branch.
func TestContract_LegacyScheme(t *testing.T) {
	legacy := clusterRole("d8:use:capability:module:cert-manager:view", map[string]string{
		"module": "cert-manager", "rbac.deckhouse.io/kind": "use", "rbac.deckhouse.io/aggregate-to-kubernetes-as": "viewer",
	}, "", "rules:\n- apiGroups: [cert-manager.io]\n  resources: [certificates]\n  verbs: [get]\n")

	t.Run("legacy object alone", func(t *testing.T) {
		got := runContract(t, t.TempDir(), rendered{"templates/rbacv2/use/view.yaml", legacy})
		require.Len(t, got, 1, "got: %v", got)
		assert.Contains(t, got[0], `error: ClusterRole "d8:use:capability:module:cert-manager:view" is of the legacy RBACv2 scheme (rbac.deckhouse.io/kind: use, the manage/use model before DKP 1.78); migrate the module with rbacv2-migrate-module.sh`)
	})

	t.Run("legacy object rendered from a gated template", func(t *testing.T) {
		modulePath := writeModule(t, map[string]string{
			"templates/rbacv2/use/view.yaml": "{{- if eq (include \"cert-manager.rbacv2_new_scheme\" .) \"true\" }}\n# new\n{{- else }}\n# legacy\n{{- end }}\n",
		})
		assert.Empty(t, runContract(t, modulePath, rendered{"templates/rbacv2/use/view.yaml", legacy}))
	})
}

// Two capabilities of one module with one marker are refused; a namespace capability granting a
// built-in cluster-scoped resource is warned about (review of #479, findings 13f and 13g).
func TestContract_DuplicateMarkerAndBuiltInScope(t *testing.T) {
	labels := func(marker string) map[string]string {
		return map[string]string{"module": "cert-manager", "rbac.deckhouse.io/kind": "capability", "rbac.deckhouse.io/scope": "namespace",
			"rbac.deckhouse.io/capability": marker, "rbac.deckhouse.io/aggregate-to-namespace-as": "viewer"}
	}

	got := runContract(t, t.TempDir(),
		rendered{"templates/rbacv2/use/view.yaml", clusterRole("d8:namespace-capability:cert-manager:view", labels("namespace-capability.cert-manager.view"), i18n,
			"rules:\n- apiGroups: [\"\"]\n  resources: [nodes]\n  verbs: [get]\n")},
		rendered{"templates/rbacv2/use/view2.yaml", clusterRole("d8:namespace-capability:cert-manager:view_more", labels("namespace-capability.cert-manager.view"), i18n,
			"rules:\n- apiGroups: [x.io]\n  resources: [ys]\n  verbs: [get]\n")},
	)

	joined := strings.Join(got, "\n")
	assert.Contains(t, joined, `capability marker "namespace-capability.cert-manager.view" is also carried by d8:namespace-capability:cert-manager:view`)
	assert.Contains(t, joined, "grants /nodes, a cluster-scoped resource, in a namespace capability")
}

// A module may ship a subsystem of its own, declared in its module.yaml: its subsystem roles and
// the capabilities that aggregate into it pass the contract, and a module that does not declare it
// still gets "unknown" (review of #480).
func TestContract_SubsystemOfTheModule(t *testing.T) {
	role := clusterRole("d8:subsystem:virtualization:manager", map[string]string{"module": "cert-manager",
		"rbac.deckhouse.io/kind":                           "role",
		"rbac.deckhouse.io/scope":                          "subsystem",
		"rbac.deckhouse.io/subsystem":                      "virtualization",
		"rbac.deckhouse.io/use-role":                       "admin",
		"rbac.deckhouse.io/aggregate-to-virtualization-as": "superadmin",
		"rbac.deckhouse.io/aggregate-to-system-as":         "manager",
	}, i18n, "aggregationRule:\n  clusterRoleSelectors:\n  - matchLabels:\n      rbac.deckhouse.io/aggregate-to-virtualization-as: manager\n"+notUse)

	capability := clusterRole("d8:system-capability:cert-manager:proxy_nodes", map[string]string{"module": "cert-manager",
		"rbac.deckhouse.io/kind":                           "capability",
		"rbac.deckhouse.io/scope":                          "system",
		"rbac.deckhouse.io/capability":                     "system-capability.cert-manager.proxy_nodes",
		"rbac.deckhouse.io/aggregate-to-virtualization-as": "manager",
	}, i18n, "rules:\n- apiGroups: [\"\"]\n  resources: [nodes/proxy]\n  verbs: [get]\n")

	objects := []rendered{
		{"templates/rbacv2/manage/roles/manager.yaml", role},
		{"templates/rbacv2/manage/proxy_nodes.yaml", capability},
	}

	declared := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(declared, "module.yaml"), []byte("name: cert-manager\nsubsystems: [virtualization]\n"), 0o600))
	assert.Empty(t, runContract(t, declared, objects...))

	got := strings.Join(runContract(t, t.TempDir(), objects...), "\n")
	assert.Contains(t, got, `role name "d8:subsystem:virtualization:manager" references unknown subsystem "virtualization"`)
	assert.Contains(t, got, `aggregation label "rbac.deckhouse.io/aggregate-to-virtualization-as" targets unknown lineage "virtualization"`)
	assert.Contains(t, got, `aggregation selector targets unknown lineage "virtualization"`)
}

// A subsystem module.yaml names beyond the platform's is the module's own only when the module
// renders its roles: a typo there must not silence the contract (review of #480, finding 9).
func TestContract_UnrenderedModuleSubsystem(t *testing.T) {
	capability := clusterRole("d8:system-capability:cert-manager:proxy_nodes", map[string]string{"module": "cert-manager",
		"rbac.deckhouse.io/kind":                  "capability",
		"rbac.deckhouse.io/scope":                 "system",
		"rbac.deckhouse.io/capability":            "system-capability.cert-manager.proxy_nodes",
		"rbac.deckhouse.io/aggregate-to-infra-as": "manager",
	}, i18n, "rules:\n- apiGroups: [\"\"]\n  resources: [nodes/proxy]\n  verbs: [get]\n")

	declared := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(declared, "module.yaml"), []byte("name: cert-manager\nsubsystems: [security, infra, infra]\n"), 0o600))

	got := runContract(t, declared, rendered{"templates/rbacv2/manage/proxy_nodes.yaml", capability})
	assert.Contains(t, strings.Join(got, "\n"), `aggregation label "rbac.deckhouse.io/aggregate-to-infra-as" targets unknown lineage "infra"`)
	assert.Contains(t, strings.Join(got, "\n"), `module.yaml subsystems: "infra" is not a subsystem of the role model`)

	var reported int

	for _, text := range got {
		if strings.Contains(text, `module.yaml subsystems: "infra"`) {
			reported++
		}
	}

	assert.Equal(t, 1, reported, "a subsystem listed twice is reported once")
}

// The subsystems are the eight of DKP: a capability aggregates into cluster or managed-services, and a
// module still on a subsystem of the legacy scheme hears that it is gone.
func TestContract_SubsystemsOfTheRoleModel(t *testing.T) {
	capability := func(lineages ...string) string {
		labels := map[string]string{"module": "cert-manager",
			"rbac.deckhouse.io/kind":       "capability",
			"rbac.deckhouse.io/scope":      "system",
			"rbac.deckhouse.io/capability": "system-capability.cert-manager.view",
		}
		for _, lineage := range lineages {
			labels["rbac.deckhouse.io/aggregate-to-"+lineage+"-as"] = "viewer"
		}

		return clusterRole("d8:system-capability:cert-manager:view", labels, i18n,
			"rules:\n- apiGroups: [cert-manager.io]\n  resources: [clusterissuers]\n  verbs: [get, list, watch]\n")
	}

	current := writeModule(t, map[string]string{"module.yaml": "name: cert-manager\nsubsystems: [cluster, managed-services]\n"})
	assert.Empty(t, runContract(t, current, rendered{"templates/rbacv2/manage/view.yaml", capability("cluster", "managed-services")}))

	legacy := writeModule(t, map[string]string{"module.yaml": "name: cert-manager\nsubsystems: [kubernetes]\n"})
	got := strings.Join(runContract(t, legacy, rendered{"templates/rbacv2/manage/view.yaml", capability("kubernetes")}), "\n")
	assert.Contains(t, got, `module.yaml subsystems: "kubernetes" is a subsystem of the legacy scheme, which the role model replaced with "cluster"; declare the module's subsystem of the role model (iam, security, cluster, delivery, network, storage, observability, managed-services)`)
	assert.Contains(t, got, `aggregation label "rbac.deckhouse.io/aggregate-to-kubernetes-as" targets unknown lineage "kubernetes"; the role model replaced it with "cluster"`)
}

// A subsystem of the legacy scheme is not the module's own even when the module renders roles for it:
// a module that kept shipping d8:subsystem:networking:<level> after the role model renamed networking
// to network is told so, and its roles and capabilities are refused.
func TestContract_LegacySubsystemBackedByARole(t *testing.T) {
	role := clusterRole("d8:subsystem:networking:manager", map[string]string{"module": "cert-manager",
		"rbac.deckhouse.io/kind":                       "role",
		"rbac.deckhouse.io/scope":                      "subsystem",
		"rbac.deckhouse.io/subsystem":                  "networking",
		"rbac.deckhouse.io/use-role":                   "admin",
		"rbac.deckhouse.io/aggregate-to-networking-as": "superadmin",
		"rbac.deckhouse.io/aggregate-to-system-as":     "manager",
	}, i18n, "aggregationRule:\n  clusterRoleSelectors:\n  - matchLabels:\n      rbac.deckhouse.io/aggregate-to-networking-as: manager\n"+notUse)

	capability := clusterRole("d8:system-capability:cert-manager:view", map[string]string{"module": "cert-manager",
		"rbac.deckhouse.io/kind":                       "capability",
		"rbac.deckhouse.io/scope":                      "system",
		"rbac.deckhouse.io/capability":                 "system-capability.cert-manager.view",
		"rbac.deckhouse.io/aggregate-to-networking-as": "viewer",
	}, i18n, "rules:\n- apiGroups: [cert-manager.io]\n  resources: [clusterissuers]\n  verbs: [get, list, watch]\n")

	declared := writeModule(t, map[string]string{"module.yaml": "name: cert-manager\nsubsystems: [networking]\n"})
	got := strings.Join(runContract(t, declared,
		rendered{"templates/rbacv2/manage/roles/manager.yaml", role},
		rendered{"templates/rbacv2/manage/view.yaml", capability},
	), "\n")

	assert.Contains(t, got, `module.yaml subsystems: "networking" is a subsystem of the legacy scheme, which the role model replaced with "network"`)
	assert.Contains(t, got, `role name "d8:subsystem:networking:manager" references unknown subsystem "networking"`)
	assert.Contains(t, got, `aggregation label "rbac.deckhouse.io/aggregate-to-networking-as" targets unknown lineage "networking"; the role model replaced it with "network"`)
	assert.Contains(t, got, `aggregation selector targets unknown lineage "networking"; the role model replaced it with "network"`)
	assert.NotContains(t, got, "renders no d8:subsystem:networking:<level> role")
}

// The module.yaml subsystems are the lineages the system capabilities carry, as the platform test
// TestRBACv2ModuleSubsystemsValidation requires: a lineage module.yaml does not declare and a
// subsystem no system capability aggregates into are reported, a module without system capabilities
// is not judged, and a subsystem of the module's own backed by its roles is compared as any other
// (review of #480, finding 13).
func TestContract_ModuleSubsystemsAreTheCarriedLineages(t *testing.T) {
	systemCapability := func(action string, lineages ...string) rendered {
		labels := map[string]string{"module": "cert-manager",
			"rbac.deckhouse.io/kind":       "capability",
			"rbac.deckhouse.io/scope":      "system",
			"rbac.deckhouse.io/capability": "system-capability.cert-manager." + action,
		}
		for _, lineage := range lineages {
			labels["rbac.deckhouse.io/aggregate-to-"+lineage+"-as"] = "viewer"
		}

		return rendered{"templates/rbacv2/manage/" + action + ".yaml", clusterRole("d8:system-capability:cert-manager:"+action, labels, i18n,
			"rules:\n- apiGroups: [cert-manager.io]\n  resources: [clusterissuers]\n  verbs: [get, list, watch]\n")}
	}

	ownRole := rendered{"templates/rbacv2/manage/roles/viewer.yaml", clusterRole("d8:subsystem:virtualization:viewer", map[string]string{"module": "cert-manager",
		"rbac.deckhouse.io/kind":      "role",
		"rbac.deckhouse.io/scope":     "subsystem",
		"rbac.deckhouse.io/subsystem": "virtualization",
		"rbac.deckhouse.io/use-role":  "viewer",
	}, i18n, "aggregationRule:\n  clusterRoleSelectors:\n  - matchLabels:\n      rbac.deckhouse.io/aggregate-to-virtualization-as: viewer\n"+notUse)}

	const advice = "the two must be the same set, as testing/rbacv2 in deckhouse requires (the documentation and the console read module.yaml, the aggregation controller the labels): "

	for name, tc := range map[string]struct {
		moduleYAML string
		objects    []rendered
		want       string
	}{
		"equal sets": {
			moduleYAML: "name: cert-manager\nsubsystems: [security, network]\n",
			objects:    []rendered{systemCapability("view", "network", "security"), systemCapability("edit", "security")},
		},
		"the system lineage is left out": {
			moduleYAML: "name: cert-manager\nsubsystems: [security]\n",
			objects:    []rendered{systemCapability("view", "security", "system")},
		},
		"a carried lineage module.yaml does not declare": {
			moduleYAML: "name: cert-manager\nsubsystems: [security]\n",
			objects:    []rendered{systemCapability("view", "security"), systemCapability("edit", "network", "security")},
			want:       "error: module.yaml subsystems: declares [security], but the system capabilities aggregate into [network, security]; " + advice + "declare network in module.yaml subsystems",
		},
		"a declared subsystem no system capability carries": {
			moduleYAML: "name: cert-manager\nsubsystems: [security, network]\n",
			objects:    []rendered{systemCapability("view", "security")},
			want:       "error: module.yaml subsystems: declares [network, security], but the system capabilities aggregate into [security]; " + advice + "aggregate the system capabilities into network (with rbac.yaml, `dmt lint --linter rbac --fix` does) or remove it from module.yaml subsystems",
		},
		"two declared subsystems no system capability carries": {
			moduleYAML: "name: cert-manager\nsubsystems: [security, network, storage]\n",
			objects:    []rendered{systemCapability("view", "security")},
			want:       "error: module.yaml subsystems: declares [network, security, storage], but the system capabilities aggregate into [security]; " + advice + "aggregate the system capabilities into network, storage (with rbac.yaml, `dmt lint --linter rbac --fix` does) or remove them from module.yaml subsystems",
		},
		"both ways at once": {
			moduleYAML: "name: cert-manager\nsubsystems: [storage]\n",
			objects:    []rendered{systemCapability("view", "cluster")},
			want:       "error: module.yaml subsystems: declares [storage], but the system capabilities aggregate into [cluster]; " + advice + "declare cluster in module.yaml subsystems; aggregate the system capabilities into storage (with rbac.yaml, `dmt lint --linter rbac --fix` does) or remove it from module.yaml subsystems",
		},
		"no module.yaml": {
			objects: []rendered{systemCapability("view", "security")},
			want:    "error: module.yaml subsystems: declares [], but the system capabilities aggregate into [security]; " + advice + "declare security in module.yaml subsystems",
		},
		"a module without system capabilities is not judged": {
			moduleYAML: "name: cert-manager\nsubsystems: [security]\n",
			objects:    []rendered{{"templates/rbacv2/use/view.yaml", validCapability}},
		},
		"a module.yaml that does not parse gives no finding": {
			moduleYAML: "subsystems: security\n",
			objects:    []rendered{systemCapability("view", "network")},
		},
		"a subsystem of the module's own backed by its roles": {
			moduleYAML: "name: cert-manager\nsubsystems: [virtualization, security]\n",
			objects:    []rendered{ownRole, systemCapability("view", "security", "virtualization")},
		},
		"a subsystem of the module's own no system capability carries": {
			moduleYAML: "name: cert-manager\nsubsystems: [virtualization, security]\n",
			objects:    []rendered{ownRole, systemCapability("view", "security")},
			want:       "error: module.yaml subsystems: declares [security, virtualization], but the system capabilities aggregate into [security]; " + advice + "aggregate the system capabilities into virtualization (with rbac.yaml, `dmt lint --linter rbac --fix` does) or remove it from module.yaml subsystems",
		},
	} {
		t.Run(name, func(t *testing.T) {
			files := map[string]string{}
			if tc.moduleYAML != "" {
				files["module.yaml"] = tc.moduleYAML
			}

			got := runContract(t, writeModule(t, files), tc.objects...)

			var subsystems []string

			for _, text := range got {
				if strings.Contains(text, "but the system capabilities aggregate into") {
					subsystems = append(subsystems, text)
				}
			}

			if tc.want == "" {
				assert.Empty(t, subsystems)
				return
			}

			assert.Equal(t, []string{tc.want}, subsystems)
		})
	}
}

// A lineage or a module.yaml subsystem another finding already names is not compared again: an
// unrendered subsystem, one of the legacy scheme, and an unknown lineage each get their own finding
// only.
func TestContract_ModuleSubsystemsLeaveOutWhatIsReported(t *testing.T) {
	capability := clusterRole("d8:system-capability:cert-manager:view", map[string]string{"module": "cert-manager",
		"rbac.deckhouse.io/kind":                       "capability",
		"rbac.deckhouse.io/scope":                      "system",
		"rbac.deckhouse.io/capability":                 "system-capability.cert-manager.view",
		"rbac.deckhouse.io/aggregate-to-security-as":   "viewer",
		"rbac.deckhouse.io/aggregate-to-kubernetes-as": "viewer",
		"rbac.deckhouse.io/aggregate-to-infra-as":      "viewer",
	}, i18n, "rules:\n- apiGroups: [cert-manager.io]\n  resources: [clusterissuers]\n  verbs: [get, list, watch]\n")

	modulePath := writeModule(t, map[string]string{"module.yaml": "name: cert-manager\nsubsystems: [security, kubernetes, infra]\n"})
	got := strings.Join(runContract(t, modulePath, rendered{"templates/rbacv2/manage/view.yaml", capability}), "\n")

	assert.Contains(t, got, `module.yaml subsystems: "kubernetes" is a subsystem of the legacy scheme`)
	assert.Contains(t, got, `module.yaml subsystems: "infra" is not a subsystem of the role model`)
	assert.Contains(t, got, `targets unknown lineage "kubernetes"`)
	assert.Contains(t, got, `targets unknown lineage "infra"`)
	assert.NotContains(t, got, "but the system capabilities aggregate into")
}
