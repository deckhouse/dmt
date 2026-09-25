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

package rbacyaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// certManagerCRDs is what the linter sees in modules/101-cert-manager/crds/cert-manager/.
var certManagerCRDs = CRDScopes{
	"cert-manager.io/certificates":        ScopeNamespaced,
	"cert-manager.io/certificaterequests": ScopeNamespaced,
	"cert-manager.io/issuers":             ScopeNamespaced,
	"cert-manager.io/clusterissuers":      ScopeCluster,
}

// validDeclaration is the ADR's cert-manager example, trimmed to what the loader needs.
const validDeclaration = `
apiVersion: rbac.deckhouse.io/v1alpha1
resources:
  - group: cert-manager.io
    resource: issuers
    namespace:
      viewer: [watch, get, list]
      manager: [create, update, patch, delete, deletecollection]
    legacy:
      User: [get, list, watch]
      Editor: [create, update, patch, delete, deletecollection]
  - group: cert-manager.io
    resource: certificates
    namespace:
      viewer: [get, list, watch]
      admin: [delete]
    legacy:
      User: [get, list, watch]
      Admin: [delete]
  - group: cert-manager.io
    resource: clusterissuers
    system:
      viewer: [get, list, watch]
      manager: [create, update, patch, delete, deletecollection]
    legacy:
      User: [get, list, watch]
      ClusterEditor: [create, update, patch, delete, deletecollection]
  - group: trivy.deckhouse.io
    resource: vulnerabilityreports
    scope: Namespaced
    namespace:
      viewer: [get, list, watch]
  - group: constraints.gatekeeper.sh
    resource: "*"
    scope: Cluster
    reason: "one CRD per ConstraintTemplate is created at runtime; the names are not known statically"
    system:
      viewer: [get, list, watch]
  - group: cert-manager.io
    resource: certificaterequests
    noAccess: "internal resource, managed by the controller"
  - group: deckhouse.io
    resource: foos/status
    scope: Cluster
    system:
      manager: [get, patch, update]
capabilities:
  namespace.admin:
    title: {en: "Module cert-manager: admin", ru: "Модуль cert-manager: администрирование"}
    description: {en: "Delete certificates in a namespace.", ru: "Удаление сертификатов в пространстве имён."}
serviceAccounts:
  - name: cainjector
    path: cainjector
    when: .Values.certManager.internal.enableCAInjector
    labels: {app: cainjector}
    clusterRules:
      - apiGroups: [cert-manager.io]
        resources: [certificates]
        verbs: [get, list, watch]
      - nonResourceURLs: [/metrics]
        verbs: [get]
    namespaceRules:
      - apiGroups: [coordination.k8s.io]
        resources: [leases]
        verbs: [get, list, watch, create, update, patch]
    bindClusterRoles: [d8:rbac-proxy]
    bindRoles:
      - namespace: kube-system
        name: extension-apiserver-authentication-reader
prometheusAccess:
  deployments: [cert-manager]
access:
  - name: admin-kubeconfig
    subjects:
      - kind: Group
        name: kubeadm:cluster-admins
    clusterRules:
      - apiGroups: [cert-manager.io]
        resources: [clusterissuers]
        verbs: [get, list, watch]
  - name: auth
    subjects:
      - kind: ServiceAccount
        name: ingress-nginx
        namespace: d8-ingress-nginx
    namespaceRules:
      - apiGroups: [apps]
        resources: [deployments/http]
        resourceNames: [documentation]
        verbs: [get]
`

func TestParseAndValidate_ValidDeclaration(t *testing.T) {
	decl, err := Parse([]byte(validDeclaration))
	require.NoError(t, err)

	errs := Validate(decl, certManagerCRDs)
	assert.Empty(t, errs, "the ADR example must validate cleanly")

	// Normalize: resources by group then resource, verbs sorted.
	keys := make([]string, 0, len(decl.Resources))
	for _, r := range decl.Resources {
		keys = append(keys, r.Key())
	}

	assert.Equal(t, []string{
		"cert-manager.io/certificaterequests",
		"cert-manager.io/certificates",
		"cert-manager.io/clusterissuers",
		"cert-manager.io/issuers",
		"constraints.gatekeeper.sh/*",
		"deckhouse.io/foos/status",
		"trivy.deckhouse.io/vulnerabilityreports",
	}, keys)
	assert.Equal(t, []string{"get", "list", "watch"}, decl.Resources[3].Namespace["viewer"], "verbs are sorted")

	// Second parse of the same bytes yields an identical declaration: normalization is a fixed point.
	again, err := Parse([]byte(validDeclaration))
	require.NoError(t, err)
	assert.Equal(t, decl, again)
}

func TestParse_RejectsWhatTheFormatDoesNotKnow(t *testing.T) {
	for name, tc := range map[string]struct {
		yaml    string
		wantErr string
	}{
		"unknown top-level key": {
			yaml:    "apiVersion: rbac.deckhouse.io/v1alpha1\nfoo: bar\n",
			wantErr: "field foo not found",
		},
		"unknown key in a resource": {
			yaml:    "apiVersion: rbac.deckhouse.io/v1alpha1\nresources:\n  - group: g\n    resource: r\n    verbs: [get]\n",
			wantErr: "field verbs not found",
		},
		"two documents": {
			yaml:    "apiVersion: rbac.deckhouse.io/v1alpha1\n---\napiVersion: rbac.deckhouse.io/v1alpha1\n",
			wantErr: "single YAML document",
		},
		"not a mapping": {
			yaml:    "- a\n- b\n",
			wantErr: "parse rbac.yaml",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(tc.yaml))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// entry wraps one resource entry into a declaration, so each table row reads as the entry alone.
func entry(body string) string {
	return "apiVersion: rbac.deckhouse.io/v1alpha1\nresources:\n  - " + strings.ReplaceAll(strings.TrimSpace(body), "\n", "\n    ") + "\n"
}

func TestValidate_ResourceEntries(t *testing.T) {
	for name, tc := range map[string]struct {
		yaml    string
		crds    CRDScopes
		wantErr string // substring of exactly one error; "" means no errors
	}{
		"R2: a verb alias is not a verb": {
			yaml: entry(`group: cert-manager.io
resource: issuers
namespace:
  viewer: [read]`),
			crds:    certManagerCRDs,
			wantErr: `"read" is not a verb; verbs are listed explicitly`,
		},
		"R3: namespace level on a cluster-scoped resource (scope from CRD)": {
			yaml: entry(`group: cert-manager.io
resource: clusterissuers
namespace:
  viewer: [get]`),
			crds:    certManagerCRDs,
			wantErr: "namespace levels are not allowed for a cluster-scoped resource",
		},
		"R3: namespace level on a cluster-scoped resource (scope declared)": {
			yaml: entry(`group: external.io
resource: things
scope: Cluster
namespace:
  viewer: [get]`),
			crds:    certManagerCRDs,
			wantErr: "namespace levels are not allowed for a cluster-scoped resource",
		},
		"R4: noAccess together with levels": {
			yaml: entry(`group: cert-manager.io
resource: issuers
noAccess: "internal"
namespace:
  viewer: [get]`),
			crds:    certManagerCRDs,
			wantErr: "noAccess excludes namespace, system and legacy",
		},
		"R4: neither levels nor noAccess": {
			yaml: entry(`group: cert-manager.io
resource: issuers`),
			crds:    certManagerCRDs,
			wantErr: "must grant at least one level",
		},
		"R6: external resource without scope": {
			yaml: entry(`group: trivy.deckhouse.io
resource: vulnerabilityreports
namespace:
  viewer: [get]`),
			crds:    certManagerCRDs,
			wantErr: "ships no CRD for this resource, so scope is required",
		},
		"R6: denied external resource needs no scope": {
			yaml: entry(`group: trivy.deckhouse.io
resource: vulnerabilityreports
noAccess: "never shown to users"`),
			crds:    certManagerCRDs,
			wantErr: "",
		},
		"R6a: declared scope disagrees with the CRD": {
			yaml: entry(`group: cert-manager.io
resource: issuers
scope: Cluster
reason: "watched cluster-wide"
system:
  viewer: [get]`),
			crds:    certManagerCRDs,
			wantErr: `scope "Cluster" disagrees with the CRD in crds/, which says "Namespaced"`,
		},
		"R6c: wildcard without reason": {
			yaml: entry(`group: constraints.gatekeeper.sh
resource: "*"
scope: Cluster
system:
  viewer: [get]`),
			crds:    certManagerCRDs,
			wantErr: `resource "*" requires reason`,
		},
		"D5: wildcard on a group the module ships CRDs for": {
			yaml: entry(`group: cert-manager.io
resource: "*"
scope: Namespaced
reason: "lazy"
namespace:
  viewer: [get]`),
			crds:    certManagerCRDs,
			wantErr: `resource "*" is allowed only for a group the module ships no CRD for`,
		},
		"R7: non-conventional level without texts": {
			yaml: entry(`group: cert-manager.io
resource: issuers
namespace:
  admin: [delete]`),
			crds:    certManagerCRDs,
			wantErr: `"namespace.admin" is used by a resource entry but has no title and description`,
		},
		"R13c: when that is not a Helm expression": {
			yaml: entry(`group: cert-manager.io
resource: issuers
when: 'and (.Values.certManager.foo'
namespace:
  viewer: [get]`),
			crds:    certManagerCRDs,
			wantErr: `when "and (.Values.certManager.foo" is not a Helm expression`,
		},
		"R13c: when with Helm and sprig functions parses": {
			yaml: entry(`group: cert-manager.io
resource: issuers
when: 'and .Values.certManager.foo (semverCompare ">= 1.80" .Values.global.deckhouseVersion) (.Capabilities.APIVersions.Has "x/v1")'
namespace:
  viewer: [get]`),
			crds:    certManagerCRDs,
			wantErr: "",
		},
		"review 6: the wildcard verb at a user-facing level": {
			yaml: entry(`group: cert-manager.io
resource: issuers
namespace:
  viewer: ["*"]`),
			crds:    certManagerCRDs,
			wantErr: `namespace.viewer grants "*", every verb`,
		},
		"review 6: every API group": {
			yaml: entry(`group: "*"
resource: things
scope: Namespaced
noAccess: nobody`),
			crds:    certManagerCRDs,
			wantErr: `group "*" grants the resource in every API group`,
		},
		"review 6: a partial wildcard in the resource name": {
			yaml: entry(`group: external.io
resource: "secret*"
scope: Namespaced
noAccess: nobody`),
			crds:    certManagerCRDs,
			wantErr: `resource "secret*" is not a resource name`,
		},
		"review 20: the wildcard resource of a subresource is valid": {
			yaml: entry(`group: external.io
resource: "*/scale"
scope: Namespaced
reason: the resources of the group are not known statically
noAccess: nobody`),
			crds:    certManagerCRDs,
			wantErr: "",
		},
		"regression hunt B12: the wildcard of a subresource needs a reason, as \"*\" does": {
			yaml: entry(`group: external.io
resource: "*/scale"
scope: Namespaced
noAccess: nobody`),
			crds:    certManagerCRDs,
			wantErr: `resource "*/scale" requires reason`,
		},
		"regression hunt B12: the wildcard of a subresource in a group of the module": {
			yaml: entry(`group: cert-manager.io
resource: "*/status"
reason: every status
noAccess: nobody`),
			crds:    certManagerCRDs,
			wantErr: `resource "*/status" is allowed only for a group the module ships no CRD for`,
		},
		"regression hunt B12: a built-in resource under the wrong scope": {
			yaml: entry(`group: ""
resource: nodes
scope: Namespaced
noAccess: nobody`),
			wantErr: `scope "Namespaced" disagrees with Kubernetes, which serves /nodes as "Cluster"`,
		},
		"regression hunt 2 B7: namespaces declared Namespaced on purpose": {
			yaml: entry(`group: ""
resource: namespaces
scope: Namespaced
namespace: {viewer: [get]}`),
			wantErr: "",
		},
		"regression hunt B12: no dot in a resource name": {
			yaml: entry(`group: external.io
resource: things.v1
scope: Namespaced
noAccess: nobody`),
			wantErr: `resource "things.v1" is not a resource name`,
		},
		"review 6: a resource name with a space": {
			yaml: entry(`group: external.io
resource: "Widgets "
scope: Namespaced
noAccess: nobody`),
			crds:    certManagerCRDs,
			wantErr: `resource "Widgets " is not a resource name`,
		},
		"review 6: a group that is not a DNS name": {
			yaml: entry(`group: "External_IO"
resource: things
scope: Namespaced
noAccess: nobody`),
			crds:    certManagerCRDs,
			wantErr: `group "External_IO" is not an API group name`,
		},
		"review 13b: when with a template delimiter": {
			yaml: entry(`group: cert-manager.io
resource: issuers
when: 'true }}{{ include "x" . }}{{ if true'
namespace:
  viewer: [get]`),
			crds:    certManagerCRDs,
			wantErr: "holds a template delimiter; write the condition only",
		},
		"R26: namespaced resource at a system level without reason": {
			yaml: entry(`group: cert-manager.io
resource: issuers
system:
  viewer: [get]`),
			crds:    certManagerCRDs,
			wantErr: "granted across the whole cluster; confirm it with reason",
		},
		"R26: namespaced resource at a system level with reason": {
			yaml: entry(`group: cert-manager.io
resource: issuers
reason: "the module operator watches issuers in every namespace"
system:
  viewer: [get]`),
			crds:    certManagerCRDs,
			wantErr: "",
		},
		"R29: admin is not a system level": {
			yaml: entry(`group: cert-manager.io
resource: clusterissuers
system:
  admin: [get]`),
			crds:    certManagerCRDs,
			wantErr: `system level "admin" is not valid; the system levels are viewer, manager, superadmin`,
		},
		"R29: unknown legacy level": {
			yaml: entry(`group: cert-manager.io
resource: issuers
legacy:
  Viewer: [get]`),
			crds:    certManagerCRDs,
			wantErr: `legacy level "Viewer" is not valid`,
		},
		"verbs: empty list": {
			yaml: entry(`group: cert-manager.io
resource: issuers
namespace:
  viewer: []`),
			crds:    certManagerCRDs,
			wantErr: "namespace.viewer lists no verbs",
		},
		"verbs: duplicate": {
			yaml: entry(`group: cert-manager.io
resource: issuers
namespace:
  viewer: [get, get]`),
			crds:    certManagerCRDs,
			wantErr: `namespace.viewer lists "get" twice`,
		},
		"subresource inherits the base scope": {
			yaml: entry(`group: cert-manager.io
resource: clusterissuers/status
namespace:
  viewer: [get]`),
			crds:    certManagerCRDs,
			wantErr: "namespace levels are not allowed for a cluster-scoped resource",
		},
		"bad scope value": {
			yaml: entry(`group: external.io
resource: things
scope: cluster
system:
  viewer: [get]`),
			crds:    certManagerCRDs,
			wantErr: `scope must be "Namespaced" or "Cluster", got "cluster"`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			decl, err := Parse([]byte(tc.yaml))
			require.NoError(t, err)

			errs := Validate(decl, tc.crds)
			if tc.wantErr == "" {
				assert.Empty(t, errs)
				return
			}

			require.Len(t, errs, 1, "exactly one error expected, got: %v", errs)
			assert.Contains(t, errs[0].Error(), tc.wantErr)
		})
	}
}

func TestValidate_TopLevel(t *testing.T) {
	for name, tc := range map[string]struct {
		yaml    string
		wantErr string
	}{
		"R39a: unknown apiVersion": {
			yaml:    "apiVersion: rbac.deckhouse.io/v2\n",
			wantErr: `apiVersion must be "rbac.deckhouse.io/v1alpha1", got "rbac.deckhouse.io/v2"`,
		},
		"R39a: missing apiVersion": {
			yaml:    "resources: []\n",
			wantErr: `apiVersion must be "rbac.deckhouse.io/v1alpha1", got ""`,
		},
		"subsystems: not a subsystem": {
			yaml:    "apiVersion: rbac.deckhouse.io/v1alpha1\nsubsystems: [networking, billing]\n",
			wantErr: `subsystems: "billing" is not a subsystem of the role model`,
		},
		"duplicate resource entry": {
			yaml: "apiVersion: rbac.deckhouse.io/v1alpha1\nresources:\n" +
				"  - {group: g.io, resource: r, scope: Cluster, system: {viewer: [get]}}\n" +
				"  - {group: g.io, resource: r, scope: Cluster, system: {viewer: [list]}}\n",
			wantErr: "duplicate entry for g.io/r",
		},
		"capabilities: conventional action needs no texts": {
			yaml:    "apiVersion: rbac.deckhouse.io/v1alpha1\ncapabilities:\n  namespace.view: {title: {en: a, ru: b}, description: {en: c, ru: d}}\n",
			wantErr: `"namespace.view" needs no texts`,
		},
		"capabilities: missing ru": {
			yaml: "apiVersion: rbac.deckhouse.io/v1alpha1\ncapabilities:\n  namespace.admin: {title: {en: a}, description: {en: c, ru: d}}\n" +
				"resources:\n  - {group: x.io, resource: things, scope: Namespaced, namespace: {admin: [get]}}\n",
			wantErr: "namespace.admin.title requires both en and ru",
		},
		"capabilities: bad key": {
			yaml:    "apiVersion: rbac.deckhouse.io/v1alpha1\ncapabilities:\n  project.admin: {title: {en: a, ru: b}, description: {en: c, ru: d}}\n",
			wantErr: `key "project.admin" must be "namespace.<level>" or "system.<level>"`,
		},
		"access: both rule kinds": {
			yaml: "apiVersion: rbac.deckhouse.io/v1alpha1\naccess:\n  - name: x\n    subjects: [{kind: Group, name: g}]\n" +
				"    clusterRules: [{apiGroups: [a], resources: [b], verbs: [get]}]\n    namespaceRules: [{apiGroups: [a], resources: [b], verbs: [get]}]\n",
			wantErr: "exactly one of clusterRules and namespaceRules",
		},
		"access: ServiceAccount subject without namespace": {
			yaml: "apiVersion: rbac.deckhouse.io/v1alpha1\naccess:\n  - name: x\n    subjects: [{kind: ServiceAccount, name: s}]\n" +
				"    clusterRules: [{apiGroups: [a], resources: [b], verbs: [get]}]\n",
			wantErr: "a ServiceAccount subject requires namespace",
		},
		"serviceAccounts: duplicate name": {
			yaml:    "apiVersion: rbac.deckhouse.io/v1alpha1\nserviceAccounts:\n  - name: a\n  - name: a\n",
			wantErr: "serviceAccounts[1] (a): duplicate name",
		},
		"serviceAccounts: resources without apiGroups": {
			yaml:    "apiVersion: rbac.deckhouse.io/v1alpha1\nserviceAccounts:\n  - name: a\n    clusterRules: [{resources: [pods], verbs: [get]}]\n",
			wantErr: `serviceAccounts[0] (a).clusterRules[0]: resources require apiGroups; the core group is ""`,
		},
		"serviceAccounts: rule without verbs": {
			yaml:    "apiVersion: rbac.deckhouse.io/v1alpha1\nserviceAccounts:\n  - name: a\n    clusterRules: [{apiGroups: [x], resources: [y]}]\n",
			wantErr: "serviceAccounts[0] (a).clusterRules[0]: verbs is required",
		},
		"prometheusAccess: when that is not a Helm expression": {
			yaml:    "apiVersion: rbac.deckhouse.io/v1alpha1\nprometheusAccess:\n  deployments: [a]\n  when: 'and (.Values.x'\n",
			wantErr: "prometheusAccess: when",
		},
		"review 13a: a template delimiter in a capability text": {
			yaml: "apiVersion: rbac.deckhouse.io/v1alpha1\ncapabilities:\n  namespace.admin: {title: {en: 'Use {{ .Values.x }}', ru: b}, description: {en: c, ru: d}}\n" +
				"resources:\n  - {group: x.io, resource: things, scope: Namespaced, namespace: {admin: [get]}}\n",
			wantErr: `capabilities.namespace.admin.title.en: "Use {{ .Values.x }}" holds a template delimiter`,
		},
		"prometheusAccess: empty": {
			yaml:    "apiVersion: rbac.deckhouse.io/v1alpha1\nprometheusAccess: {}\n",
			wantErr: "prometheusAccess: names no workload",
		},
	} {
		t.Run(name, func(t *testing.T) {
			decl, err := Parse([]byte(tc.yaml))
			require.NoError(t, err)

			errs := Validate(decl, nil)
			require.Len(t, errs, 1, "exactly one error expected, got: %v", errs)
			assert.Contains(t, errs[0].Error(), tc.wantErr)
		})
	}
}

func TestLoad(t *testing.T) {
	modulePath := t.TempDir()

	_, err := Load(modulePath)
	assert.True(t, errors.Is(err, ErrNotFound), "a module without rbac.yaml is not an error of the file, got %v", err)

	require.NoError(t, os.WriteFile(filepath.Join(modulePath, Filename), []byte(validDeclaration), 0o600))

	decl, err := Load(modulePath)
	require.NoError(t, err)
	assert.Equal(t, APIVersionV1Alpha1, decl.APIVersion)
	assert.Len(t, decl.Resources, 7)
}

// extraClusterRoles: named, unique, with rules; automountServiceAccountToken parses.
func TestValidate_ExtraClusterRoles(t *testing.T) {
	decl, err := Parse([]byte(`apiVersion: rbac.deckhouse.io/v1alpha1
serviceAccounts:
  - name: webhook
    automountServiceAccountToken: true
    extraClusterRoles:
      - name: requester
        bind: false
        rules:
          - apiGroups: [admission.cert-manager.io]
            resources: [certificates]
            verbs: [create]
      - name: requester
        rules:
          - apiGroups: [""]
            resources: [secrets]
            verbs: [get]
      - name: ""
        rules: []
      - name: d8:other:full-name
        rules: []
`))
	require.NoError(t, err)
	require.NotNil(t, decl.ServiceAccounts[0].AutomountToken)
	assert.True(t, *decl.ServiceAccounts[0].AutomountToken)
	assert.False(t, decl.ServiceAccounts[0].ExtraClusterRoles[0].IsBound())
	assert.True(t, decl.ServiceAccounts[0].ExtraClusterRoles[1].IsBound())
	assert.Equal(t, "d8:cert-manager:webhook:requester", decl.ServiceAccounts[0].ExtraClusterRoles[0].FullName("cert-manager", "webhook"))
	assert.Equal(t, "d8:other:full-name", decl.ServiceAccounts[0].ExtraClusterRoles[3].FullName("cert-manager", "webhook"))

	errs := Validate(decl, nil)

	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}

	assert.Contains(t, msgs, `serviceAccounts[0] (webhook).extraClusterRoles[1]: duplicate name "requester"`)
	assert.Contains(t, msgs, "serviceAccounts[0] (webhook).extraClusterRoles[2]: name is required")
	assert.Contains(t, msgs, "serviceAccounts[0] (webhook).extraClusterRoles[3] (d8:other:full-name): rules is required")
	assert.Len(t, msgs, 3, "got: %v", msgs)
}

// Built-in Kubernetes resources need no scope of their own; a duplicate subject is an error.
func TestValidate_WellKnownScopesAndDuplicateSubjects(t *testing.T) {
	decl, err := Parse([]byte(`apiVersion: rbac.deckhouse.io/v1alpha1
resources:
  - group: ""
    resource: configmaps
    namespace:
      viewer: [get]
  - group: ""
    resource: namespaces
    namespace:
      viewer: [get]
  - group: apps
    resource: deployments/scale
    system:
      manager: [update]
    reason: "cluster-wide scaling"
access:
  - name: dup
    subjects:
      - kind: Group
        name: g
      - kind: Group
        name: g
    clusterRules:
      - apiGroups: [""]
        resources: [pods]
        verbs: [get]
`))
	require.NoError(t, err)

	errs := Validate(decl, nil)

	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}

	assert.Contains(t, msgs, "resources[1] (/namespaces): namespace levels are not allowed for a cluster-scoped resource: a namespace capability is granted through a RoleBinding, where such a rule grants nothing; use system", "the built-in scope is known and applied")
	assert.Contains(t, msgs, "access[0] (dup).subjects[1]: duplicate subject Group g")
	assert.Len(t, msgs, 2, "configmaps and deployments/scale need no scope: %v", msgs)
}

// legacy.SuperAdmin validates but reaches nobody; it is a warning (review of #479, finding 13e).
func TestWarnings_LegacySuperAdmin(t *testing.T) {
	decl, err := Parse([]byte(entry(`group: cert-manager.io
resource: issuers
legacy:
  SuperAdmin: [get]`)))
	require.NoError(t, err)
	assert.Empty(t, Validate(decl, certManagerCRDs))

	w := Warnings(decl)
	require.Len(t, w, 1)
	assert.Contains(t, w[0], "legacy.SuperAdmin produces a role user-authz does not aggregate")
}

// The generator writes label and annotation keys unquoted; the generator's and Helm's own
// annotations are not the declaration's (regression hunt, B7).
func TestValidate_AccountMetadataKeys(t *testing.T) {
	decl := &Declaration{APIVersion: APIVersionV1Alpha1, ServiceAccounts: []ServiceAccount{{
		Name:            "m",
		Labels:          map[string]string{"bad key": "x", "example.com/" + strings.Repeat("a", 80): "x"},
		Annotations:     map[string]string{"helm.sh/resource-policy": "keep", "meta.helm.sh/release-name": "m"},
		RBACAnnotations: map[string]string{"rbac.deckhouse.io/kind": "x", "werf.io/deploy-on": "pre-install"},
	}}}

	errs := Validate(decl, nil)

	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}

	got := strings.Join(msgs, "\n")
	assert.Contains(t, got, `serviceAccounts[0] (m).labels: "bad key" is not a valid key`)
	assert.Contains(t, got, `serviceAccounts[0] (m).labels: "example.com/`+strings.Repeat("a", 80)+`" is not a valid key`, "the name of a key is 63 characters at most")
	assert.Contains(t, got, `serviceAccounts[0] (m).annotations: "meta.helm.sh/release-name" is set by the generator or by Helm`)
	assert.Contains(t, got, `serviceAccounts[0] (m).rbacAnnotations: "rbac.deckhouse.io/kind" is set by the generator or by Helm`)
	assert.NotContains(t, got, "helm.sh/resource-policy")
	assert.NotContains(t, got, "werf.io")
}

// A text for a level nobody grants is reported; so are an account name Kubernetes refuses and an
// empty value in a rule; messages carry the entry's index in the file (regression hunt, B12).
func TestValidate_RegressionHuntB12(t *testing.T) {
	decl, err := Parse([]byte(`apiVersion: rbac.deckhouse.io/v1alpha1
capabilities:
  namespace.approve: {title: {en: a, ru: b}, description: {en: c, ru: d}}
resources:
  - {group: z.io, resource: things, scope: Namespaced, namespace: {viewer: [get]}}
  - {group: a.io, resource: things, scope: Namespaced, namespace: {viewer: [bogus]}}
serviceAccounts:
  - name: Bad_Name
    clusterRules:
      - apiGroups: [""]
        resources: [""]
        verbs: [get, ""]
`))
	require.NoError(t, err)

	errs := Validate(decl, nil)

	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}

	got := strings.Join(msgs, "\n")
	assert.Contains(t, got, `capabilities: "namespace.approve" has texts, but no resource entry grants namespace level "approve"`)
	assert.Contains(t, got, `resources[1] (a.io/things): namespace.viewer: "bogus" is not a verb`, "the index of the file, not of the sorted list")
	assert.NotContains(t, got, "resources[0] (a.io/things)")
	assert.Contains(t, got, `serviceAccounts[0] (Bad_Name): a ServiceAccount name is a lowercase DNS subdomain`)
	assert.Contains(t, got, `serviceAccounts[0] (Bad_Name).clusterRules[0]: verbs holds an empty value`)
	assert.Contains(t, got, `serviceAccounts[0] (Bad_Name).clusterRules[0]: resources holds an empty value`)
}

// A condition that parses but passes a function without its arguments fails when it renders; the
// validator names it (regression hunt 2, B1).
func TestValidateWhen_BareFunction(t *testing.T) {
	check := func(when string) string {
		var got []string

		validateWhen(when, "x", func(format string, args ...any) { got = append(got, fmt.Sprintf(format, args...)) })

		return strings.Join(got, "\n")
	}

	assert.Contains(t, check("and not (.Values.a) (.Values.b)"), "passes not to another function without its arguments")
	assert.Contains(t, check("and (.Values.a) not (.Values.b)"), "passes not")
	assert.Empty(t, check("and (not (.Values.a)) (.Values.b)"))
	assert.Empty(t, check(`.Values.global.enabledModules | has "prometheus"`))
	assert.Empty(t, check(`and .Values.a (not .Values.b)`))
	assert.Empty(t, check(`include "helper" . | eq "true"`))
	assert.Empty(t, check(`lt (now | unixEpoch) 0 | not`))
}

// A TODO `when` is an open decision, not a malformed expression (review of #479, finding 51).
func TestValidateWhen_TODO(t *testing.T) {
	var got []string

	validateWhen("TODO: write the condition", "serviceAccounts[0] (m)", func(format string, args ...any) { got = append(got, fmt.Sprintf(format, args...)) })

	require.Len(t, got, 1)
	assert.Contains(t, got[0], "is still undecided: a decision is needed")
	assert.NotContains(t, got[0], "not a Helm expression")
}
