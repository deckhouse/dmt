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
	"maps"
	"regexp"
	"slices"
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
	// Aggregated marks a ClusterRole with an aggregationRule: its rules belong to the aggregation
	// controller, and the declaration has no place for the selectors.
	Aggregated bool

	// The template blocks around the object, from its template's text (TemplateDocs, Locate):
	// When is the condition it renders under, Unmanageable why the declaration cannot carry that,
	// Partial that a block opens inside it. Located is false when the text did not tell.
	When         string
	Unmanageable string
	Partial      bool
	Located      bool
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
	// Unrendered are the RBAC objects the template text holds that no render showed: under a
	// condition false for the linter's values. The declaration does not hold them.
	Unrendered []string
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
	in  Input
	res map[[2]string]*resourceAcc
	// restricted collects, per resource, the grants limited to resourceNames: the format cannot
	// keep that limit on a capability, and widening a grant is not the importer's call, so they are
	// left out of the levels and named in a note. Keyed by resource only for the note; the verbs of
	// one level never widen another's.
	restricted map[[2]string][]string
	texts      map[string]rbacyaml.CapabilityText
	lineages   map[string]struct{}
	used       map[string]struct{}
	notes      []string
	unmanaged  []string
	decl       *rbacyaml.Declaration
	// prometheusFolded is set once the note on folding several scrape Roles is written.
	prometheusFolded bool
}

func (b *builder) note(format string, args ...any) {
	b.notes = append(b.notes, fmt.Sprintf(format, args...))
}

func (b *builder) unmanage(o Object, why string) {
	b.unmanaged = append(b.unmanaged, fmt.Sprintf("%s (%s): %s", o.identity(), o.Path, why))
}

func (b *builder) mark(o Object) { b.used[o.identity()] = struct{}{} }

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
	// The lint path fills Objects from a map; the notes and the unmanaged list go into the file
	// header in this order, so it is fixed here rather than at every caller.
	in.Objects = slices.Clone(in.Objects)
	sort.SliceStable(in.Objects, func(i, j int) bool {
		return in.Objects[i].identity() < in.Objects[j].identity()
	})

	b := &builder{in: in, res: map[[2]string]*resourceAcc{}, restricted: map[[2]string][]string{}, texts: map[string]rbacyaml.CapabilityText{}, lineages: map[string]struct{}{}, used: map[string]struct{}{},
		decl: &rbacyaml.Declaration{APIVersion: rbacyaml.APIVersionV1Alpha1}}

	for _, u := range in.Unrendered {
		b.note("%s is in the templates but did not render with the linter's values, so this declaration does not hold it -- add it (with its `when`), or lint with --values-file values that render it, before the templates are regenerated", u)
	}

	b.templateBlocks()
	b.capabilitiesAndLegacy()
	b.serviceAccounts()
	b.otherBindings()
	b.leftovers()
	b.resources()

	return Result{Decl: b.decl, Notes: b.notes, Unmanaged: b.unmanaged}
}

// generatedModuleConfigVerbs are the verbs of the moduleconfigs rule the generator adds itself to
// the system view and edit capabilities, on the module's own ModuleConfig only.
var generatedModuleConfigVerbs = map[string][]string{
	"view": {"get", "list", "watch"},
	"edit": {"create", "update", "patch", "delete"},
}

// scopeTODO is the scope bootstrap writes for an external resource whose scope it cannot know.
// A Cluster-scoped resource cannot keep namespace levels: the text says so, since the entry the
// person decides on may hold them.
const scopeTODO = "TODO: Namespaced or Cluster (Cluster drops the namespace levels)"

// wildcardGrant reports whether any rule grants "*" verbs or API groups, which the declaration
// refuses at every level.
func wildcardGrant(rules []rbacv1.PolicyRule) bool {
	for _, r := range rules {
		if slices.Contains(r.Verbs, "*") || slices.Contains(r.APIGroups, "*") {
			return true
		}
	}

	return false
}

func (b *builder) addRules(sectionName, level string, rules []rbacv1.PolicyRule, systemCapability bool) {
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
				if systemCapability && g == "deckhouse.io" && rs == "moduleconfigs" && len(r.ResourceNames) > 0 {
					// The generator adds the module's own ModuleConfig rule to view and edit; that one
					// is not imported. Another one limited to names -- at another level, on another
					// module's config, with other verbs -- the format cannot hold and is named instead
					// of dropped. A grant on every ModuleConfig (no resourceNames, as the deckhouse
					// module has) is an ordinary resource entry.
					own := slices.Equal(r.ResourceNames, []string{b.in.Module})
					if want, conventional := generatedModuleConfigVerbs[rbaccontract.CapabilityAction(level)]; !own || !conventional || !subset(r.Verbs, want) {
						b.note("system/%s: a moduleconfigs rule the generator does not produce (%s on %v) is not carried over; the format has no place for it", level, strings.Join(r.Verbs, ","), r.ResourceNames)
					}

					continue
				}

				key := [2]string{g, rs}

				if b.res[key] == nil {
					b.res[key] = newAcc()
				}

				if len(r.ResourceNames) > 0 {
					b.restricted[key] = append(b.restricted[key], fmt.Sprintf("%s/%s: %s on %v", sectionName, level, strings.Join(r.Verbs, ","), r.ResourceNames))
					continue
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

// templateBlocks keeps out what the declaration cannot describe because of the template around
// it: an object in a range, a with or a define, one rendered by a named template of a library, a
// role without rules. A block inside an object is noted: its rules depend on the values.
func (b *builder) templateBlocks() {
	for _, o := range b.in.Objects {
		switch {
		case o.Unmanageable != "":
			b.unmanage(o, o.Unmanageable)
			b.mark(o)
		case (b.ownClusterRole(o) || o.Kind == "Role") && len(o.Rules) == 0:
			b.unmanage(o, "has no rules with these values; the declaration writes no role without them")
			b.mark(o)
		case o.Partial:
			b.note("%s %s (%s) has a template block inside it: part of it depends on the values, and the declaration holds what rendered with the linter's values -- put `when` on the rules the block gates", o.Kind, o.Name, o.Path)
		case !o.Located && o.Path != "":
			b.note("%s %s (%s) was not found in the text of its template, so whether it renders under a condition is unknown; the declaration writes it unconditionally -- add `when` if the template has one", o.Kind, o.Name, o.Path)
		}
	}
}

// conditional notes a capability or a legacy role under a condition: resources[] have no `when`,
// the regenerated role renders for every value.
func (b *builder) conditional(o Object) {
	if o.When != "" {
		b.note("ClusterRole %s renders under `%s`; resources[] capabilities and legacy roles render unconditionally -- with the condition false, the regenerated role grants what the module does not grant today", o.Name, o.When)
	}
}

func (b *builder) capabilitiesAndLegacy() {
	for _, o := range b.in.Objects {
		if o.Kind != "ClusterRole" || b.isUsed(o) {
			continue
		}

		if level := o.Annotations[rbaccontract.AccessLevelAnnotation]; level != "" {
			if wildcardGrant(o.Rules) {
				why := "grants \"*\" verbs or API groups, which the declaration refuses at every level; the regeneration of " + o.Path + " removes it"
				if level == "SuperAdmin" {
					why += " -- user-authz does not aggregate SuperAdmin, so the role grants nothing today"
				}

				b.unmanage(o, why)
				b.mark(o)

				continue
			}

			if len(o.Rules) == 0 {
				b.note("ClusterRole %s (legacy %s) has no rules and grants nothing; the declaration writes no empty legacy role, so the regeneration drops it", o.Name, level)
			}

			b.rename("ClusterRole", o.Name, "d8:user-authz:"+b.in.Module+":"+rbaccontract.LegacyKebab(level))
			b.addRules("legacy", level, o.Rules, false)
			b.conditional(o)
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
			level := rbaccontract.LevelOfAction(action)

			if wildcardGrant(o.Rules) {
				b.unmanage(o, "grants \"*\" verbs or API groups, which the declaration refuses at every level; list them in the template before the declaration can describe it")
				b.mark(o)

				continue
			}

			b.addRules(lineage, level, o.Rules, lineage == rbaccontract.LineageSystem)
			b.conditional(o)
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
	return o.Kind == "ClusterRole" && !o.Aggregated && o.Labels[rbaccontract.LabelModule] == b.in.Module && o.Labels[rbaccontract.LabelKind] == "" && o.Annotations[rbaccontract.AccessLevelAnnotation] == ""
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
		if b.isUsed(sa) {
			continue
		}

		if b.ns(sa) != b.in.Namespace {
			b.unmanage(sa, "outside the module namespace")
			b.mark(sa)

			continue
		}

		e := rbacyaml.ServiceAccount{Name: sa.Name}

		if m := pathRe.FindStringSubmatch(sa.Path); m != nil {
			e.Path = m[1]
		} else {
			b.note("ServiceAccount %s lives in %s; the generator keeps accounts in templates/[<path>/]rbac-for-us.yaml and will write it to templates/rbac-for-us.yaml", sa.Name, sa.Path)
		}

		e.Labels = copyLabels(sa.Labels)

		if sa.Automount == nil || *sa.Automount {
			yes := true
			e.AutomountToken = &yes

			b.note("ServiceAccount %s mounted its token (automountServiceAccountToken unset or true); kept as true -- set false once its pods mount the token themselves", sa.Name)
		}

		e.Annotations = copyAnnotations(sa.Annotations)
		e.When = sa.When

		b.mark(sa)

		clusterName := "d8:" + b.in.Module + ":" + sa.Name

		// The roles and bindings of the account carry one set of annotations in the format; the
		// first object's set is kept and a differing one is noted.
		var (
			rbacFrom string
			apart    []string
		)

		keep := func(o Object) {
			if o.When != sa.When {
				apart = append(apart, fmt.Sprintf("%s %s renders under `%s`", o.Kind, o.Name, orAlways(o.When)))
			}

			if !maps.Equal(copyLabels(o.Labels), e.Labels) {
				b.note("%s %s carries labels other than ServiceAccount %s; the account's objects share its labels, so the regeneration writes those", o.Kind, o.Name, sa.Name)
			}

			switch {
			case rbacFrom == "":
				rbacFrom = o.Kind + " " + o.Name
				e.RBACAnnotations = copyAnnotations(o.Annotations)
			case !maps.Equal(e.RBACAnnotations, copyAnnotations(o.Annotations)):
				b.note("%s %s carries annotations other than %s; the account's roles and bindings share one set (rbacAnnotations), the set of %s is kept", o.Kind, o.Name, rbacFrom, rbacFrom)
			}
		}

		for _, crb := range b.byKind("ClusterRoleBinding") {
			if b.isUsed(crb) || !subjectsAreOnly(crb, sa.Name, b.in.Namespace) {
				continue
			}

			cr, found := b.clusterRole(crb.RoleRef.Name)
			exclusive := found && !b.isUsed(cr) && b.ownClusterRole(cr) && len(b.bindingsOf(cr.Name)) == 1

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
			case slices.Contains(e.BindClusterRoles, crb.RoleRef.Name):
				// A second binding of the same account to the same role grants nothing more, and the
				// generator names one binding per role: it folds into the first.
				b.note("ClusterRoleBinding %s binds %s to %s again; it folds into %s", crb.Name, sa.Name, crb.RoleRef.Name, clusterName+":"+rbaccontract.BindingSuffix(crb.RoleRef.Name))
			default:
				e.BindClusterRoles = append(e.BindClusterRoles, crb.RoleRef.Name)
				b.rename("ClusterRoleBinding", crb.Name, clusterName+":"+rbaccontract.BindingSuffix(crb.RoleRef.Name))
			}

			if exclusive {
				keep(cr)
				b.mark(cr)
			}

			keep(crb)
			b.mark(crb)
		}

		for _, rb := range b.byKind("RoleBinding") {
			if b.isUsed(rb) || !subjectsAreOnly(rb, sa.Name, b.in.Namespace) {
				continue
			}

			if role, ok := b.role(b.ns(rb), rb.RoleRef.Name); ok && !b.isUsed(role) && b.ns(rb) == b.in.Namespace && e.NamespaceRules == nil && rb.RoleRef.Kind == "Role" {
				e.NamespaceRules = policyRules(role.Rules)
				b.rename("Role", role.Name, rbaccontract.AccountRoleName(b.in.Module, e.Path, sa.Name))
				b.rename("RoleBinding", rb.Name, rbaccontract.AccountRoleName(b.in.Module, e.Path, sa.Name))
				keep(role)
				b.mark(role)
			} else if rb.RoleRef.Kind != "Role" {
				// bindRoles binds Roles; the format has no RoleBinding to a ClusterRole, and turning it
				// into one to a Role of that name would bind nothing (Kubernetes accepts a binding to a
				// Role that does not exist).
				b.unmanage(rb, "a RoleBinding to the ClusterRole "+rb.RoleRef.Name+", which bindRoles cannot express")
			} else if ref := (rbacyaml.RoleRef{Namespace: b.ns(rb), Name: rb.RoleRef.Name}); slices.Contains(e.BindRoles, ref) {
				b.note("RoleBinding %s/%s binds %s to the Role %s again; it folds into one", b.ns(rb), rb.Name, sa.Name, rb.RoleRef.Name)
			} else {
				e.BindRoles = append(e.BindRoles, rbacyaml.RoleRef{Namespace: b.ns(rb), Name: rb.RoleRef.Name})
				b.rename("RoleBinding", b.ns(rb)+"/"+rb.Name, b.ns(rb)+"/"+rbaccontract.AccountForeignBindingPrefix(b.in.Module, e.Path, sa.Name)+":"+rbaccontract.BindingSuffix(rb.RoleRef.Name))
			}

			if rb.RoleRef.Kind == "Role" {
				keep(rb)
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

			keep(cr)
			b.mark(cr)
		}

		// The declaration puts every object of an account under the account's condition; a role or
		// a binding under another one is a decision for a person.
		if len(apart) > 0 {
			e.When = fmt.Sprintf("TODO: the account renders under `%s`, but %s; the declaration puts all of them under one condition", orAlways(sa.When), strings.Join(apart, ", "))
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
		if !found || !b.ownClusterRole(cr) || b.isUsed(cr) {
			b.unmanage(crb, fmt.Sprintf("binds %s, which is not a plain ClusterRole of this module the declaration describes", crb.RoleRef.Name))
			continue
		}

		name := strings.TrimPrefix(cr.Name, "d8:"+b.in.Module+":")
		b.decl.Access = append(b.decl.Access, rbacyaml.Access{Name: name, Path: componentOf(cr.Path, "rbac-for-us.yaml"), When: accessWhen(cr, crb),
			Labels: b.sharedLabels(cr, crb), Annotations: b.sharedAnnotations(cr, crb), Subjects: subjects(crb.Subjects), ClusterRules: policyRules(cr.Rules)})
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
		if !ok || b.isUsed(role) || b.ns(rb) != b.in.Namespace || rb.RoleRef.Kind != "Role" {
			b.unmanage(rb, fmt.Sprintf("binds %s/%s outside the module's own Roles", rb.RoleRef.Kind, rb.RoleRef.Name))
			continue
		}

		if b.prometheus(role, rb) {
			continue
		}

		path := componentOf(role.Path, "rbac-to-us.yaml")
		name := rb.Name

		// The entry name is what follows the placement prefix: access-to-<directory>- in a
		// component file, access-to-<module>- (with or without the directory) at the root.
		for _, prefix := range []string{
			"access-to-" + b.in.Module + "-" + strings.ReplaceAll(path, "/", "-") + "-",
			"access-to-" + strings.ReplaceAll(path, "/", "-") + "-",
			"access-to-" + b.in.Module + "-",
		} {
			if trimmed, ok := strings.CutPrefix(name, prefix); ok && trimmed != "" && (path != "" || prefix == "access-to-"+b.in.Module+"-") {
				name = trimmed
				break
			}
		}

		b.decl.Access = append(b.decl.Access, rbacyaml.Access{Name: name, Path: path, When: accessWhen(role, rb),
			Labels: b.sharedLabels(role, rb), Annotations: b.sharedAnnotations(role, rb), Subjects: subjects(rb.Subjects), NamespaceRules: policyRules(role.Rules)})
		b.rename("Role", role.Name, rbaccontract.AccessRoleName(b.in.Module, path, name))
		b.rename("RoleBinding", rb.Name, rbaccontract.AccessRoleName(b.in.Module, path, name))
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

	first := b.decl.PrometheusAccess == nil
	if first {
		b.decl.PrometheusAccess = &rbacyaml.PrometheusAccess{When: rb.When, Labels: b.sharedLabels(role, rb), Annotations: b.sharedAnnotations(role, rb)}

		if !rb.Located {
			b.note("prometheusAccess: the template of RoleBinding %s could not be read, so whether it gated the scraper binding is unknown; add `when: .Values.global.enabledModules | has \"prometheus\"` if it did", rb.Name)
		}
	} else if !b.prometheusFolded {
		b.prometheusFolded = true
		b.note("several Prometheus access Roles fold into one prometheusAccess (Role access-to-%s)", b.in.Module)
	}

	pa := b.decl.PrometheusAccess

	if !first && rb.When != pa.When && !strings.HasPrefix(pa.When, "TODO") {
		pa.When = fmt.Sprintf("TODO: the scraper bindings render under different conditions (`%s`, `%s`); prometheusAccess has one", orAlways(pa.When), orAlways(rb.When))
	}

	if role.When != "" {
		b.note("Role %s renders under `%s`; prometheusAccess writes the Role unconditionally and gates only the binding", role.Name, role.When)
	}

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
			// The CRD in crds/ is the source of the scope (ADR); writing it out would be a second
			// copy that validation has to keep in step. A CRD that only an edition overlay ships
			// is the one case where a lint of the base directory asks for scope: the author adds
			// it then, as the validation message says.
		case resource == "*":
			e.Scope, scope = rbacyaml.ScopeCluster, rbacyaml.ScopeCluster
			e.Reason = "TODO: the templates grant the whole group; say why the resource names are not known statically"
		default:
			if s, ok := rbacyaml.WellKnownScope(group, base); ok {
				scope = s
			} else {
				// A TODO value, not only a note: the run that writes the file keeps its finding, and
				// the declaration does not validate until a person fills it.
				scope = scopeTODO
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

		// A grant limited to resourceNames is never widened to every object, at any level: it
		// stays out of the entry and is named for the author. When nothing else grants the
		// resource, the entry is left undecided and coverage keeps the run red until it is.
		if restricted := b.restricted[k]; len(restricted) > 0 {
			sort.Strings(restricted)

			if !e.HasLevels() {
				e = rbacyaml.Resource{Group: group, Resource: resource, Scope: e.Scope,
					NoAccess: "TODO: the templates limited this grant to specific resourceNames, which the format cannot express; grant the levels to every object or keep denying"}
				b.note("%s/%s: every grant carried resourceNames; left as noAccess TODO instead of widening it to every object (%s)", group, resource, strings.Join(restricted, "; "))
			} else {
				b.note("%s/%s: grants limited to resourceNames are not carried over, the format would grant them on every object: %s", group, resource, strings.Join(restricted, "; "))
			}
		}

		if !e.HasLevels() && e.NoAccess == "" {
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
			// No scope: the CRD in crds/ carries it, as for every CRD-backed entry above.
			b.decl.Resources = append(b.decl.Resources, rbacyaml.Resource{Group: group, Resource: plural,
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

// subset reports whether every item of a is in b.
func subset(a, b []string) bool {
	for _, x := range a {
		if !slices.Contains(b, x) {
			return false
		}
	}

	return true
}

// copyAnnotations returns the annotations a declaration can carry: Helm's own meta.helm.sh keys
// are stamped at install time and the rbac.deckhouse.io keys are the generator's.
func copyAnnotations(in map[string]string) map[string]string {
	var out map[string]string

	for k, v := range in {
		if strings.HasPrefix(k, "meta.helm.sh/") || strings.HasPrefix(k, "rbac.deckhouse.io/") {
			continue
		}

		if out == nil {
			out = make(map[string]string, len(in))
		}

		out[k] = v
	}

	return out
}

// orAlways names an empty condition in a note.
func orAlways(when string) string {
	if when == "" {
		return "no condition"
	}

	return when
}

// accessWhen is the condition of an access entry: its role and its binding render together, or
// the entry holds a decision for a person.
func accessWhen(role, binding Object) string {
	if role.When == binding.When {
		return binding.When
	}

	return fmt.Sprintf("TODO: %s %s renders under `%s`, %s %s under `%s`; the access entry has one condition", role.Kind, role.Name, orAlways(role.When), binding.Kind, binding.Name, orAlways(binding.When))
}

// copyLabels returns the labels a declaration carries: helm_lib_module_labels writes heritage and
// module on every object itself.
func copyLabels(in map[string]string) map[string]string {
	var out map[string]string

	for k, v := range in {
		if k == "heritage" || k == "module" {
			continue
		}

		if out == nil {
			out = make(map[string]string, len(in))
		}

		out[k] = v
	}

	return out
}

// sharedLabels are the labels of an entry whose role and binding the declaration writes with one
// set; the binding's set is kept, and a role with another is noted.
func (b *builder) sharedLabels(role, binding Object) map[string]string {
	labels := copyLabels(binding.Labels)
	if !maps.Equal(labels, copyLabels(role.Labels)) {
		b.note("%s %s and %s %s carry different labels; the entry has one set, the binding's is kept", role.Kind, role.Name, binding.Kind, binding.Name)
	}

	return labels
}

// sharedAnnotations is sharedLabels for annotations.
func (b *builder) sharedAnnotations(role, binding Object) map[string]string {
	annotations := copyAnnotations(binding.Annotations)
	if !maps.Equal(annotations, copyAnnotations(role.Annotations)) {
		b.note("%s %s and %s %s carry different annotations; the entry has one set, the binding's is kept", role.Kind, role.Name, binding.Kind, binding.Name)
	}

	return annotations
}
