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
	"fmt"
	"maps"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/Masterminds/sprig/v3"

	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbaccontract"
)

// helmFuncs is the function set a `when` expression may use: sprig, as Helm ships it, plus the
// functions Helm's engine adds. Only the names matter here -- the expression is parsed, not
// evaluated; whether it holds under the linter's value stubs is the render's business.
var helmFuncs = func() template.FuncMap {
	funcs := sprig.TxtFuncMap()

	for _, name := range []string{"include", "tpl", "required", "lookup", "toYaml", "fromYaml", "fromYamlArray", "toJson", "fromJson", "fromJsonArray", "toToml"} {
		funcs[name] = func(...any) any { return nil }
	}

	return funcs
}()

// validateWhen rejects a condition that is not a Helm expression: the generator wraps it in
// {{- if <when> }} verbatim, and a typo there breaks the render of the whole file (R13c).
func validateWhen(when, where string, report reporter) {
	if when == "" {
		return
	}

	// The condition is written between "{{- if " and " }}"; a brace pair inside would close that
	// action and put the rest of the value into the file as template text of its own.
	if strings.Contains(when, "{{") || strings.Contains(when, "}}") {
		report("%s: when %q holds a template delimiter; write the condition only, without {{ and }}", where, when)
		return
	}

	tpl, err := template.New("when").Funcs(helmFuncs).Parse("{{ if " + when + " }}{{ end }}")
	if err != nil {
		report("%s: when %q is not a Helm expression: %v", where, when, err)
		return
	}

	// `and not (a) (b)` parses: not is an argument of and, called with no arguments of its own,
	// and the render fails. A function takes its arguments inside parentheses: (not (a)).
	if tpl.Tree != nil && tpl.Tree.Root != nil {
		if name := bareFunction(tpl.Tree.Root); name != "" {
			report("%s: when %q passes %s to another function without its arguments; write (%s ...) in parentheses", where, when, name, name)
		}
	}
}

// bareFunction returns the first function that stands as an argument of another call without
// arguments of its own -- a call with none, which only a function of no parameters survives.
func bareFunction(node parse.Node) string {
	switch n := node.(type) {
	case *parse.ListNode:
		for _, c := range n.Nodes {
			if name := bareFunction(c); name != "" {
				return name
			}
		}
	case *parse.IfNode:
		return bareFunction(n.Pipe)
	case *parse.PipeNode:
		if n == nil {
			return ""
		}

		for _, cmd := range n.Cmds {
			for i, arg := range cmd.Args {
				if id, ok := arg.(*parse.IdentifierNode); ok && i > 0 && !niladic(id.Ident) {
					return id.Ident
				}

				if name := bareFunction(arg); name != "" {
					return name
				}
			}
		}
	}

	return ""
}

// niladic reports whether the function takes no arguments (sprig's now, uuidv4, ...).
func niladic(name string) bool {
	f, ok := helmFuncs[name]
	if !ok {
		return false // a builtin: and, not, eq, len, ... all take arguments
	}

	t := reflect.TypeOf(f)

	return t.Kind() == reflect.Func && t.NumIn() == 0
}

// Validate checks the declaration against the format rules. crds is what the linted tree says
// about the module's own resources (the scope of every CRD under crds/); an entry whose
// resource is not in it is external, and its scope has to be declared. Every problem is
// returned; none is fixed, because a declaration error is a decision the author has to make.
//
// The messages are stable: the sync rule reports them verbatim and the coverage rule and the
// generator refuse to run on a declaration with any of them.
func Validate(d *Declaration, crds CRDScopes) []error {
	var errs []error

	report := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}

	if d.APIVersion != APIVersionV1Alpha1 {
		report("apiVersion must be %q, got %q", APIVersionV1Alpha1, d.APIVersion)
	}

	validateNoTemplateText(reflect.ValueOf(d).Elem(), "", report)

	for _, s := range d.Subsystems {
		if !rbaccontract.IsSubsystem(s) {
			report("subsystems: %q is not a subsystem of the role model (%s)", s, strings.Join(rbaccontract.Subsystems, ", "))
		}
	}

	seen := make(map[string]struct{}, len(d.Resources))
	usedCapabilities := make(map[string]struct{})

	for i := range d.Resources {
		r := &d.Resources[i]

		// The index the author sees in the file, not the one after sorting.
		pos := i
		if d.parsed {
			pos = r.Position
		}

		where := fmt.Sprintf("resources[%d] (%s)", pos, r.Key())

		if _, dup := seen[r.Key()]; dup {
			report("%s: duplicate entry for %s", where, r.Key())
		}

		seen[r.Key()] = struct{}{}

		validateResource(r, where, crds, usedCapabilities, report)
	}

	validateCapabilities(d.Capabilities, usedCapabilities, report)
	validateServiceAccounts(d.ServiceAccounts, report)
	validateAccess(d.Access, report)

	if d.PrometheusAccess != nil {
		if len(d.PrometheusAccess.Deployments)+len(d.PrometheusAccess.DaemonSets)+len(d.PrometheusAccess.StatefulSets) == 0 {
			report("prometheusAccess: names no workload; remove the section or list deployments, daemonsets or statefulsets")
		}

		validateWhen(d.PrometheusAccess.When, "prometheusAccess", report)
		validateMetadataKeys(d.PrometheusAccess.Labels, "prometheusAccess.labels", false, report)
		validateMetadataKeys(d.PrometheusAccess.Annotations, "prometheusAccess.annotations", true, report)
	}

	// Several checks walk maps; the reader and the e2e expectations get one order.
	sort.SliceStable(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })

	return errs
}

type reporter func(format string, args ...any)

func validateResource(r *Resource, where string, crds CRDScopes, usedCapabilities map[string]struct{}, report reporter) {
	if r.Resource == "" {
		report("%s: resource is required", where)
		return
	}

	// Which shape is this entry?
	switch {
	case r.NoAccess != "" && r.HasLevels():
		report("%s: noAccess excludes namespace, system and legacy: an entry either denies access with a reason or grants levels", where)
	case r.NoAccess == "" && !r.HasLevels():
		report("%s: an entry must grant at least one level (namespace, system or legacy) or deny access with noAccess: \"<reason>\"", where)
	}

	validateWhen(r.When, where, report)

	switch {
	case r.Group == "*":
		report("%s: group \"*\" grants the resource in every API group; name the group", where)
	case !groupNameRe.MatchString(r.Group):
		report("%s: group %q is not an API group name (lowercase DNS subdomain; \"\" for the core group)", where, r.Group)
	}

	// RBAC matches "*" as a whole resource name or as the resource of "*/<subresource>"
	// (k8s.io/component-helpers/auth/rbac/validation); anything else is no name Kubernetes knows.
	if !resourceNameRe.MatchString(r.Resource) {
		report("%s: resource %q is not a resource name: a lowercase plural, optionally /<subresource>; \"*\" and \"*/<subresource>\" are the only wildcards", where, r.Resource)
	}

	scope, scopeErr := resolveScope(r, crds)
	if scopeErr != "" {
		report("%s: %s", where, scopeErr)
	}

	// "*/<subresource>" is a wildcard over the resources as much as "*" is.
	if r.IsWildcard() || strings.HasPrefix(r.Resource, "*/") {
		if crds.groupKnown(r.Group) {
			report("%s: resource %q is allowed only for a group the module ships no CRD for; list the resources of %q", where, r.Resource, r.Group)
		}

		if r.Reason == "" {
			report("%s: resource %q requires reason: why the resource names are not known statically", where, r.Resource)
		}
	}

	if scope == ScopeCluster && len(r.Namespace) > 0 {
		report("%s: namespace levels are not allowed for a cluster-scoped resource: a namespace capability is granted through a RoleBinding, where such a rule grants nothing; use system", where)
	}

	if scope == ScopeNamespaced && len(r.System) > 0 && r.Reason == "" {
		report("%s: a namespaced resource granted at a system level is granted across the whole cluster; confirm it with reason: \"<why>\"", where)
	}

	validateLevels(r.Namespace, rbaccontract.LineageNamespace, rbaccontract.NamespaceLevels, where, usedCapabilities, report)
	validateLevels(r.System, rbaccontract.LineageSystem, rbaccontract.SystemLevels, where, usedCapabilities, report)
	validateLevels(r.Legacy, "legacy", rbaccontract.LegacyLevels, where, nil, report)
}

// resolveScope returns the effective scope of the entry: the declared one, checked against the
// CRD when the tree has it; the CRD's when nothing is declared; and an error when neither is
// available. A subresource inherits the scope of its base resource.
func resolveScope(r *Resource, crds CRDScopes) (string, string) {
	if r.Scope != "" && r.Scope != ScopeNamespaced && r.Scope != ScopeCluster {
		return "", fmt.Sprintf("scope must be %q or %q, got %q", ScopeNamespaced, ScopeCluster, r.Scope)
	}

	base := r.Resource
	if i := strings.IndexByte(base, '/'); i >= 0 {
		base = base[:i]
	}

	fromCRD, known := crds[r.Group+"/"+base]

	switch {
	case known && r.Scope != "" && r.Scope != fromCRD:
		return fromCRD, fmt.Sprintf("scope %q disagrees with the CRD in crds/, which says %q", r.Scope, fromCRD)
	case known:
		return fromCRD, ""
	case r.Scope != "":
		// A built-in resource has the scope Kubernetes serves it with; a declared one that differs
		// would generate a capability that grants nothing (or a namespaced grant cluster-wide).
		// namespaces is the exception: a RoleBinding that grants get on namespaces lets its
		// subject read the Namespace it is bound in, so a namespace capability declares it
		// Namespaced on purpose (user-authz's view_resources does).
		if builtin, ok := WellKnownScope(r.Group, r.Resource); ok && builtin != r.Scope && (r.Group != "" || base != "namespaces") {
			return builtin, fmt.Sprintf("scope %q disagrees with Kubernetes, which serves %s as %q", r.Scope, r.Key(), builtin)
		}

		return r.Scope, ""
	default:
		if scope, ok := WellKnownScope(r.Group, r.Resource); ok {
			return scope, ""
		}
	}

	switch {
	case r.NoAccess != "":
		// A denied external resource needs no scope: nothing is generated for it.
		return "", ""
	default:
		return "", "the module ships no CRD for this resource, so scope is required (Namespaced or Cluster)"
	}
}

// groupKnown reports whether any CRD of the tree belongs to the group.
func (c CRDScopes) groupKnown(group string) bool {
	for key := range c {
		if strings.HasPrefix(key, group+"/") {
			return true
		}
	}

	return false
}

func validateLevels(levels map[string][]string, lineage string, allowed []string, where string, usedCapabilities map[string]struct{}, report reporter) {
	for level, verbs := range levels {
		if !slices.Contains(allowed, level) {
			report("%s: %s level %q is not valid; the %s levels are %s", where, lineage, level, lineage, strings.Join(allowed, ", "))
			continue
		}

		if len(verbs) == 0 {
			report("%s: %s.%s lists no verbs", where, lineage, level)
		}

		for _, verb := range verbs {
			switch {
			case verb == "*":
				// No other rule sees a user-facing capability's wildcard: the wildcards rule reads a
				// ServiceAccount's own templates only.
				report("%s: %s.%s grants \"*\", every verb including the ones Kubernetes adds later; list the verbs (%s)", where, lineage, level, strings.Join(rbaccontract.ResourceVerbs, ", "))
			case !slices.Contains(rbaccontract.Verbs, verb):
				report("%s: %s.%s: %q is not a verb; verbs are listed explicitly (%s), there are no aliases", where, lineage, level, verb, strings.Join(rbaccontract.ResourceVerbs, ", "))
			}
		}

		if dup := firstDuplicate(verbs); dup != "" {
			report("%s: %s.%s lists %q twice", where, lineage, level, dup)
		}

		if usedCapabilities != nil {
			usedCapabilities[lineage+"."+rbaccontract.CapabilityAction(level)] = struct{}{}
		}
	}
}

// validateCapabilities requires localized texts for every capability outside the platform
// convention (anything but view/edit) and rejects malformed entries.
func validateCapabilities(texts map[string]CapabilityText, used map[string]struct{}, report reporter) {
	for _, key := range slices.Sorted(maps.Keys(texts)) {
		text := texts[key]

		lineage, action, ok := strings.Cut(key, ".")
		if !ok || (lineage != rbaccontract.LineageNamespace && lineage != rbaccontract.LineageSystem) {
			report("capabilities: key %q must be \"namespace.<level>\" or \"system.<level>\"", key)
			continue
		}

		if rbaccontract.IsConventionalAction(action) {
			report("capabilities: %q needs no texts: view and edit capabilities take the platform's conventional texts", key)
		} else if _, isUsed := used[key]; !isUsed {
			// A text for a level nobody grants -- a typo in the key, or an entry that was removed --
			// produces nothing, and the level it was meant for is left without texts.
			report("capabilities: %q has texts, but no resource entry grants %s level %q; check the key, or drop the texts", key, lineage, action)
		}

		for _, field := range []struct {
			name  string
			value LocalizedText
		}{{"title", text.Title}, {"description", text.Description}} {
			if field.value.EN == "" || field.value.RU == "" {
				report("capabilities: %s.%s requires both en and ru", key, field.name)
			}
		}
	}

	for key := range used {
		_, action, _ := strings.Cut(key, ".")
		if rbaccontract.IsConventionalAction(action) {
			continue
		}

		if _, ok := texts[key]; !ok {
			report("capabilities: %q is used by a resource entry but has no title and description; a capability outside the view/edit convention needs localized texts", key)
		}
	}
}

func validateServiceAccounts(accounts []ServiceAccount, report reporter) {
	names := make(map[string]struct{}, len(accounts))

	for i, sa := range accounts {
		where := fmt.Sprintf("serviceAccounts[%d] (%s)", i, sa.Name)

		if sa.Name == "" {
			report("serviceAccounts[%d]: name is required", i)
			continue
		}

		if len(sa.Name) > 253 || !dnsSubdomainRe.MatchString(sa.Name) {
			report("%s: a ServiceAccount name is a lowercase DNS subdomain (letters, digits, '-' and '.')", where)
		}

		if _, dup := names[sa.Name]; dup {
			report("%s: duplicate name", where)
		}

		names[sa.Name] = struct{}{}

		validateWhen(sa.When, where, report)

		validateMetadataKeys(sa.Labels, where+".labels", false, report)
		validateMetadataKeys(sa.Annotations, where+".annotations", true, report)
		validateMetadataKeys(sa.RBACAnnotations, where+".rbacAnnotations", true, report)

		if strings.HasPrefix(sa.Path, "/") || strings.HasSuffix(sa.Path, "/") || strings.Contains(sa.Path, "..") {
			report("%s: path must be a directory under templates/ without leading or trailing slashes, got %q", where, sa.Path)
		}

		validatePolicyRules(sa.ClusterRules, where+".clusterRules", report)
		validatePolicyRules(sa.NamespaceRules, where+".namespaceRules", report)

		extraNames := make(map[string]struct{}, len(sa.ExtraClusterRoles))

		for j, extra := range sa.ExtraClusterRoles {
			ewhere := fmt.Sprintf("%s.extraClusterRoles[%d]", where, j)

			if extra.Name == "" {
				report("%s: name is required", ewhere)
				continue
			}

			if _, dup := extraNames[extra.Name]; dup {
				report("%s: duplicate name %q", ewhere, extra.Name)
			}

			extraNames[extra.Name] = struct{}{}

			if len(extra.Rules) == 0 {
				report("%s (%s): rules is required", ewhere, extra.Name)
			}

			validatePolicyRules(extra.Rules, ewhere+".rules", report)
		}

		for j, ref := range sa.BindRoles {
			if ref.Namespace == "" || ref.Name == "" {
				report("%s.bindRoles[%d]: namespace and name are required", where, j)
			}
		}

		for j, name := range sa.BindClusterRoles {
			if name == "" {
				report("%s.bindClusterRoles[%d]: empty name", where, j)
			}
		}
	}
}

func validateAccess(access []Access, report reporter) {
	names := make(map[string]struct{}, len(access))

	for i, a := range access {
		where := fmt.Sprintf("access[%d] (%s)", i, a.Name)

		if a.Name == "" {
			report("access[%d]: name is required", i)
			continue
		}

		if _, dup := names[a.Name]; dup {
			report("%s: duplicate name", where)
		}

		names[a.Name] = struct{}{}

		if len(a.Subjects) == 0 {
			report("%s: subjects is required", where)
		}

		validateWhen(a.When, where, report)
		validateMetadataKeys(a.Labels, where+".labels", false, report)
		validateMetadataKeys(a.Annotations, where+".annotations", true, report)

		if strings.HasPrefix(a.Path, "/") || strings.HasSuffix(a.Path, "/") || strings.Contains(a.Path, "..") {
			report("%s: path must be a directory under templates/ without leading or trailing slashes, got %q", where, a.Path)
		}

		seenSubjects := make(map[string]struct{}, len(a.Subjects))

		for j, s := range a.Subjects {
			key := s.Kind + "/" + s.Namespace + "/" + s.Name
			if _, dup := seenSubjects[key]; dup {
				report("%s.subjects[%d]: duplicate subject %s %s", where, j, s.Kind, s.Name)
			}

			seenSubjects[key] = struct{}{}

			switch s.Kind {
			case "User", "Group":
				if s.Namespace != "" {
					report("%s.subjects[%d]: a %s has no namespace", where, j, s.Kind)
				}
			case "ServiceAccount":
				if s.Namespace == "" {
					report("%s.subjects[%d]: a ServiceAccount subject requires namespace", where, j)
				}
			default:
				report("%s.subjects[%d]: kind must be User, Group or ServiceAccount, got %q", where, j, s.Kind)
			}

			if s.Name == "" {
				report("%s.subjects[%d]: name is required", where, j)
			}
		}

		switch {
		case len(a.ClusterRules) > 0 && len(a.NamespaceRules) > 0:
			report("%s: exactly one of clusterRules and namespaceRules: cluster rules go to rbac-for-us.yaml, namespace rules to rbac-to-us.yaml", where)
		case len(a.ClusterRules) == 0 && len(a.NamespaceRules) == 0:
			report("%s: clusterRules or namespaceRules is required", where)
		}

		validatePolicyRules(a.ClusterRules, where+".clusterRules", report)
		validatePolicyRules(a.NamespaceRules, where+".namespaceRules", report)
	}
}

func validatePolicyRules(rules []PolicyRule, where string, report reporter) {
	for i, rule := range rules {
		if len(rule.Verbs) == 0 {
			report("%s[%d]: verbs is required", where, i)
		}

		// An empty string is no verb, resource or name: Kubernetes keeps it and it matches nothing.
		for _, field := range []struct {
			name   string
			values []string
		}{{"verbs", rule.Verbs}, {"resources", rule.Resources}, {"resourceNames", rule.ResourceNames}, {"nonResourceURLs", rule.NonResourceURLs}} {
			if slices.Contains(field.values, "") {
				report("%s[%d]: %s holds an empty value", where, i, field.name)
			}
		}

		if len(rule.NonResourceURLs) > 0 && (len(rule.APIGroups) > 0 || len(rule.Resources) > 0 || len(rule.ResourceNames) > 0) {
			report("%s[%d]: nonResourceURLs cannot be combined with apiGroups, resources or resourceNames", where, i)
		}

		if len(rule.NonResourceURLs) == 0 && len(rule.Resources) == 0 {
			report("%s[%d]: resources (with apiGroups) or nonResourceURLs is required", where, i)
		}

		if len(rule.Resources) > 0 && len(rule.APIGroups) == 0 {
			report("%s[%d]: resources require apiGroups; the core group is \"\"", where, i)
		}
	}
}

func firstDuplicate(values []string) string {
	seen := make(map[string]struct{}, len(values))

	for _, v := range values {
		if _, ok := seen[v]; ok {
			return v
		}

		seen[v] = struct{}{}
	}

	return ""
}

// validateNoTemplateText refuses a template delimiter in any value the generator writes into a
// template, apart from the `when` conditions (validateWhen judges those). Helm would evaluate it:
// a title like "Use {{ .Values.x }}" breaks the render or renders something the declaration does
// not say, and the sync rule then diverges forever.
func validateNoTemplateText(v reflect.Value, path string, report reporter) {
	switch v.Kind() {
	case reflect.String:
		if s := v.String(); strings.Contains(s, "{{") || strings.Contains(s, "}}") {
			report("%s: %q holds a template delimiter; the value is written into a Helm template as it is", strings.TrimPrefix(path, "."), s)
		}
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			validateNoTemplateText(v.Elem(), path, report)
		}
	case reflect.Struct:
		t := v.Type()
		for i := range v.NumField() {
			if t.Field(i).Name == "When" || !t.Field(i).IsExported() {
				continue
			}

			validateNoTemplateText(v.Field(i), path+"."+yamlName(t.Field(i)), report)
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			validateNoTemplateText(v.Index(i), fmt.Sprintf("%s[%d]", path, i), report)
		}
	case reflect.Map:
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j]) })

		for _, k := range keys {
			validateNoTemplateText(k, path, report)
			validateNoTemplateText(v.MapIndex(k), fmt.Sprintf("%s.%v", path, k), report)
		}
	}
}

// yamlName is the key a field has in rbac.yaml, for the messages.
func yamlName(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
	if name == "" {
		return f.Name
	}

	return name
}

// Warnings lists what the declaration may say but likely does not mean. They do not stop
// generation.
func Warnings(d *Declaration) []string {
	var out []string

	for i, r := range d.Resources {
		// SuperAdmin is in the ClusterAuthorizationRule enum, but user-authz aggregates custom
		// legacy roles only for User through ClusterAdmin: d8:user-authz:<module>:super-admin is
		// generated and then ignored by the platform.
		if _, ok := r.Legacy["SuperAdmin"]; ok {
			out = append(out, fmt.Sprintf("resources[%d] (%s): legacy.SuperAdmin produces a role user-authz does not aggregate (it handles User through ClusterAdmin); the grant reaches nobody", i, r.Key()))
		}
	}

	return out
}

var (
	groupNameRe    = regexp.MustCompile(`^$|^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	resourceNameRe = regexp.MustCompile(`^(\*|[a-z0-9]([-a-z0-9]*[a-z0-9])?)(/[a-z0-9]([-a-z0-9]*[a-z0-9])?)?$`)
	dnsSubdomainRe = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
)

// qualifiedNameRe is a Kubernetes label or annotation key: an optional DNS prefix and a name.
var qualifiedNameRe = regexp.MustCompile(`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$`)

// validateMetadataKeys checks label or annotation keys: the generator writes them unquoted, and
// the rbac.deckhouse.io and meta.helm.sh annotations belong to the generator and to Helm.
func validateMetadataKeys(m map[string]string, where string, annotations bool, report reporter) {
	for _, k := range slices.Sorted(maps.Keys(m)) {
		if len(k) > 316 || !qualifiedNameRe.MatchString(k) {
			report("%s: %q is not a valid key ([prefix/]name, the name up to 63 characters of letters, digits, '-', '_' and '.')", where, k)
			continue
		}

		if annotations && (strings.HasPrefix(k, "rbac.deckhouse.io/") || strings.HasPrefix(k, "meta.helm.sh/")) {
			report("%s: %q is set by the generator or by Helm, not by the declaration", where, k)
		}
	}
}
