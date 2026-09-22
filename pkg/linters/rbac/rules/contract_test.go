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

	validRole = clusterRole("d8:subsystem:networking:viewer", map[string]string{"module": "cert-manager",
		"rbac.deckhouse.io/kind":      "role",
		"rbac.deckhouse.io/scope":     "subsystem",
		"rbac.deckhouse.io/subsystem": "networking",
		"rbac.deckhouse.io/use-role":  "viewer",
	}, i18n, "aggregationRule:\n  clusterRoleSelectors:\n  - matchLabels:\n      rbac.deckhouse.io/aggregate-to-networking-as: viewer\n")
)

func TestContract_CleanObjectsAndOutOfScopeFiles(t *testing.T) {
	got := runContract(t, t.TempDir(),
		rendered{"templates/rbacv2/use/view.yaml", validCapability},
		rendered{"templates/rbacv2/global/subsystem/roles/networking/viewer.yaml", validRole},
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
			}, i18n, "aggregationRule:\n  clusterRoleSelectors:\n  - matchLabels:\n      rbac.deckhouse.io/aggregate-to-system-as: viewer\n"),
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
			}, i18n, "aggregationRule:\n  clusterRoleSelectors:\n  - matchLabels:\n      rbac.deckhouse.io/aggregate-to-system-as: admin\n"),
			wantErrs: []string{
				`error: role name "d8:system:admin" has invalid level "admin"; the system lineage has viewer, manager, superadmin`,
				`error: role "d8:system:admin" aggregation selector has invalid level "admin"`,
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
