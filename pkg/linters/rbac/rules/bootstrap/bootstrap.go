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

// Package bootstrap writes the first rbac.yaml of a module from the RBAC objects it renders today:
// the declaration a person would have transcribed from the templates by hand, with a note wherever
// a decision is still theirs. It is the entry point of an existing module into the declaration;
// from then on rbac.yaml is the source and the templates follow it.
package bootstrap

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbaccontract"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbacyaml"
)

// Object is one rendered RBAC object or ServiceAccount.
type Object struct {
	Kind, Name, Namespace string
	// Path is the template the object came from, relative to the module root.
	Path        string
	Labels      map[string]string
	Annotations map[string]string
	Rules       []rbacv1.PolicyRule
	RoleRef     rbacv1.RoleRef
	Subjects    []rbacv1.Subject
	Automount   *bool
}

// Input is what the render says about the module.
type Input struct {
	Module, Namespace string
	// Subsystems are the module.yaml subsystems; the declaration overrides them only when the
	// rendered system capabilities aggregate into a different set.
	Subsystems []string
	Objects    []Object
	// CRDs maps group/plural to scope for the CRDs under crds/.
	CRDs map[string]string
}

// Result is the declaration with the reader's homework.
type Result struct {
	Decl *rbacyaml.Declaration
	// Notes are decisions the reader must check: scopes the linter could not tell, grants kept as
	// TODO, objects the generator will name differently.
	Notes []string
	// Unmanaged are the RBAC objects the declaration cannot describe; they stay hand-written.
	Unmanaged []string
}

var capabilityRe = regexp.MustCompile(`^d8:(namespace|system)-capability:([a-z0-9-]+):([a-z0-9_]+)$`)

type resourceAcc struct {
	namespace, system, legacy map[string]map[string]struct{}
}

func newAcc() *resourceAcc {
	return &resourceAcc{namespace: map[string]map[string]struct{}{}, system: map[string]map[string]struct{}{}, legacy: map[string]map[string]struct{}{}}
}

func addVerbs(into map[string]map[string]struct{}, level string, verbs []string) {
	if into[level] == nil {
		into[level] = map[string]struct{}{}
	}

	for _, v := range verbs {
		into[level][v] = struct{}{}
	}
}

func sortedLevels(m map[string]map[string]struct{}) map[string][]string {
	if len(m) == 0 {
		return nil
	}

	out := make(map[string][]string, len(m))

	for level, verbs := range m {
		list := make([]string, 0, len(verbs))
		for v := range verbs {
			list = append(list, v)
		}

		sort.Strings(list)
		out[level] = list
	}

	return out
}

type builder struct {
	in        Input
	res       map[[2]string]*resourceAcc
	texts     map[string]rbacyaml.CapabilityText
	lineages  map[string]struct{}
	used      map[string]struct{}
	notes     []string
	unmanaged []string
	decl      *rbacyaml.Declaration
}

func (b *builder) note(format string, args ...any) {
	b.notes = append(b.notes, fmt.Sprintf(format, args...))
}
func (b *builder) unmanage(o Object, why string) {
	b.unmanaged = append(b.unmanaged, fmt.Sprintf("%s (%s): %s", o.identity(), o.Path, why))
}
func (b *builder) mark(o Object)        { b.used[o.identity()] = struct{}{} }
func (b *builder) isUsed(o Object) bool { _, ok := b.used[o.identity()]; return ok }
func (b *builder) ns(o Object) string {
	if o.Namespace == "" {
		return b.in.Namespace
	}

	return o.Namespace
}
func (b *builder) rename(kind, from, to string) {
	if from != to {
		b.note("%s %s will be named %s by the generator", kind, from, to)
	}
}

func (o Object) identity() string {
	if o.Namespace != "" {
		return o.Namespace + "/" + o.Kind + "/" + o.Name
	}

	return o.Kind + "/" + o.Name
}

// Build derives the declaration.
func Build(in Input) Result {
	b := &builder{in: in, res: map[[2]string]*resourceAcc{}, texts: map[string]rbacyaml.CapabilityText{}, lineages: map[string]struct{}{}, used: map[string]struct{}{},
		decl: &rbacyaml.Declaration{APIVersion: rbacyaml.APIVersionV1Alpha1}}

	b.capabilitiesAndLegacy()
	b.serviceAccounts()
	b.otherBindings()
	b.leftovers()
	b.resources()

	return Result{Decl: b.decl, Notes: b.notes, Unmanaged: b.unmanaged}
}

func (b *builder) addRules(sectionName, level string, rules []rbacv1.PolicyRule, skipModuleConfigs bool) {
	for _, r := range rules {
		if len(r.NonResourceURLs) > 0 {
			b.note("%s/%s: a nonResourceURLs rule (%s) has no place in resources[]; keep it in a ServiceAccount's clusterRules", sectionName, level, strings.Join(r.NonResourceURLs, ", "))
			continue
		}

		groups := r.APIGroups
		if len(groups) == 0 {
			groups = []string{""}
		}

		for _, g := range groups {
			for _, rs := range r.Resources {
				if skipModuleConfigs && g == "deckhouse.io" && rs == "moduleconfigs" {
					continue // the generator adds it
				}

				if len(r.ResourceNames) > 0 {
					b.note("%s/%s: %s/%s was limited to resourceNames %v; the format has no resourceNames for capabilities, the grant is now on every object", sectionName, level, g, rs, r.ResourceNames)
				}

				key := [2]string{g, rs}
				if b.res[key] == nil {
					b.res[key] = newAcc()
				}

				switch sectionName {
				case rbaccontract.LineageNamespace:
					addVerbs(b.res[key].namespace, level, r.Verbs)
				case rbaccontract.LineageSystem:
					addVerbs(b.res[key].system, level, r.Verbs)
				default:
					addVerbs(b.res[key].legacy, level, r.Verbs)
				}
			}
		}
	}
}

func (b *builder) capabilitiesAndLegacy() {
	for _, o := range b.in.Objects {
		if o.Kind != "ClusterRole" {
			continue
		}

		if level := o.Annotations[rbaccontract.AccessLevelAnnotation]; level != "" {
			b.rename("ClusterRole", o.Name, "d8:user-authz:"+b.in.Module+":"+rbaccontract.LegacyKebab(level))
			b.addRules("legacy", level, o.Rules, false)
			b.mark(o)

			continue
		}

		switch o.Labels[rbaccontract.LabelKind] {
		case rbaccontract.KindCapability:
			m := capabilityRe.FindStringSubmatch(o.Name)
			if m == nil || m[2] != b.in.Module {
				b.unmanage(o, "a capability the declaration cannot produce (not d8:<namespace|system>-capability:"+b.in.Module+":<action>)")
				b.mark(o)

				continue
			}

			lineage, action := m[1], m[3]
			level := action

			for lvl, act := range map[string]string{"viewer": "view", "manager": "edit"} {
				if act == action {
					level = lvl
				}
			}

			b.addRules(lineage, level, o.Rules, lineage == rbaccontract.LineageSystem)
			b.mark(o)

			if lineage == rbaccontract.LineageSystem {
				for key := range o.Labels {
					if strings.HasPrefix(key, rbaccontract.AggregationLabelPrefix) && strings.HasSuffix(key, rbaccontract.AggregationLabelSuffix) {
						b.lineages[strings.TrimSuffix(strings.TrimPrefix(key, rbaccontract.AggregationLabelPrefix), rbaccontract.AggregationLabelSuffix)] = struct{}{}
					}
				}
			}

			if !rbaccontract.IsConventionalAction(action) {
				b.texts[lineage+"."+action] = rbacyaml.CapabilityText{
					Title:       rbacyaml.LocalizedText{EN: o.Annotations[rbaccontract.AnnotationTitleEN], RU: o.Annotations[rbaccontract.AnnotationTitleRU]},
					Description: rbacyaml.LocalizedText{EN: o.Annotations[rbaccontract.AnnotationDescriptionEN], RU: o.Annotations[rbaccontract.AnnotationDescriptionRU]},
				}
			}
		case rbaccontract.KindRole:
			b.unmanage(o, "a role of the role model")
			b.mark(o)
		case rbaccontract.KindLegacyUse, rbaccontract.KindLegacyManage:
			b.unmanage(o, "a capability of the RBACv2 scheme before DKP 1.78; run rbacv2-migrate-module.sh first")
			b.mark(o)
		}
	}
}

func (b *builder) byKind(kind string) []Object {
	out := make([]Object, 0, len(b.in.Objects))

	for _, o := range b.in.Objects {
		if o.Kind == kind {
			out = append(out, o)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].identity() < out[j].identity() })

	return out
}

func (b *builder) clusterRole(name string) (Object, bool) {
	for _, o := range b.in.Objects {
		if o.Kind == "ClusterRole" && o.Name == name {
			return o, true
		}
	}

	return Object{}, false
}

func (b *builder) role(ns, name string) (Object, bool) {
	for _, o := range b.in.Objects {
		if o.Kind == "Role" && o.Name == name && b.ns(o) == ns {
			return o, true
		}
	}

	return Object{}, false
}

// ownClusterRole reports whether the ClusterRole is the module's own plain one: labelled with the
// module, neither a capability nor a role of the model nor a legacy role.
func (b *builder) ownClusterRole(o Object) bool {
	return o.Kind == "ClusterRole" && o.Labels[rbaccontract.LabelModule] == b.in.Module && o.Labels[rbaccontract.LabelKind] == "" && o.Annotations[rbaccontract.AccessLevelAnnotation] == ""
}

func (b *builder) bindingsOf(name string) []Object {
	out := make([]Object, 0, len(b.in.Objects))

	for _, o := range b.in.Objects {
		if o.Kind == "ClusterRoleBinding" && o.RoleRef.Name == name {
			out = append(out, o)
		}
	}

	return out
}

func subjectsAreOnly(o Object, saName, saNS string) bool {
	if len(o.Subjects) == 0 {
		return false
	}

	for _, s := range o.Subjects {
		if s.Kind != "ServiceAccount" || s.Name != saName || (s.Namespace != "" && s.Namespace != saNS) {
			return false
		}
	}

	return true
}

var pathRe = regexp.MustCompile(`^templates/(?:(.*)/)?rbac-for-us\.yaml$`)

func (b *builder) serviceAccounts() {
	for _, sa := range b.byKind("ServiceAccount") {
		if b.ns(sa) != b.in.Namespace {
			b.unmanage(sa, "outside the module namespace")
			continue
		}

		e := rbacyaml.ServiceAccount{Name: sa.Name}

		if m := pathRe.FindStringSubmatch(sa.Path); m != nil {
			e.Path = m[1]
		} else {
			b.note("ServiceAccount %s lives in %s; the generator keeps accounts in templates/[<path>/]rbac-for-us.yaml and will write it to templates/rbac-for-us.yaml", sa.Name, sa.Path)
		}

		if app := sa.Labels["app"]; app != "" {
			e.Labels = map[string]string{"app": app}
		}

		if sa.Automount == nil || *sa.Automount {
			yes := true
			e.AutomountToken = &yes

			b.note("ServiceAccount %s mounted its token (automountServiceAccountToken unset or true); kept as true -- set false once its pods mount the token themselves", sa.Name)
		}

		b.mark(sa)

		clusterName := "d8:" + b.in.Module + ":" + sa.Name

		for _, crb := range b.byKind("ClusterRoleBinding") {
			if b.isUsed(crb) || !subjectsAreOnly(crb, sa.Name, b.in.Namespace) {
				continue
			}

			cr, found := b.clusterRole(crb.RoleRef.Name)
			exclusive := found && b.ownClusterRole(cr) && len(b.bindingsOf(cr.Name)) == 1

			switch {
			case exclusive && cr.Name == clusterName && e.ClusterRules == nil:
				e.ClusterRules = policyRules(cr.Rules)

				b.rename("ClusterRoleBinding", crb.Name, clusterName)
			case exclusive:
				extra := rbacyaml.ExtraClusterRole{Name: strings.TrimPrefix(cr.Name, clusterName+":"), Rules: policyRules(cr.Rules)}
				if !strings.HasPrefix(cr.Name, clusterName+":") && !strings.HasPrefix(cr.Name, "d8:") {
					b.rename("ClusterRole", cr.Name, extra.FullName(b.in.Module, sa.Name))
				}

				e.ExtraClusterRoles = append(e.ExtraClusterRoles, extra)
				b.rename("ClusterRoleBinding", crb.Name, extra.FullName(b.in.Module, sa.Name))
			default:
				e.BindClusterRoles = append(e.BindClusterRoles, crb.RoleRef.Name)
				b.rename("ClusterRoleBinding", crb.Name, clusterName+":"+bindingSuffix(crb.RoleRef.Name))
			}

			if found && (exclusive) {
				b.mark(cr)
			}

			b.mark(crb)
		}

		for _, rb := range b.byKind("RoleBinding") {
			if b.isUsed(rb) || !subjectsAreOnly(rb, sa.Name, b.in.Namespace) {
				continue
			}

			if role, ok := b.role(b.ns(rb), rb.RoleRef.Name); ok && b.ns(rb) == b.in.Namespace && e.NamespaceRules == nil && rb.RoleRef.Kind == "Role" {
				e.NamespaceRules = policyRules(role.Rules)
				b.rename("Role", role.Name, sa.Name)
				b.rename("RoleBinding", rb.Name, sa.Name)
				b.mark(role)
			} else {
				e.BindRoles = append(e.BindRoles, rbacyaml.RoleRef{Namespace: b.ns(rb), Name: rb.RoleRef.Name})
				b.rename("RoleBinding", b.ns(rb)+"/"+rb.Name, b.ns(rb)+"/"+clusterName+":"+bindingSuffix(rb.RoleRef.Name))
			}

			b.mark(rb)
		}

		// Unbound ClusterRoles of the module in the account's file: roles shipped for others to
		// bind (an aggregated apiserver's requester). They travel with the account, unbound.
		for _, cr := range b.byKind("ClusterRole") {
			if b.isUsed(cr) || !b.ownClusterRole(cr) || cr.Path != sa.Path || len(b.bindingsOf(cr.Name)) != 0 {
				continue
			}

			no := false
			extra := rbacyaml.ExtraClusterRole{Name: strings.TrimPrefix(cr.Name, clusterName+":"), Rules: policyRules(cr.Rules), Bind: &no}

			if !strings.HasPrefix(cr.Name, clusterName+":") && !strings.HasPrefix(cr.Name, "d8:") {
				b.rename("ClusterRole", cr.Name, extra.FullName(b.in.Module, sa.Name))
			}

			e.ExtraClusterRoles = append(e.ExtraClusterRoles, extra)

			b.mark(cr)
		}

		if n := len(e.ExtraClusterRoles); n > 1 {
			b.note("ServiceAccount %s has %d ClusterRoles of its own besides d8:%s:%s; they are kept separate as extraClusterRoles -- merge them into clusterRules if nothing binds them separately", sa.Name, n, b.in.Module, sa.Name)
		}

		b.decl.ServiceAccounts = append(b.decl.ServiceAccounts, e)
	}
}

// otherBindings turns bindings to subjects other than the module's accounts into access entries
// and the Prometheus scrape access.
func (b *builder) otherBindings() {
	for _, crb := range b.byKind("ClusterRoleBinding") {
		if b.isUsed(crb) {
			continue
		}

		cr, found := b.clusterRole(crb.RoleRef.Name)
		if !found || !b.ownClusterRole(cr) {
			b.unmanage(crb, fmt.Sprintf("binds %s, which is not a plain ClusterRole of this module", crb.RoleRef.Name))
			continue
		}

		name := strings.TrimPrefix(cr.Name, "d8:"+b.in.Module+":")
		b.decl.Access = append(b.decl.Access, rbacyaml.Access{Name: name, Path: componentOf(cr.Path, "rbac-for-us.yaml"), Subjects: subjects(crb.Subjects), ClusterRules: policyRules(cr.Rules)})
		b.rename("ClusterRoleBinding", crb.Name, "d8:"+b.in.Module+":"+name)
		b.rename("ClusterRole", cr.Name, "d8:"+b.in.Module+":"+name)
		b.mark(cr)
		b.mark(crb)
	}

	for _, rb := range b.byKind("RoleBinding") {
		if b.isUsed(rb) {
			continue
		}

		role, ok := b.role(b.ns(rb), rb.RoleRef.Name)
		if !ok || b.ns(rb) != b.in.Namespace || rb.RoleRef.Kind != "Role" {
			b.unmanage(rb, fmt.Sprintf("binds %s/%s outside the module's own Roles", rb.RoleRef.Kind, rb.RoleRef.Name))
			continue
		}

		if b.prometheus(role, rb) {
			continue
		}

		name := strings.TrimPrefix(rb.Name, "access-to-"+b.in.Module+"-")
		b.decl.Access = append(b.decl.Access, rbacyaml.Access{Name: name, Path: componentOf(role.Path, "rbac-to-us.yaml"), Subjects: subjects(rb.Subjects), NamespaceRules: policyRules(role.Rules)})
		b.rename("Role", role.Name, "access-to-"+b.in.Module+"-"+name)
		b.rename("RoleBinding", rb.Name, "access-to-"+b.in.Module+"-"+name)
		b.mark(role)
		b.mark(rb)
	}
}

// prometheus recognizes the scrape access: a Role of <kind>/prometheus-metrics rules bound to the
// scraper. Several such Roles fold into one prometheusAccess.
func (b *builder) prometheus(role, rb Object) bool {
	scraper := false

	for _, s := range rb.Subjects {
		if s.Name == "d8-monitoring:scraper" || (s.Kind == "ServiceAccount" && s.Name == "prometheus" && s.Namespace == "d8-monitoring") {
			scraper = true
		}
	}

	if !scraper || len(role.Rules) == 0 {
		return false
	}

	for _, r := range role.Rules {
		for _, res := range r.Resources {
			if !strings.HasSuffix(res, "/prometheus-metrics") {
				return false
			}
		}
	}

	if b.decl.PrometheusAccess == nil {
		b.decl.PrometheusAccess = &rbacyaml.PrometheusAccess{}
	} else {
		b.note("several Prometheus access Roles fold into one prometheusAccess (Role access-to-%s)", b.in.Module)
	}

	pa := b.decl.PrometheusAccess

	for _, r := range role.Rules {
		for _, res := range r.Resources {
			switch strings.TrimSuffix(res, "/prometheus-metrics") {
			case "deployments":
				pa.Deployments = append(pa.Deployments, r.ResourceNames...)
			case "daemonsets":
				pa.DaemonSets = append(pa.DaemonSets, r.ResourceNames...)
			case "statefulsets":
				pa.StatefulSets = append(pa.StatefulSets, r.ResourceNames...)
			}
		}
	}

	sort.Strings(pa.Deployments)
	sort.Strings(pa.DaemonSets)
	sort.Strings(pa.StatefulSets)
	b.rename("Role", role.Name, "access-to-"+b.in.Module)
	b.rename("RoleBinding", rb.Name, "access-to-"+b.in.Module)
	b.mark(role)
	b.mark(rb)

	return true
}

func (b *builder) leftovers() {
	for _, o := range b.in.Objects {
		if b.isUsed(o) {
			continue
		}

		switch o.Kind {
		case "ClusterRole":
			b.unmanage(o, "bound to nothing the declaration describes")
		case "Role":
			b.unmanage(o, "bound to nothing the declaration describes")
		case "ClusterRoleBinding", "RoleBinding", "ServiceAccount":
			b.unmanage(o, "not described")
		}
	}
}

func (b *builder) resources() {
	keys := make([][2]string, 0, len(b.res))
	for k := range b.res {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}

		return keys[i][1] < keys[j][1]
	})

	for _, k := range keys {
		group, resource := k[0], k[1]
		acc := b.res[k]
		e := rbacyaml.Resource{Group: group, Resource: resource}

		base := resource
		if i := strings.IndexByte(base, '/'); i >= 0 {
			base = base[:i]
		}

		scope, fromCRD := b.in.CRDs[group+"/"+base]

		switch {
		case fromCRD:
			// Written out even though the CRD says it: a lint of one edition directory does not see
			// the CRDs of the other editions, and an entry without a scope would then stop the
			// whole validation instead of raising one coverage warning.
			if !strings.Contains(resource, "/") {
				e.Scope = scope
			}
		case resource == "*":
			e.Scope, scope = rbacyaml.ScopeCluster, rbacyaml.ScopeCluster
			e.Reason = "TODO: the templates grant the whole group; say why the resource names are not known statically"
		default:
			if s, ok := rbacyaml.WellKnownScope(group, base); ok {
				scope = s
			} else {
				b.note("%s/%s: the module ships no CRD and the scope is not known; fill scope: Namespaced|Cluster", group, resource)
			}

			if !strings.Contains(resource, "/") {
				e.Scope = scope
			}
		}

		if len(acc.namespace) > 0 && scope == rbacyaml.ScopeCluster {
			b.note("%s/%s: cluster-scoped, yet granted in a namespace capability -- the rule granted nothing through a RoleBinding and is dropped from namespace", group, resource)

			acc.namespace = map[string]map[string]struct{}{}
		}

		if len(acc.system) > 0 && scope == rbacyaml.ScopeNamespaced && e.Reason == "" {
			e.Reason = "TODO: a namespaced resource granted cluster-wide, as the templates did; confirm or move it to namespace"
		}

		e.Namespace, e.System, e.Legacy = sortedLevels(acc.namespace), sortedLevels(acc.system), sortedLevels(acc.legacy)
		if !e.HasLevels() {
			continue
		}

		b.decl.Resources = append(b.decl.Resources, e)
	}

	crdKeys := make([]string, 0, len(b.in.CRDs))
	for k := range b.in.CRDs {
		crdKeys = append(crdKeys, k)
	}

	sort.Strings(crdKeys)

	for _, k := range crdKeys {
		group, plural, _ := strings.Cut(k, "/")
		declared := false

		for _, r := range b.decl.Resources {
			if r.Group == group && r.Resource == plural {
				declared = true
			}
		}

		if !declared {
			// The scope is written out for the same reason as above: a lint of one edition directory
			// that lacks this CRD must read the entry as a documented external resource, not as a
			// stale one.
			b.decl.Resources = append(b.decl.Resources, rbacyaml.Resource{Group: group, Resource: plural, Scope: b.in.CRDs[k],
				NoAccess: "TODO: no user-facing access in the templates today; grant levels or say why users get none"})
		}
	}

	if len(b.lineages) > 0 {
		got := make([]string, 0, len(b.lineages))
		for l := range b.lineages {
			got = append(got, l)
		}

		sort.Strings(got)

		want := append([]string(nil), b.in.Subsystems...)
		sort.Strings(want)

		if strings.Join(got, ",") != strings.Join(want, ",") {
			b.decl.Subsystems = got
			b.note("system capabilities aggregate into %v while module.yaml says %v; subsystems is set explicitly", got, want)
		}
	}

	if len(b.texts) > 0 {
		b.decl.Capabilities = b.texts
	}
}

// componentOf returns the component directory of templates/<component>/<file>, "" for the root file
// or any other template.
func componentOf(path, file string) string {
	rest, ok := strings.CutPrefix(path, "templates/")
	if !ok || !strings.HasSuffix(rest, "/"+file) {
		return ""
	}

	return strings.TrimSuffix(rest, "/"+file)
}

func policyRules(rules []rbacv1.PolicyRule) []rbacyaml.PolicyRule {
	out := make([]rbacyaml.PolicyRule, 0, len(rules))
	for _, r := range rules {
		out = append(out, rbacyaml.PolicyRule{APIGroups: r.APIGroups, Resources: r.Resources, ResourceNames: r.ResourceNames, NonResourceURLs: r.NonResourceURLs, Verbs: r.Verbs})
	}

	return out
}

func subjects(list []rbacv1.Subject) []rbacyaml.Subject {
	out := make([]rbacyaml.Subject, 0, len(list))
	for _, s := range list {
		out = append(out, rbacyaml.Subject{Kind: s.Kind, Name: s.Name, Namespace: s.Namespace})
	}

	return out
}

func bindingSuffix(roleName string) string {
	return strings.ReplaceAll(strings.TrimPrefix(roleName, "d8:"), ":", "-")
}
