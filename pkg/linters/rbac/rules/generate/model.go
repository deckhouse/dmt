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

// Package generate turns a module RBAC declaration (rbac.yaml) into the RBAC objects the module
// ships and into the Helm templates that produce them. The model is built first and the text is
// rendered from it, so that the sync rule compares the rendered chart against the very objects the
// generator would write.
//
// What is produced follows the ADR "Единый rbac.yaml модуля", "Что генерируется": one file per
// capability under templates/rbacv2/{use,manage}/, the legacy roles in
// templates/user-authz-cluster-roles.yaml, the ServiceAccount rights in templates/[<path>/]rbac-for-us.yaml
// and the external access in templates/rbac-to-us.yaml.
package generate

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbaccontract"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbacyaml"
)

// Input is everything the generator needs besides the declaration: the module identity from
// module.yaml. Subsystems are module.yaml's unless the declaration overrides them.
type Input struct {
	Module     string
	Namespace  string
	Subsystems []string
	Decl       *rbacyaml.Declaration
}

// Class is one of the three classes of objects the sync rule owns (ADR, "Область ответственности sync").
type Class string

const (
	// ClassLegacy is a legacy (user-authz v1) ClusterRole, recognized by its access-level annotation.
	ClassLegacy Class = "legacy"
	// ClassCapability is an RBACv2 capability of the module, recognized by kind: capability and the
	// module label.
	ClassCapability Class = "capability"
	// ClassDeclared is an object whose name the generator builds from serviceAccounts, access and
	// prometheusAccess; it is owned only when declared.
	ClassDeclared Class = "declared"
)

// Rule is one PolicyRule of an object, with the condition it is rendered under ("" for always).
type Rule struct {
	rbacyaml.PolicyRule
	When string
}

// Subject is an RBAC subject of a binding.
type Subject struct {
	Kind      string
	Name      string
	Namespace string
}

// Object is one RBAC object the generator produces.
type Object struct {
	Kind      string
	Name      string
	Namespace string
	Class     Class

	// When is the condition the whole object is rendered under ("" for always).
	When string

	// Labels are the labels the generator sets besides the module labels helm_lib_module_labels adds
	// (heritage, module). For capabilities these carry the contract: kind, scope, marker, aggregation.
	Labels map[string]string
	// Annotations are the localized texts of a capability or the access level of a legacy role.
	Annotations map[string]string

	Rules []Rule

	// Binding fields.
	RoleRefKind string
	RoleRefName string
	Subjects    []Subject

	// ServiceAccount fields.
	AutomountToken *bool
}

// Identity returns Kind/Name or Namespace/Kind/Name, matching storage.ResourceIndex.AsString.
func (o Object) Identity() string {
	if o.Namespace == "" {
		return o.Kind + "/" + o.Name
	}

	return o.Namespace + "/" + o.Kind + "/" + o.Name
}

// AggregationLabels returns the (lineage, level) pairs of a capability, sorted by lineage.
func (o Object) AggregationLabels() []LineageLevel {
	out := make([]LineageLevel, 0, len(o.Labels))

	for key, value := range o.Labels {
		if strings.HasPrefix(key, rbaccontract.AggregationLabelPrefix) && strings.HasSuffix(key, rbaccontract.AggregationLabelSuffix) {
			out = append(out, LineageLevel{
				Lineage: strings.TrimSuffix(strings.TrimPrefix(key, rbaccontract.AggregationLabelPrefix), rbaccontract.AggregationLabelSuffix),
				Level:   value,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Lineage < out[j].Lineage })

	return out
}

// LineageLevel is one aggregation edge of a capability.
type LineageLevel struct {
	Lineage string
	Level   string
}

// File is one generated template with its objects in order.
type File struct {
	// Path is relative to the module root.
	Path    string
	Objects []Object
}

// Model is the full set of generated files, sorted by path.
type Model struct {
	Files []File
}

// File returns the file at path, or nil.
func (m *Model) File(path string) *File {
	for i := range m.Files {
		if m.Files[i].Path == path {
			return &m.Files[i]
		}
	}

	return nil
}

// Paths returns the generated paths in order.
func (m *Model) Paths() []string {
	out := make([]string, 0, len(m.Files))
	for _, f := range m.Files {
		out = append(out, f.Path)
	}

	return out
}

// Build derives the object model from the declaration. The declaration must have passed
// rbacyaml.Validate: Build trusts it.
func Build(in Input) (*Model, error) {
	if in.Decl == nil {
		return nil, errors.New("no declaration")
	}

	if in.Module == "" {
		return nil, errors.New("the module name is required")
	}

	if err := checkAgainstModule(in); err != nil {
		return nil, err
	}

	b := &builder{in: in, files: map[string]*File{}}

	b.capabilities()
	b.legacyRoles()
	b.serviceAccounts()
	b.access()

	model := &Model{Files: make([]File, 0, len(b.files))}
	for _, f := range b.files {
		model.Files = append(model.Files, *f)
	}

	sort.Slice(model.Files, func(i, j int) bool { return model.Files[i].Path < model.Files[j].Path })

	// A marker past 63 characters fails the contract; only a long module name can cause it.
	for _, f := range model.Files {
		seen := make(map[string]struct{}, len(f.Objects))

		for _, o := range f.Objects {
			if _, dup := seen[o.Identity()]; dup {
				return nil, fmt.Errorf("%s would hold two objects named %s: two declared roles or bindings map to the same generated name", f.Path, o.Identity())
			}

			seen[o.Identity()] = struct{}{}
		}

		for _, o := range f.Objects {
			if marker := o.Labels[rbaccontract.LabelCapability]; len(marker) > 63 {
				return nil, fmt.Errorf("capability marker %q is %d characters, a label value holds 63: the module name and the level name together are too long for %s", marker, len(marker), o.Name)
			}
		}
	}

	return model, nil
}

// checkAgainstModule refuses what the declaration alone cannot know is wrong: it needs the module's
// metadata, and the objects it would produce would fail the platform's other rules.
func checkAgainstModule(in Input) error {
	systemLevels := false

	for _, r := range in.Decl.Resources {
		if len(r.System) > 0 {
			systemLevels = true
		}
	}

	if systemLevels && len(in.Decl.Subsystems) == 0 && len(in.Subsystems) == 0 {
		return fmt.Errorf("system levels are declared but the module aggregates into no subsystem: module.yaml declares none, so set subsystems in %s", rbacyaml.Filename)
	}

	for _, sa := range in.Decl.ServiceAccounts {
		if sa.Path == "" {
			continue
		}

		if strings.Contains(sa.Path, "/") {
			return fmt.Errorf("serviceAccounts[%s].path %q: one directory under templates/ only; the placement rule names the objects of a nested directory in a way the generator cannot follow", sa.Name, sa.Path)
		}

		if sa.Name != sa.Path && sa.Name != in.Module+"-"+sa.Path {
			return fmt.Errorf("serviceAccounts[%s].path %q: the placement rule wants the account named %q or %q after its directory; rename the account or move it to the module root", sa.Name, sa.Path, sa.Path, in.Module+"-"+sa.Path)
		}
	}

	return nil
}

type builder struct {
	in    Input
	files map[string]*File
}

func (b *builder) add(path string, obj Object) {
	f, ok := b.files[path]
	if !ok {
		f = &File{Path: path}
		b.files[path] = f
	}

	f.Objects = append(f.Objects, obj)
}

func (b *builder) subsystems() []string {
	if len(b.in.Decl.Subsystems) > 0 {
		return b.in.Decl.Subsystems
	}

	out := append([]string(nil), b.in.Subsystems...)
	sort.Strings(out)

	return out
}

// capabilities produces one namespace capability per namespace level in use and the system
// capabilities: view and edit always (every module gets access to its own ModuleConfig), the
// other levels when a resource names them.
func (b *builder) capabilities() {
	namespaceLevels := map[string][]Rule{}
	systemLevels := map[string][]Rule{}

	for _, res := range b.in.Decl.Resources {
		for level, verbs := range res.Namespace {
			namespaceLevels[level] = append(namespaceLevels[level], resourceRule(res, verbs))
		}

		for level, verbs := range res.System {
			systemLevels[level] = append(systemLevels[level], resourceRule(res, verbs))
		}
	}

	for _, level := range rbaccontract.NamespaceLevels {
		rules, ok := namespaceLevels[level]
		if !ok {
			continue
		}

		action := rbaccontract.CapabilityAction(level)
		b.add("templates/rbacv2/use/"+action+".yaml", Object{
			Kind:  "ClusterRole",
			Name:  rbaccontract.NamespaceCapabilityPrefix + b.in.Module + ":" + action,
			Class: ClassCapability,
			Labels: map[string]string{
				rbaccontract.LabelKind:       rbaccontract.KindCapability,
				rbaccontract.LabelScope:      rbaccontract.LineageNamespace,
				rbaccontract.LabelCapability: rbaccontract.LineageNamespace + "-capability." + b.in.Module + "." + action,
				rbaccontract.AggregationLabelPrefix + rbaccontract.LineageNamespace + rbaccontract.AggregationLabelSuffix: level,
			},
			Annotations: b.texts(rbaccontract.LineageNamespace, action),
			Rules:       sortRules(rules),
		})
	}

	for _, level := range rbaccontract.SystemLevels {
		action := rbaccontract.CapabilityAction(level)
		rules := systemLevels[level]

		switch action {
		case "view":
			rules = append(rules, moduleConfigRule(b.in.Module, []string{"get", "list", "watch"}))
		case "edit":
			rules = append(rules, moduleConfigRule(b.in.Module, []string{"create", "update", "patch", "delete"}))
		default:
			if len(rules) == 0 {
				continue
			}
		}

		labels := map[string]string{
			rbaccontract.LabelKind:       rbaccontract.KindCapability,
			rbaccontract.LabelScope:      rbaccontract.LineageSystem,
			rbaccontract.LabelCapability: rbaccontract.LineageSystem + "-capability." + b.in.Module + "." + action,
		}

		for _, subsystem := range b.subsystems() {
			labels[rbaccontract.AggregationLabelPrefix+subsystem+rbaccontract.AggregationLabelSuffix] = level
		}

		if strings.HasPrefix(b.in.Namespace, "d8-") {
			labels[rbaccontract.LabelNamespace] = b.in.Namespace
		}

		b.add("templates/rbacv2/manage/"+action+".yaml", Object{
			Kind:        "ClusterRole",
			Name:        rbaccontract.SystemCapabilityPrefix + b.in.Module + ":" + action,
			Class:       ClassCapability,
			Labels:      labels,
			Annotations: b.texts(rbaccontract.LineageSystem, action),
			Rules:       sortRules(rules),
		})
	}
}

// texts returns the four localized annotations of a capability: the platform convention for
// view/edit, the declaration's capabilities entry otherwise (Validate made sure it exists).
func (b *builder) texts(lineage, action string) map[string]string {
	var title, description rbaccontract.Text

	if conventional, ok := rbaccontract.ConventionalTexts[lineage+"."+action]; ok {
		title = rbaccontract.Text{EN: fmt.Sprintf(conventional.Title.EN, b.in.Module), RU: fmt.Sprintf(conventional.Title.RU, b.in.Module)}
		description = rbaccontract.Text{EN: fmt.Sprintf(conventional.Description.EN, b.in.Module), RU: fmt.Sprintf(conventional.Description.RU, b.in.Module)}
	} else if custom, ok := b.in.Decl.Capabilities[lineage+"."+action]; ok {
		title = rbaccontract.Text{EN: custom.Title.EN, RU: custom.Title.RU}
		description = rbaccontract.Text{EN: custom.Description.EN, RU: custom.Description.RU}
	}

	return map[string]string{
		rbaccontract.AnnotationTitleEN:       title.EN,
		rbaccontract.AnnotationTitleRU:       title.RU,
		rbaccontract.AnnotationDescriptionEN: description.EN,
		rbaccontract.AnnotationDescriptionRU: description.RU,
	}
}

// legacyRoles produces one legacy ClusterRole per access level in use, in the enum order.
func (b *builder) legacyRoles() {
	byLevel := map[string][]Rule{}

	for _, res := range b.in.Decl.Resources {
		for level, verbs := range res.Legacy {
			byLevel[level] = append(byLevel[level], resourceRule(res, verbs))
		}
	}

	for _, level := range rbaccontract.LegacyLevels {
		rules, ok := byLevel[level]
		if !ok {
			continue
		}

		b.add("templates/user-authz-cluster-roles.yaml", Object{
			Kind:        "ClusterRole",
			Name:        rbaccontract.LegacyRolePrefix + b.in.Module + ":" + rbaccontract.LegacyKebab(level),
			Class:       ClassLegacy,
			Annotations: map[string]string{rbaccontract.AccessLevelAnnotation: level},
			Rules:       sortRules(rules),
		})
	}
}

// serviceAccounts produces the ServiceAccount, its ClusterRole/ClusterRoleBinding, Role/RoleBinding
// and the extra bindings, into templates/[<path>/]rbac-for-us.yaml.
func (b *builder) serviceAccounts() {
	accounts := append([]rbacyaml.ServiceAccount(nil), b.in.Decl.ServiceAccounts...)
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].Name < accounts[j].Name })

	for _, sa := range accounts {
		path := "templates/rbac-for-us.yaml"
		if sa.Path != "" {
			path = "templates/" + sa.Path + "/rbac-for-us.yaml"
		}

		labels := map[string]string{}
		for k, v := range sa.Labels {
			labels[k] = v
		}

		automount := sa.AutomountToken != nil && *sa.AutomountToken
		b.add(path, Object{
			Kind: "ServiceAccount", Name: sa.Name, Namespace: b.in.Namespace, Class: ClassDeclared,
			When: sa.When, Labels: labels, AutomountToken: &automount,
		})

		subject := []Subject{{Kind: "ServiceAccount", Name: sa.Name, Namespace: b.in.Namespace}}
		clusterName := "d8:" + b.in.Module + ":" + sa.Name

		if len(sa.ClusterRules) > 0 {
			b.add(path, Object{Kind: "ClusterRole", Name: clusterName, Class: ClassDeclared, When: sa.When, Labels: labels, Rules: policyRules(sa.ClusterRules)})
			b.add(path, Object{Kind: "ClusterRoleBinding", Name: clusterName, Class: ClassDeclared, When: sa.When, Labels: labels, RoleRefKind: "ClusterRole", RoleRefName: clusterName, Subjects: subject})
		}

		if len(sa.NamespaceRules) > 0 {
			b.add(path, Object{Kind: "Role", Name: sa.Name, Namespace: b.in.Namespace, Class: ClassDeclared, When: sa.When, Labels: labels, Rules: policyRules(sa.NamespaceRules)})
			b.add(path, Object{Kind: "RoleBinding", Name: sa.Name, Namespace: b.in.Namespace, Class: ClassDeclared, When: sa.When, Labels: labels, RoleRefKind: "Role", RoleRefName: sa.Name, Subjects: subject})
		}

		for _, extra := range sa.ExtraClusterRoles {
			extraName := extra.FullName(b.in.Module, sa.Name)
			b.add(path, Object{Kind: "ClusterRole", Name: extraName, Class: ClassDeclared, When: sa.When, Labels: labels, Rules: policyRules(extra.Rules)})

			if extra.IsBound() {
				b.add(path, Object{Kind: "ClusterRoleBinding", Name: extraName, Class: ClassDeclared, When: sa.When, Labels: labels, RoleRefKind: "ClusterRole", RoleRefName: extraName, Subjects: subject})
			}
		}

		for _, bound := range sa.BindClusterRoles {
			b.add(path, Object{
				Kind: "ClusterRoleBinding", Name: clusterName + ":" + rbaccontract.BindingSuffix(bound), Class: ClassDeclared, When: sa.When, Labels: labels,
				RoleRefKind: "ClusterRole", RoleRefName: bound, Subjects: subject,
			})
		}

		for _, ref := range sa.BindRoles {
			b.add(path, Object{
				Kind: "RoleBinding", Name: clusterName + ":" + rbaccontract.BindingSuffix(ref.Name), Namespace: ref.Namespace, Class: ClassDeclared, When: sa.When, Labels: labels,
				RoleRefKind: "Role", RoleRefName: ref.Name, Subjects: subject,
			})
		}
	}
}

// access produces the Prometheus access and the arbitrary-subject grants: cluster rules go to
// templates/rbac-for-us.yaml (the placement rule keeps ClusterRoles there), namespace rules and the
// metrics access to templates/rbac-to-us.yaml.
func (b *builder) access() {
	if pa := b.in.Decl.PrometheusAccess; pa != nil {
		var rules []Rule

		for kind, names := range map[string][]string{"deployments": pa.Deployments, "daemonsets": pa.DaemonSets, "statefulsets": pa.StatefulSets} {
			if len(names) == 0 {
				continue
			}

			sorted := append([]string(nil), names...)
			sort.Strings(sorted)

			rules = append(rules, Rule{PolicyRule: rbacyaml.PolicyRule{
				APIGroups: []string{"apps"}, Resources: []string{kind + "/prometheus-metrics"}, ResourceNames: sorted, Verbs: []string{"get"},
			}})
		}

		rules = sortRules(rules)
		name := "access-to-" + b.in.Module

		b.add("templates/rbac-to-us.yaml", Object{Kind: "Role", Name: name, Namespace: b.in.Namespace, Class: ClassDeclared, Rules: rules})
		b.add("templates/rbac-to-us.yaml", Object{
			Kind: "RoleBinding", Name: name, Namespace: b.in.Namespace, Class: ClassDeclared, RoleRefKind: "Role", RoleRefName: name,
			Subjects: []Subject{{Kind: "User", Name: "d8-monitoring:scraper"}, {Kind: "ServiceAccount", Name: "prometheus", Namespace: "d8-monitoring"}},
		})
	}

	grants := append([]rbacyaml.Access(nil), b.in.Decl.Access...)
	sort.Slice(grants, func(i, j int) bool { return grants[i].Name < grants[j].Name })

	for _, a := range grants {
		subjects := make([]Subject, 0, len(a.Subjects))
		for _, s := range a.Subjects {
			subjects = append(subjects, Subject{Kind: s.Kind, Name: s.Name, Namespace: s.Namespace})
		}

		dir := "templates/"
		if a.Path != "" {
			dir = "templates/" + a.Path + "/"
		}

		if len(a.ClusterRules) > 0 {
			name := "d8:" + b.in.Module + ":" + a.Name
			b.add(dir+"rbac-for-us.yaml", Object{Kind: "ClusterRole", Name: name, Class: ClassDeclared, Rules: policyRules(a.ClusterRules)})
			b.add(dir+"rbac-for-us.yaml", Object{Kind: "ClusterRoleBinding", Name: name, Class: ClassDeclared, RoleRefKind: "ClusterRole", RoleRefName: name, Subjects: subjects})
		}

		if len(a.NamespaceRules) > 0 {
			name := "access-to-" + b.in.Module + "-" + a.Name
			b.add(dir+"rbac-to-us.yaml", Object{Kind: "Role", Name: name, Namespace: b.in.Namespace, Class: ClassDeclared, Rules: policyRules(a.NamespaceRules)})
			b.add(dir+"rbac-to-us.yaml", Object{Kind: "RoleBinding", Name: name, Namespace: b.in.Namespace, Class: ClassDeclared, RoleRefKind: "Role", RoleRefName: name, Subjects: subjects})
		}
	}
}

func resourceRule(res rbacyaml.Resource, verbs []string) Rule {
	sorted := append([]string(nil), verbs...)
	sort.Strings(sorted)

	return Rule{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{res.Group}, Resources: []string{res.Resource}, Verbs: sorted}, When: res.When}
}

func moduleConfigRule(module string, verbs []string) Rule {
	sorted := append([]string(nil), verbs...)
	sort.Strings(sorted)

	return Rule{PolicyRule: rbacyaml.PolicyRule{APIGroups: []string{"deckhouse.io"}, Resources: []string{"moduleconfigs"}, ResourceNames: []string{module}, Verbs: sorted}}
}

// policyRules copies raw rules as they are; their condition is the object's, not their own.
func policyRules(rules []rbacyaml.PolicyRule) []Rule {
	out := make([]Rule, 0, len(rules))
	for _, r := range rules {
		out = append(out, Rule{PolicyRule: r})
	}

	return out
}

// sortRules orders rules by group, then resource: the order of a generated file never depends on the
// order of the declaration. The ModuleConfig rule sorts with the rest (deckhouse.io).
func sortRules(rules []Rule) []Rule {
	out := append([]Rule(nil), rules...)
	sort.SliceStable(out, func(i, j int) bool {
		gi, gj := strings.Join(out[i].APIGroups, ","), strings.Join(out[j].APIGroups, ",")
		if gi != gj {
			return gi < gj
		}

		ri, rj := strings.Join(out[i].Resources, ","), strings.Join(out[j].Resources, ",")
		if ri != rj {
			return ri < rj
		}

		return strings.Join(out[i].NonResourceURLs, ",") < strings.Join(out[j].NonResourceURLs, ",")
	})

	return out
}
