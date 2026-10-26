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

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbaccontract"
)

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

	// parsed marks a declaration read by Parse: its resources carry their Position in the file.
	parsed bool

	// Subsystems is no longer read: the system capabilities aggregate into the subsystems of
	// module.yaml, which the platform test (testing/rbacv2 in deckhouse) holds equal to the lineages
	// they carry. The key still parses, so that a file written by an earlier dmt gets a finding that
	// says where the subsystems go, rather than an unknown key.
	Subsystems []string `yaml:"subsystems,omitempty"`

	Resources []Resource `yaml:"resources,omitempty"`

	// Capabilities describes capabilities beyond what a level alone says, keyed "<lineage>.<action>"
	// (for example "namespace.admin"): the localized texts of every capability outside the view/edit
	// convention, the level of a capability with an action of its own, and labels of the module.
	Capabilities map[string]Capability `yaml:"capabilities,omitempty"`

	ServiceAccounts []ServiceAccount `yaml:"serviceAccounts,omitempty"`

	PrometheusAccess PrometheusAccessList `yaml:"prometheusAccess,omitempty"`

	Access []Access `yaml:"access,omitempty"`
}

// Resource is one user-facing resource of the module and the access every role model grants
// to it. Exactly one of two shapes is valid: NoAccess with a reason, or at least one of
// Namespace/System/Legacy.
type Resource struct {
	Group    string `yaml:"group"`
	Resource string `yaml:"resource"`

	// Position is the entry's index in the file as written, for the messages: Normalize sorts
	// the entries. Set by Parse only.
	Position int `yaml:"-"`

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

	// Namespace maps RBACv2 namespace-lineage levels to verbs, or the action of a capability that
	// Capabilities gives a level. Allowed for Namespaced resources only.
	Namespace map[string][]string `yaml:"namespace,omitempty"`
	// System maps RBACv2 system-lineage levels to verbs, or the action of a capability that
	// Capabilities gives a level. Allowed for both scopes.
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

// Capability is one entry of Capabilities. A capability named after a level takes the action of
// that level (viewer -> view, manager -> edit, the rest as they are); a capability with an action
// of its own (download_snapshots, access_terminal) names the level it aggregates into, and a
// resource entry grants it under its action instead of a level.
type Capability struct {
	// Level is the level of the lineage the capability aggregates into; required for an action of
	// its own, not allowed for the action of a level.
	Level string `yaml:"level,omitempty"`

	// Labels go on the capability beside the ones of the role model: a label of the module that a
	// role of the module selects, outside the role model (rbac.deckhouse.io/ is dmt's).
	Labels map[string]string `yaml:"labels,omitempty"`

	// Title and Description are required for every capability but view and edit, which take the
	// platform's conventional texts.
	Title       LocalizedText `yaml:"title,omitempty"`
	Description LocalizedText `yaml:"description,omitempty"`
}

// HasTexts reports whether the entry sets a title or a description.
func (c Capability) HasTexts() bool {
	return c.Title != LocalizedText{} || c.Description != LocalizedText{}
}

// ActionOf resolves a key of a resource entry's Namespace or System map to the action of the
// capability it grants: a level grants the capability of that level's action, any other key is the
// action of a capability Capabilities gives a level.
func ActionOf(lineage, key string) string {
	if slices.Contains(rbaccontract.LevelsOf(lineage), key) {
		return rbaccontract.CapabilityAction(key)
	}

	return key
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
// (from ClusterRules), a Role in the module namespace with its RoleBinding (from NamespaceRules,
// NamespaceRoleNames), a ClusterRoleBinding per BindClusterRoles entry and a RoleBinding in a
// foreign namespace per BindRoles entry, all into templates/[<path>/]rbac-for-us.yaml.
type ServiceAccount struct {
	Name string `yaml:"name"`
	// Path is the component directory under templates/; empty means the module root file.
	Path         string            `yaml:"path,omitempty"`
	When         string            `yaml:"when,omitempty"`
	Labels       map[string]string `yaml:"labels,omitempty"`
	ClusterRules []PolicyRule      `yaml:"clusterRules,omitempty"`
	// ClusterRoleBindingName names the ClusterRoleBinding of ClusterRules instead of the ClusterRole's
	// name, d8:<module>:<name>: the name the binding had before the declaration, which the bootstrap
	// keeps where the placement rule accepts it.
	ClusterRoleBindingName string       `yaml:"clusterRoleBindingName,omitempty"`
	NamespaceRules         []PolicyRule `yaml:"namespaceRules,omitempty"`
	// RoleName names the Role of NamespaceRules instead of rbaccontract.AccountRoleName, and
	// RoleBindingName its RoleBinding instead of the Role's name: the names the objects had before the
	// declaration, which the bootstrap keeps where the placement rule accepts them.
	RoleName         string           `yaml:"roleName,omitempty"`
	RoleBindingName  string           `yaml:"roleBindingName,omitempty"`
	BindClusterRoles []ClusterRoleRef `yaml:"bindClusterRoles,omitempty"`
	BindRoles        []RoleRef        `yaml:"bindRoles,omitempty"`

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

// ClusterRoleNames returns the names of the ClusterRole of ClusterRules, d8:<module>:<name>, and of its
// ClusterRoleBinding: ClusterRoleBindingName, else the ClusterRole's.
func (sa ServiceAccount) ClusterRoleNames(module string) (string, string) {
	role := "d8:" + module + ":" + sa.Name

	binding := sa.ClusterRoleBindingName
	if binding == "" {
		binding = role
	}

	return role, binding
}

// NamespaceRoleNames returns the names of the Role of NamespaceRules and of its RoleBinding: RoleName,
// else the name the placement rule wants (rbaccontract.AccountRoleName); RoleBindingName, else the
// Role's.
func (sa ServiceAccount) NamespaceRoleNames(module string) (string, string) {
	role := sa.RoleName
	if role == "" {
		role = rbaccontract.AccountRoleName(module, sa.Path, sa.Name)
	}

	binding := sa.RoleBindingName
	if binding == "" {
		binding = role
	}

	return role, binding
}

// ExtraClusterRole is one more ClusterRole in a ServiceAccount's file. Bind unset or true also
// produces the ClusterRoleBinding to the account, of the same name or BindingName; false leaves the
// role unbound.
type ExtraClusterRole struct {
	Name string `yaml:"name"`
	// BindingName names the ClusterRoleBinding instead of the role's full name: the name the binding had
	// before the declaration, which the bootstrap keeps where the placement rule accepts it.
	BindingName string       `yaml:"bindingName,omitempty"`
	Rules       []PolicyRule `yaml:"rules"`
	Bind        *bool        `yaml:"bind,omitempty"`
}

// IsBound reports whether the role is bound to its account (the default).
func (r ExtraClusterRole) IsBound() bool { return r.Bind == nil || *r.Bind }

// BindingNameOf is the name of the role's ClusterRoleBinding: BindingName, else the role's full name.
func (r ExtraClusterRole) BindingNameOf(module, account string) string {
	if r.BindingName != "" {
		return r.BindingName
	}

	return r.FullName(module, account)
}

// FullName returns the ClusterRole name: the given one when it already starts with d8:, else
// d8:<module>:<account>:<name>.
func (r ExtraClusterRole) FullName(module, account string) string {
	if strings.HasPrefix(r.Name, "d8:") {
		return r.Name
	}

	return "d8:" + module + ":" + account + ":" + r.Name
}

// ClusterRoleRef names an existing ClusterRole to bind a ServiceAccount to. It is written as the role's
// name, or as a mapping when it names the binding too.
type ClusterRoleRef struct {
	Name string `yaml:"name"`
	// BindingName is the name of the ClusterRoleBinding; empty means d8:<module>:<account>:<role>
	// (BindingNameOf). The bootstrap sets it when the module's binding already has another name the
	// placement rule accepts, so the first --fix renames nothing.
	BindingName string `yaml:"bindingName,omitempty"`
}

// BindingNameOf is the name of the account's ClusterRoleBinding to the role: BindingName, else
// d8:<module>:<account>:<the role's name without d8:, colons as dashes>.
func (r ClusterRoleRef) BindingNameOf(module, account string) string {
	if r.BindingName != "" {
		return r.BindingName
	}

	return "d8:" + module + ":" + account + ":" + rbaccontract.BindingSuffix(r.Name)
}

// clusterRoleRefKeys are the keys of the mapping form, for the unknown-key check.
var clusterRoleRefKeys = yamlKeys(reflect.TypeFor[ClusterRoleRef]())

// UnmarshalYAML reads the role's name or the mapping. Unknown keys of the mapping are refused, as the
// strict decoder refuses them elsewhere in the file.
func (r *ClusterRoleRef) UnmarshalYAML(node *yaml.Node) error {
	switch {
	case node.Kind == yaml.ScalarNode && node.ShortTag() == "!!null":
		*r = ClusterRoleRef{}

		return nil
	case node.Kind == yaml.ScalarNode:
		*r = ClusterRoleRef{Name: node.Value}

		return nil
	case node.Kind == yaml.MappingNode:
	default:
		return &yaml.TypeError{Errors: []string{fmt.Sprintf("line %d: an entry of bindClusterRoles is the name of a ClusterRole or a mapping of name and bindingName", node.Line)}}
	}

	var unknown []string

	for i := 0; i+1 < len(node.Content); i += 2 {
		if key := node.Content[i]; !slices.Contains(clusterRoleRefKeys, key.Value) {
			unknown = append(unknown, fmt.Sprintf("line %d: field %s not found in type rbacyaml.ClusterRoleRef", key.Line, key.Value))
		}
	}

	if len(unknown) > 0 {
		return &yaml.TypeError{Errors: unknown}
	}

	// The plain type has no UnmarshalYAML: decoding into it does not come back here.
	type plain ClusterRoleRef

	var out plain
	if err := node.Decode(&out); err != nil {
		return err
	}

	*r = ClusterRoleRef(out)

	return nil
}

// MarshalYAML writes the role's name alone when the binding takes the derived name.
func (r ClusterRoleRef) MarshalYAML() (any, error) {
	if r.BindingName == "" {
		return r.Name, nil
	}

	type plain ClusterRoleRef

	return plain(r), nil
}

// RoleRef names an existing Role in a foreign namespace to bind a ServiceAccount to.
type RoleRef struct {
	Namespace string `yaml:"namespace"`
	Name      string `yaml:"name"`
	// BindingName is the name of the RoleBinding; empty means the name the placement rule wants
	// (rbaccontract.AccountForeignBindingName). The bootstrap sets it when the module's binding
	// already has another name the placement rule accepts, so the first --fix renames nothing.
	BindingName string `yaml:"bindingName,omitempty"`
}

// Refers reports whether the entry binds the Role name in the namespace ns.
func (r RoleRef) Refers(ns, name string) bool {
	return r.Namespace == ns && r.Name == name
}

// PrometheusAccess declares the workloads whose metrics Prometheus scrapes; the generator
// produces a Role and a RoleBinding in the module namespace: access-to-<module> in
// templates/rbac-to-us.yaml, or with Path and Name the name access[] gets
// (rbaccontract.AccessRoleName) in templates/[<path>/]rbac-to-us.yaml.
type PrometheusAccess struct {
	// Path is the component directory under templates/ whose rbac-to-us.yaml holds the objects;
	// empty means the module root file. A path needs a Name: the placement rule wants
	// access-to-<directory>-<name> there.
	Path string `yaml:"path,omitempty"`
	// Name names the objects access-to-<module>-<name> at the root, access-to-<path with dashes>-<name>
	// under Path; empty means access-to-<module>.
	Name string `yaml:"name,omitempty"`
	// BindingName names the RoleBinding instead of the Role's name: the name a binding had beside its
	// Role before the declaration, which the bootstrap keeps where the placement rule accepts it.
	BindingName  string   `yaml:"bindingName,omitempty"`
	Deployments  []string `yaml:"deployments,omitempty"`
	DaemonSets   []string `yaml:"daemonsets,omitempty"`
	StatefulSets []string `yaml:"statefulsets,omitempty"`
	// When gates the RoleBinding to the scraper, the way the modules gate it today:
	// `.Values.global.enabledModules | has "prometheus"`. The Role stays unconditional, so the
	// generated file keeps the shape of the hand-written ones.
	When string `yaml:"when,omitempty"`
	// Labels and Annotations go on the Role and the RoleBinding.
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// RoleName is the name of the entry's Role and RoleBinding.
func (p PrometheusAccess) RoleName(module string) string {
	if p.Path == "" && p.Name == "" {
		return "access-to-" + module
	}

	return rbaccontract.AccessRoleName(module, p.Path, p.Name)
}

// BindingNameOf is the name of the entry's RoleBinding: BindingName, else the Role's name.
func (p PrometheusAccess) BindingNameOf(module string) string {
	if p.BindingName != "" {
		return p.BindingName
	}

	return p.RoleName(module)
}

// File is the template the entry's objects go to, relative to the module root.
func (p PrometheusAccess) File() string {
	if p.Path == "" {
		return "templates/rbac-to-us.yaml"
	}

	return "templates/" + p.Path + "/rbac-to-us.yaml"
}

// PrometheusAccessList is the prometheusAccess section: one entry written as a mapping, the form
// of the first declarations, or a list of entries. A list of one entry is written back as the
// mapping.
type PrometheusAccessList []PrometheusAccess

// prometheusAccessKeys are the keys of an entry, for the unknown-key check the decoder does not do
// below a custom unmarshaller.
var prometheusAccessKeys = yamlKeys(reflect.TypeFor[PrometheusAccess]())

// UnmarshalYAML reads either form. Unknown keys are refused as the strict decoder refuses them
// elsewhere in the file: a node decoded by hand does not inherit its strictness.
func (l *PrometheusAccessList) UnmarshalYAML(node *yaml.Node) error {
	var items []*yaml.Node

	switch node.Kind {
	case yaml.MappingNode:
		items = []*yaml.Node{node}
	case yaml.SequenceNode:
		items = node.Content
	default:
		return &yaml.TypeError{Errors: []string{fmt.Sprintf("line %d: prometheusAccess is an entry (a mapping) or a list of entries", node.Line)}}
	}

	out := make(PrometheusAccessList, 0, len(items))

	var unknown []string

	for _, item := range items {
		if item.Kind != yaml.MappingNode {
			return &yaml.TypeError{Errors: []string{fmt.Sprintf("line %d: an entry of prometheusAccess is a mapping", item.Line)}}
		}

		for i := 0; i+1 < len(item.Content); i += 2 {
			if key := item.Content[i]; !slices.Contains(prometheusAccessKeys, key.Value) {
				unknown = append(unknown, fmt.Sprintf("line %d: field %s not found in type rbacyaml.PrometheusAccess", key.Line, key.Value))
			}
		}

		var entry PrometheusAccess
		if err := item.Decode(&entry); err != nil {
			return err
		}

		out = append(out, entry)
	}

	if len(unknown) > 0 {
		return &yaml.TypeError{Errors: unknown}
	}

	*l = out

	return nil
}

// Sorted returns the entries in the order of their files and names: the root entry of
// access-to-<module> first.
func (l PrometheusAccessList) Sorted() PrometheusAccessList {
	out := slices.Clone(l)
	slices.SortStableFunc(out, func(a, b PrometheusAccess) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}

		return strings.Compare(a.Name, b.Name)
	})

	return out
}

// MarshalYAML writes one entry as the mapping and several as a list.
func (l PrometheusAccessList) MarshalYAML() (any, error) {
	if len(l) == 1 {
		return l[0], nil
	}

	return []PrometheusAccess(l), nil
}

// yamlKeys lists the keys of a struct type as rbac.yaml spells them.
func yamlKeys(t reflect.Type) []string {
	out := make([]string, 0, t.NumField())

	for i := range t.NumField() {
		if name := yamlName(t.Field(i)); t.Field(i).IsExported() && name != "-" {
			out = append(out, name)
		}
	}

	return out
}

// Access grants arbitrary subjects rights on the module. ClusterRules produce a ClusterRole and
// ClusterRoleBinding d8:<module>:<name> in templates/rbac-for-us.yaml; NamespaceRules produce a
// Role and RoleBinding access-to-<module>-<name> in templates/rbac-to-us.yaml, or
// access-to-<path with dashes>-<name> in templates/<path>/rbac-to-us.yaml. Exactly one of the
// two must be set: the placement rule keeps cluster-scoped objects out of rbac-to-us.yaml.
type Access struct {
	Name string `yaml:"name"`
	// RoleName names the ClusterRole or the Role instead of the name derived from Name, and
	// BindingName the binding instead of the role's name: the names the objects had before the
	// declaration, which the bootstrap keeps where the placement rule accepts them.
	RoleName    string    `yaml:"roleName,omitempty"`
	BindingName string    `yaml:"bindingName,omitempty"`
	Subjects    []Subject `yaml:"subjects"`
	// Path is the component directory under templates/ whose rbac-for-us.yaml (clusterRules) or
	// rbac-to-us.yaml (namespaceRules) holds the objects; empty means the module root files.
	Path string `yaml:"path,omitempty"`
	// When wraps the role and the binding in {{- if <When> }}, as for a ServiceAccount.
	When string `yaml:"when,omitempty"`
	// Labels and Annotations go on the role and the binding.
	Labels         map[string]string `yaml:"labels,omitempty"`
	Annotations    map[string]string `yaml:"annotations,omitempty"`
	ClusterRules   []PolicyRule      `yaml:"clusterRules,omitempty"`
	NamespaceRules []PolicyRule      `yaml:"namespaceRules,omitempty"`
}

// ObjectNames returns the role and binding names of the entry: RoleName, else d8:<module>:<name> for
// cluster rules and rbaccontract.AccessRoleName for namespace rules; BindingName, else the role's.
func (a Access) ObjectNames(module string) (string, string) {
	role := a.RoleName
	if role == "" {
		role = rbaccontract.AccessRoleName(module, a.Path, a.Name)
		if len(a.ClusterRules) > 0 {
			role = "d8:" + module + ":" + a.Name
		}
	}

	binding := a.BindingName
	if binding == "" {
		binding = role
	}

	return role, binding
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
