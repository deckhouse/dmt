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

// Package rbacyaml reads and validates the module RBAC declaration, modules/<module>/rbac.yaml:
// the single source the rbac linter's coverage and sync rules compare the rendered RBAC objects
// against, and the generator renders templates from. The format is the one of the ADR
// "Единый rbac.yaml модуля" (platform-security/2026-04-27-module-rbac-yaml.md), version
// rbac.deckhouse.io/v1alpha1.
package rbacyaml

import "strings"

// Filename is the declaration's name in the module root.
const Filename = "rbac.yaml"

// APIVersionV1Alpha1 is the only format version this package accepts. The format is frozen as
// rbac.deckhouse.io/v1 after the pilots.
const APIVersionV1Alpha1 = "rbac.deckhouse.io/v1alpha1"

// NoAccessTODO is the placeholder the coverage autofix writes into a stub entry. It is a valid
// value for the loader, so the rest of the file stays usable, and an error for the coverage
// rule: a decision is still owed.
const NoAccessTODO = "TODO"

// Resource scopes as the CRD spells them.
const (
	ScopeNamespaced = "Namespaced"
	ScopeCluster    = "Cluster"
)

// Declaration is the parsed rbac.yaml.
type Declaration struct {
	APIVersion string `yaml:"apiVersion"`

	// Subsystems are the lineages the module's system capabilities aggregate into. Empty means
	// "the subsystems of module.yaml"; a module whose templates aggregate into more subsystems
	// than module.yaml declares must set it (kube-dns, kube-proxy, istio).
	Subsystems []string `yaml:"subsystems,omitempty"`

	Resources []Resource `yaml:"resources,omitempty"`

	// Capabilities holds the localized texts of capabilities outside the platform convention,
	// keyed "<lineage>.<level>" (for example "namespace.admin"). view/edit need no entry.
	Capabilities map[string]CapabilityText `yaml:"capabilities,omitempty"`

	ServiceAccounts []ServiceAccount `yaml:"serviceAccounts,omitempty"`

	PrometheusAccess *PrometheusAccess `yaml:"prometheusAccess,omitempty"`

	Access []Access `yaml:"access,omitempty"`
}

// Resource is one user-facing resource of the module and the access every role model grants
// to it. Exactly one of two shapes is valid: NoAccess with a reason, or at least one of
// Namespace/System/Legacy.
type Resource struct {
	Group    string `yaml:"group"`
	Resource string `yaml:"resource"`

	// Scope is required when the module ships no CRD for the resource (the linter cannot see
	// it) and when Resource is "*"; when the CRD is in the module tree the scope is read from
	// it and a declared Scope must agree.
	Scope string `yaml:"scope,omitempty"`

	// Reason is required when Resource is "*" (why the resource names are not known
	// statically) and when a Namespaced resource is granted at a system level (why the access
	// has to be cluster-wide). It is free text for the reader of the declaration.
	Reason string `yaml:"reason,omitempty"`

	// When is a Helm expression; the generated rules are wrapped in {{- if <When> }}. It must
	// evaluate under the linter's value stubs. A rule under When that is absent from the render
	// is not a divergence.
	When string `yaml:"when,omitempty"`

	// NoAccess documents the deliberate decision to grant users nothing on this resource. It
	// excludes Namespace, System and Legacy. NoAccessTODO is the undecided stub.
	NoAccess string `yaml:"noAccess,omitempty"`

	// Namespace maps RBACv2 namespace-lineage levels to verbs. Allowed for Namespaced
	// resources only.
	Namespace map[string][]string `yaml:"namespace,omitempty"`
	// System maps RBACv2 system-lineage levels to verbs. Allowed for both scopes.
	System map[string][]string `yaml:"system,omitempty"`
	// Legacy maps user-authz v1 access levels to verbs. It is never derived from the RBACv2
	// levels; an absent Legacy means no legacy rights.
	Legacy map[string][]string `yaml:"legacy,omitempty"`
}

// IsWildcard reports whether the entry grants a whole group ("resource: *").
func (r Resource) IsWildcard() bool { return r.Resource == "*" }

// IsSubresource reports whether the entry names a subresource (a "/" in the name).
func (r Resource) IsSubresource() bool {
	return strings.Contains(r.Resource, "/")
}

// HasLevels reports whether any role model grants something on the resource.
func (r Resource) HasLevels() bool {
	return len(r.Namespace) > 0 || len(r.System) > 0 || len(r.Legacy) > 0
}

// Key returns "group/resource", the identity of the entry.
func (r Resource) Key() string { return r.Group + "/" + r.Resource }

// CapabilityText is the localized title and description of a capability.
type CapabilityText struct {
	Title       LocalizedText `yaml:"title"`
	Description LocalizedText `yaml:"description"`
}

// LocalizedText is an en/ru pair; both are required.
type LocalizedText struct {
	EN string `yaml:"en"`
	RU string `yaml:"ru"`
}

// PolicyRule is a raw RBAC rule as Kubernetes spells it; the generator copies it verbatim.
type PolicyRule struct {
	APIGroups       []string `yaml:"apiGroups,omitempty"`
	Resources       []string `yaml:"resources,omitempty"`
	ResourceNames   []string `yaml:"resourceNames,omitempty"`
	NonResourceURLs []string `yaml:"nonResourceURLs,omitempty"`
	Verbs           []string `yaml:"verbs"`
}

// ServiceAccount declares one ServiceAccount of the module with its rights. The generator
// produces the ServiceAccount, ClusterRole d8:<module>:<name> with its ClusterRoleBinding
// (from ClusterRules), Role <name> in the module namespace with its RoleBinding (from
// NamespaceRules), a ClusterRoleBinding per BindClusterRoles entry and a RoleBinding in a
// foreign namespace per BindRoles entry, all into templates/[<path>/]rbac-for-us.yaml.
type ServiceAccount struct {
	Name string `yaml:"name"`
	// Path is the component directory under templates/; empty means the module root file.
	Path             string            `yaml:"path,omitempty"`
	When             string            `yaml:"when,omitempty"`
	Labels           map[string]string `yaml:"labels,omitempty"`
	ClusterRules     []PolicyRule      `yaml:"clusterRules,omitempty"`
	NamespaceRules   []PolicyRule      `yaml:"namespaceRules,omitempty"`
	BindClusterRoles []string          `yaml:"bindClusterRoles,omitempty"`
	BindRoles        []RoleRef         `yaml:"bindRoles,omitempty"`

	// ExtraClusterRoles are further ClusterRoles that live in the account's rbac-for-us.yaml:
	// controller roles split by concern (cert-manager's approve, certificates, ...), or roles the
	// module ships for other subjects to bind (an aggregated apiserver's requester role). Each is
	// d8:<module>:<name of the account>:<name>, or exactly the given name when it starts with d8:.
	ExtraClusterRoles []ExtraClusterRole `yaml:"extraClusterRoles,omitempty"`

	// Annotations go on the ServiceAccount (helm.sh/resource-policy: keep, werf.io/deploy-on, ...);
	// RBACAnnotations on every role and binding generated for the account.
	Annotations     map[string]string `yaml:"annotations,omitempty"`
	RBACAnnotations map[string]string `yaml:"rbacAnnotations,omitempty"`

	// AutomountToken is the ServiceAccount's automountServiceAccountToken; unset means false, the
	// platform convention. A pod that needs the token sets it true on the pod, or the account
	// declares true here.
	AutomountToken *bool `yaml:"automountServiceAccountToken,omitempty"`
}

// ExtraClusterRole is one more ClusterRole in a ServiceAccount's file. Bind unset or true also
// produces the ClusterRoleBinding of the same name to the account; false leaves the role unbound.
type ExtraClusterRole struct {
	Name  string       `yaml:"name"`
	Rules []PolicyRule `yaml:"rules"`
	Bind  *bool        `yaml:"bind,omitempty"`
}

// IsBound reports whether the role is bound to its account (the default).
func (r ExtraClusterRole) IsBound() bool { return r.Bind == nil || *r.Bind }

// FullName returns the ClusterRole name: the given one when it already starts with d8:, else
// d8:<module>:<account>:<name>.
func (r ExtraClusterRole) FullName(module, account string) string {
	if strings.HasPrefix(r.Name, "d8:") {
		return r.Name
	}

	return "d8:" + module + ":" + account + ":" + r.Name
}

// RoleRef names an existing Role in a foreign namespace to bind a ServiceAccount to.
type RoleRef struct {
	Namespace string `yaml:"namespace"`
	Name      string `yaml:"name"`
}

// PrometheusAccess declares the workloads whose metrics Prometheus scrapes; the generator
// produces the access-to-<module> Role and RoleBinding in the module namespace.
type PrometheusAccess struct {
	Deployments  []string `yaml:"deployments,omitempty"`
	DaemonSets   []string `yaml:"daemonsets,omitempty"`
	StatefulSets []string `yaml:"statefulsets,omitempty"`
	// When gates the RoleBinding to the scraper, the way the modules gate it today:
	// `.Values.global.enabledModules | has "prometheus"`. The Role stays unconditional, so the
	// generated file keeps the shape of the hand-written ones.
	When string `yaml:"when,omitempty"`
}

// Access grants arbitrary subjects rights on the module. ClusterRules produce a ClusterRole and
// ClusterRoleBinding d8:<module>:<name> in templates/rbac-for-us.yaml; NamespaceRules produce a
// Role and RoleBinding access-to-<module>-<name> in templates/rbac-to-us.yaml, or
// access-to-<path with dashes>-<name> in templates/<path>/rbac-to-us.yaml. Exactly one of the
// two must be set: the placement rule keeps cluster-scoped objects out of rbac-to-us.yaml.
type Access struct {
	Name     string    `yaml:"name"`
	Subjects []Subject `yaml:"subjects"`
	// Path is the component directory under templates/ whose rbac-for-us.yaml (clusterRules) or
	// rbac-to-us.yaml (namespaceRules) holds the objects; empty means the module root files.
	Path           string       `yaml:"path,omitempty"`
	ClusterRules   []PolicyRule `yaml:"clusterRules,omitempty"`
	NamespaceRules []PolicyRule `yaml:"namespaceRules,omitempty"`
}

// Subject is an RBAC subject; Namespace is required for a ServiceAccount.
type Subject struct {
	Kind      string `yaml:"kind"`
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace,omitempty"`
}

// CRDScopes is what the module tree says about its own resources: the scope of every CRD
// under crds/, keyed "group/plural". It is the loader's view of what is resolvable; a resource
// absent from it is external to this lint run.
type CRDScopes map[string]string
