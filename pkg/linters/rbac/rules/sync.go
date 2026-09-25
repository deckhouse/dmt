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
	stderrors "errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/bootstrap"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/generate"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbaccontract"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbacyaml"
)

const SyncRuleName = "sync"

// SyncRule compares the RBAC objects the chart renders with the ones rbac.yaml declares, in both
// directions, and on --fix rewrites a file the declaration produces from the declaration, unless the
// lint finds a case in it only a change of the templates or the declaration closes. It owns three classes
// of rendered objects (ADR, "Область ответственности sync"): legacy roles, the module's RBACv2
// capabilities, and the objects whose names the generator builds; everything else in the render is
// unmanaged and never reported.
//
// It runs only when the module has an rbac.yaml. A declaration that does not validate is reported
// and nothing else is compared or generated (spec 005 R14). A rule declared under `when` that is
// absent from the render is not a divergence (R13a); a rule declared without `when` that is absent
// is (R13b) -- the condition belongs to the declaration.
type SyncRule struct {
	pkg.RuleMeta
	pkg.KindRule

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*SyncRule)(nil)

func NewSyncRule(excludeRules []pkg.KindRuleExclude, m pkg.Module, errorList *errors.LintRuleErrorsList) *SyncRule {
	return &SyncRule{
		RuleMeta:  pkg.RuleMeta{Name: SyncRuleName},
		KindRule:  pkg.KindRule{ExcludeRules: excludeRules},
		module:    m,
		errorList: errorList.WithRule(SyncRuleName),
	}
}

// moduleMetadata is the part of module.yaml the generator reads.
type moduleMetadata struct {
	Subsystems []string `json:"subsystems"`
}

func readModuleMetadata(modulePath string) (moduleMetadata, error) {
	var meta moduleMetadata

	data, err := os.ReadFile(filepath.Join(modulePath, "module.yaml"))
	if err != nil {
		if stderrors.Is(err, os.ErrNotExist) {
			return meta, nil
		}

		return meta, err
	}

	// The subsystems decide the aggregation edges of every system capability; a module.yaml that
	// does not parse must stop the rule, not strip them.
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return meta, fmt.Errorf("parse module.yaml: %w", err)
	}

	return meta, nil
}

func (r *SyncRule) Check(_ context.Context) {
	modulePath := r.module.GetPath()
	declList := r.errorList.WithFilePath(rbacyaml.Filename)

	decl, err := rbacyaml.Load(modulePath)
	overlay := editionOverlay(modulePath)

	if stderrors.Is(err, rbacyaml.ErrNotFound) {
		// An overlay carries no declaration of its own: the one in the base directory describes
		// the union of editions, so there is nothing to write here.
		if overlay == "" {
			r.bootstrap(declList)
		}

		return
	}

	if overlay != "" {
		declList.Errorf("%s lies in the edition overlay %s; the declaration describes the union of editions and belongs to modules/<module>/ only -- CI merges the overlays over modules/ before linting, so a copy here would shadow it or go unseen: move the file",
			rbacyaml.Filename, overlay)

		return
	}

	if err != nil {
		if content, readErr := os.ReadFile(rbacyaml.Path(modulePath)); readErr == nil && !strings.Contains(string(content), "apiVersion:") {
			declList.Errorf("%s is not a declaration (no apiVersion): an rbac.yaml of an earlier shape that nothing reads; delete it and run `%s` to write the declaration from the render", rbacyaml.Filename, FixCommand)
			return
		}

		declList.Errorf("%v; nothing is compared or generated until the declaration parses", err)

		return
	}

	// A CRD document that does not parse is reported by coverage; the declaration is judged
	// against the CRDs that do.
	crds, _ := moduleCRDs(modulePath)

	meta, err := readModuleMetadata(modulePath)
	if err != nil {
		r.errorList.WithFilePath("module.yaml").Errorf("%v; nothing is compared or generated until it parses: its subsystems decide the aggregation of every system capability", err)
		return
	}

	if errs := rbacyaml.Validate(decl, crdScopes(crds)); len(errs) > 0 {
		for _, e := range errs {
			declList.Errorf("%v; nothing is compared or generated until the declaration is valid", e)
		}

		return
	}

	for _, w := range rbacyaml.Warnings(decl) {
		declList.Warnf("%s", w)
	}

	model, err := generate.Build(generate.Input{
		Module:     r.module.GetName(),
		Namespace:  r.module.GetNamespace(),
		Subsystems: meta.Subsystems,
		Decl:       decl,
	})
	if err != nil {
		declList.Errorf("cannot derive the RBAC objects from the declaration: %v", err)
		return
	}

	run := r.newSyncRun(modulePath, model)
	divergences := r.compareRender(run)
	compareFiles(run, divergences)
	r.report(run, divergences)
}

// syncRun is what one Check computes once and every step reads: the model, the rendered objects the
// rule owns, where the declaration puts each object, and the text of every template.
type syncRun struct {
	modulePath string
	model      *generate.Model
	actual     map[string]managedObject
	// placed maps every identity the declaration produces to the file it puts the object in.
	placed map[string]string
	// templates holds the text of every template, by path relative to the module.
	templates map[string]templateText
	// rendered is the identity of every rendered object; fromFile the rendered objects by template.
	rendered map[string]struct{}
	fromFile map[string]map[string]storage.StoreObject
}

// templateText is one template as the lint reads it; err is why it could not be read.
type templateText struct {
	content string
	docs    []textDocument
	err     error
}

func (r *SyncRule) newSyncRun(modulePath string, model *generate.Model) *syncRun {
	run := &syncRun{
		modulePath: modulePath,
		model:      model,
		actual:     r.managedObjects(model),
		placed:     map[string]string{},
		templates:  templateTexts(modulePath),
		rendered:   map[string]struct{}{},
		fromFile:   map[string]map[string]storage.StoreObject{},
	}

	for _, file := range model.Files {
		for _, o := range file.Objects {
			run.placed[o.Identity()] = file.Path
		}
	}

	for index, object := range r.module.GetStorage() {
		id := index.AsString()
		run.rendered[id] = struct{}{}

		if run.fromFile[object.ShortPath()] == nil {
			run.fromFile[object.ShortPath()] = map[string]storage.StoreObject{}
		}

		run.fromFile[object.ShortPath()][id] = object
	}

	return run
}

// compareRender judges the declaration against the rendered objects, both ways: every produced
// object must be rendered as produced, and every legacy role or module capability rendered must be
// produced. The findings are collected per template.
func (r *SyncRule) compareRender(run *syncRun) map[string][]string {
	legacy := r.legacyFiles()
	divergences := map[string][]string{}

	for _, file := range run.model.Files {
		kind, isLegacy := legacy[file.Path]

		// A template that serves both models behind the version gate rendered its legacy branch:
		// the values of this run say the cluster is below 1.78. The 1.78 objects it declares are
		// conditional on the gate and are compared in the run where the gate answers "new" -- the
		// default one -- so this is not a divergence (the same reading as a rule under when, D4).
		if isLegacy && templateGated(run.templates[file.Path].content, "") {
			continue
		}

		found := compareFile(r.enabledObjects(run.withoutUnrendered(file)), run.actual, r.module.GetName())

		// The template renders the scheme before 1.78 where the declaration produces the new one:
		// the objects the declaration names cannot be there. Say so once instead of listing them.
		if isLegacy && len(found) > 0 {
			found = append(found, fmt.Sprintf("the template renders the legacy RBACv2 scheme (%s: %s, the manage/use model before DKP 1.78) where the declaration produces the 1.78 model; migrate the module with rbacv2-migrate-module.sh to serve both, or delete the file and run `%s` to serve the new one only",
				rbaccontract.LabelKind, kind, FixCommand))
		}

		divergences[file.Path] = append(divergences[file.Path], found...)
	}

	divergences = r.replacedCopies(run, divergences)

	// A produced object that renders from another template than the one the declaration puts it
	// in is misplaced: both files are reported, and neither is rewritten while it renders there.
	for path, objects := range run.fromFile {
		for id, object := range objects {
			where, produced := run.placed[id]
			if !produced || path == where || !r.Enabled(object.Unstructured.GetKind(), object.Unstructured.GetName()) {
				continue
			}

			divergences[where] = append(divergences[where], id+" renders from "+path+"; the declaration puts it in this file")
			divergences[path] = append(divergences[path], id+" renders here; the declaration puts it in "+where)
		}
	}

	// A legacy role or a module capability the declaration does not produce is an object the
	// declaration must own: it is reported under the template it came from.
	for identity, obj := range run.actual {
		if _, produced := run.placed[identity]; produced || obj.class == generate.ClassDeclared {
			continue
		}

		// exclude-rules.sync silences the finding, not the object: an excluded object is still
		// known to the rule, so a declared counterpart is not reported as absent either.
		if !r.Enabled(obj.object.Unstructured.GetKind(), obj.object.Unstructured.GetName()) {
			continue
		}

		why := "declare its rights in rbac.yaml or remove it from the template"

		kind := obj.object.Unstructured.GetKind()
		if rules, found, _ := unstructured.NestedSlice(obj.object.Unstructured.Object, "rules"); (kind == "Role" || kind == "ClusterRole") && (!found || len(rules) == 0) {
			// A role without rules grants nothing and has nothing to declare.
			why = "it has no rules and grants nothing, so the fix drops it -- remove it from the template"
		}

		divergences[obj.object.ShortPath()] = append(divergences[obj.object.ShortPath()],
			fmt.Sprintf("%s is in the render but rbac.yaml does not produce it: %s", identity, why))
	}

	return divergences
}

// compareFiles reports a file the declaration produces that does not exist while an object it
// holds is absent from the render: the render cannot tell a conditional object whose condition is
// false from one whose template was never written, but the file system can (D4 covers the render,
// not the file).
func compareFiles(run *syncRun, divergences map[string][]string) {
	for _, file := range run.model.Files {
		_, err := os.Stat(filepath.Join(run.modulePath, file.Path))

		switch {
		case err == nil:
		case !stderrors.Is(err, os.ErrNotExist):
			divergences[file.Path] = append(divergences[file.Path], fmt.Sprintf("the file cannot be read: %v", err))
		case hasAbsentObject(file, run.actual):
			divergences[file.Path] = append(divergences[file.Path],
				"the file does not exist, and objects the declaration puts in it are absent from the render (objects under `when` included: no template produces them)")
		}
	}
}

// report emits one finding per template. A file the declaration produces gets the fix that rewrites
// it from the declaration, unless the lint finds a case only a change of the templates or of the
// declaration closes: then the finding names the case and carries no fix. A template the
// declaration produces nothing for is a finding without a fix: declare what it renders or delete
// it, the autofix deletes no file.
func (r *SyncRule) report(run *syncRun, divergences map[string][]string) {
	// Every variant judges every produced file, reported or not: under --matrix the fix of one
	// variant must not rewrite a file another variant found a case in.
	cases := make(map[string][]string, len(run.model.Files))

	for _, file := range run.model.Files {
		if found := r.unfixable(run, file); len(found) > 0 {
			cases[file.Path] = found
			withholdFix(filepath.Join(run.modulePath, file.Path))
		}
	}

	paths := make([]string, 0, len(divergences))
	for path, list := range divergences {
		if len(list) > 0 {
			paths = append(paths, path)
		}
	}

	sort.Strings(paths)

	for _, path := range paths {
		list := divergences[path]
		sort.Strings(list)

		fileList := r.errorList.WithFilePath(path).WithObjectID(path)

		file := run.model.File(path)

		switch {
		case file == nil:
			fileList.Errorf("%s does not match %s: %s", path, rbacyaml.Filename, strings.Join(list, "; "))
		case len(cases[path]) > 0:
			fileList.Errorf("%s does not match %s: %s. The autofix leaves the file as it is: %s",
				path, rbacyaml.Filename, strings.Join(list, "; "), strings.Join(cases[path], "; "))
		default:
			fileList.WithFix(regenerateFix(run.modulePath, *file, list)).Errorf("%s does not match %s: %s. Run `%s` to rewrite the file from the declaration",
				path, rbacyaml.Filename, strings.Join(list, "; "), FixCommand)
		}
	}
}

// unfixable lists what keeps the fix from rewriting a file the declaration produces: every document
// its text holds must be one the declaration puts there, a legacy role or a module capability the
// declaration no longer produces (the rewrite drops it: sync owns them by class), or an object the
// declaration now produces under another name. The text is the same in every render variant, so
// every variant reaches the same answer; the render adds what the text cannot show: an object the
// file would produce that renders from another template.
func (r *SyncRule) unfixable(run *syncRun, file generate.File) []string {
	var out []string

	// What the lint could not read, the fix must not write over.
	text := run.templates[file.Path]
	if text.err != nil {
		return []string{fmt.Sprintf("the file cannot be read (%v), so what it holds is unknown", text.err)}
	}

	if templateGated(text.content, generate.RenderFile(file)) {
		out = append(out, fmt.Sprintf("the template serves both role models behind the version gate (the %s gate of rbacv2-migrate-module.sh, or a deckhouseVersion test the declaration does not produce), and a rewrite would drop the legacy branch -- edit the new branch, or drop the gate and the legacy object once clusters below DKP 1.78 are no longer served",
			rbaccontract.GateMarker))
	}

	produced := identitiesOf(file.Objects)
	fromFile := run.fromFile[file.Path]
	renderedRoles := renderedRolesOf(r.module.GetStorage(), file.Path)

	for _, doc := range text.docs {
		if _, ok := produced[doc.id]; ok {
			continue
		}

		if doc.unreadable {
			out = append(out, "it holds a document the linter cannot read (an include, a range, a computed name or a template action the declaration does not write) -- declare what it renders in "+rbacyaml.Filename+" or move it to another template")
			continue
		}

		if where, ok := run.placed[doc.id]; ok && where != file.Path {
			out = append(out, doc.id+" is declared in "+where+" -- move it there")
			continue
		}

		// exclude-rules.sync silences an object's finding; it must not turn the object into a
		// silent removal either.
		if doc.managed && r.Enabled(doc.kind, doc.name) {
			continue
		}

		if object, ok := fromFile[doc.id]; ok && replacedByProduced(object, file.Objects, renderedRoles, run.rendered) {
			continue
		}

		out = append(out, doc.id+", which "+rbacyaml.Filename+" does not produce -- declare it in "+rbacyaml.Filename+" or move it to another template")
	}

	// The render is judged too: an object that renders from the file and that the text did not show
	// -- a document the reading above missed -- must not vanish with the rewrite either.
	for id, object := range fromFile {
		if _, ok := produced[id]; ok {
			continue
		}

		if where, ok := run.placed[id]; ok && where != file.Path {
			out = append(out, id+" is declared in "+where+" -- move it there")
			continue
		}

		kind, name := object.Unstructured.GetKind(), object.Unstructured.GetName()
		if m, ok := run.actual[id]; ok && m.class != generate.ClassDeclared && r.Enabled(kind, name) {
			continue
		}

		// The model before 1.78: rewriting the file would serve the new model only, a decision the
		// finding above leaves to a person.
		if kind == "ClusterRole" && rbaccontract.IsLegacyKind(object.Unstructured.GetLabels()[rbaccontract.LabelKind]) {
			out = append(out, "a rewrite would replace the legacy RBACv2 scheme the template renders")
			continue
		}

		if replacedByProduced(object, file.Objects, renderedRoles, run.rendered) {
			continue
		}

		out = append(out, id+", which "+rbacyaml.Filename+" does not produce -- declare it in "+rbacyaml.Filename+" or move it to another template")
	}

	for _, id := range r.renderedElsewhere(file) {
		out = append(out, id+" -- the declaration puts it in this file: move it here")
	}

	for _, id := range heldElsewhere(file, run.templates) {
		out = append(out, id+" -- the declaration puts it in this file: move it here")
	}

	sort.Strings(out)

	return slices.Compact(out)
}

// withoutUnrendered leaves out the objects a template holds by its text when nothing rendered from
// it: the render skipped the template (and warned about it) or every object in it is under a
// condition false for these values. Either way the render tells nothing about them, and they are
// judged where the template renders.
func (run *syncRun) withoutUnrendered(file generate.File) generate.File {
	if len(run.fromFile[file.Path]) > 0 {
		return file
	}

	inText := map[string]bool{}
	for _, doc := range run.templates[file.Path].docs {
		inText[doc.id] = true
	}

	kept := make([]generate.Object, 0, len(file.Objects))

	for _, o := range file.Objects {
		if !inText[o.Identity()] {
			kept = append(kept, o)
		}
	}

	file.Objects = kept

	return file
}

// enabledObjects returns the file with the objects exclude-rules.sync names left out: they are
// neither compared nor reported as absent.
func (r *SyncRule) enabledObjects(file generate.File) generate.File {
	kept := make([]generate.Object, 0, len(file.Objects))

	for _, o := range file.Objects {
		if r.Enabled(o.Kind, o.Name) {
			kept = append(kept, o)
		}
	}

	file.Objects = kept

	return file
}

// legacyFiles maps the templates that rendered an object of the scheme before 1.78 to its kind.
// Those objects belong to no class the declaration produces; they are the module's old model.
func (r *SyncRule) legacyFiles() map[string]string {
	out := map[string]string{}

	for _, object := range r.module.GetStorage() {
		kind := object.Unstructured.GetLabels()[rbaccontract.LabelKind]
		if object.Unstructured.GetKind() != "ClusterRole" || !rbaccontract.IsLegacyKind(kind) {
			continue
		}

		// A file with both kinds names the smaller one, whatever order the storage yields.
		if prev, seen := out[object.ShortPath()]; !seen || kind < prev {
			out[object.ShortPath()] = kind
		}
	}

	return out
}

// isRBACKind reports whether the kind is one the declaration writes.
func isRBACKind(kind string) bool {
	return rbacKinds[kind]
}

// replacedByProduced reports whether a rendered object has a produced counterpart of the same kind
// and content under another name.
func replacedByProduced(object storage.StoreObject, produced []generate.Object, renderedRoles map[string]tupleSet, rendered map[string]struct{}) bool {
	content := object.Unstructured.UnstructuredContent()

	switch object.Unstructured.GetKind() {
	case "ClusterRoleBinding", "RoleBinding":
		binding := new(rbacv1.RoleBinding) // the fields compared are shared by both kinds
		if runtime.DefaultUnstructuredConverter.FromUnstructured(content, binding) != nil {
			return false
		}

		got := subjectSet(binding.Subjects)

		for _, o := range produced {
			if o.Kind != object.Unstructured.GetKind() || o.Namespace != object.Unstructured.GetNamespace() || o.RoleRefKind != binding.RoleRef.Kind {
				continue
			}

			// A counterpart already in the render is not what this object became: this one is a
			// duplicate someone keeps for its own reasons, not an old name.
			if _, present := rendered[o.Identity()]; present {
				continue
			}

			if roleRefMatches(o, binding.RoleRef, produced, renderedRoles) && subjectSetOf(o.Subjects) == got {
				return true
			}
		}
	case "ClusterRole", "Role":
		role := new(rbacv1.ClusterRole)
		if runtime.DefaultUnstructuredConverter.FromUnstructured(content, role) != nil {
			return false
		}

		got := expandRenderedRules(role.Rules)

		for _, o := range produced {
			if o.Kind != object.Unstructured.GetKind() || o.Namespace != object.Unstructured.GetNamespace() {
				continue
			}

			if _, present := rendered[o.Identity()]; present {
				continue
			}

			if grantsExactly(o, got) {
				return true
			}
		}
	}

	return false
}

// renderedRolesOf collects the rules of every role rendered from the file, by kind and name, for
// the rename check of bindings.
func renderedRolesOf(objects map[storage.ResourceIndex]storage.StoreObject, path string) map[string]tupleSet {
	out := map[string]tupleSet{}

	for _, object := range objects {
		if object.ShortPath() != path {
			continue
		}

		kind := object.Unstructured.GetKind()
		if kind != "ClusterRole" && kind != "Role" {
			continue
		}

		role := new(rbacv1.ClusterRole)
		if runtime.DefaultUnstructuredConverter.FromUnstructured(object.Unstructured.UnstructuredContent(), role) == nil {
			out[kind+"/"+object.Unstructured.GetName()] = expandRenderedRules(role.Rules)
		}
	}

	return out
}

// roleRefMatches accepts the produced roleRef itself, or the role the rendered binding pointed at
// when that role is rendered in the same file and the produced role carries exactly its rules: a
// renamed pair. A binding to anything else -- cluster-admin, a role of another file -- is not a
// rename, whatever its subjects, and stays a foreign object.
func roleRefMatches(producedBinding generate.Object, renderedRef rbacv1.RoleRef, produced []generate.Object, renderedRoles map[string]tupleSet) bool {
	if producedBinding.RoleRefName == renderedRef.Name {
		return true
	}

	rendered, ok := renderedRoles[renderedRef.Kind+"/"+renderedRef.Name]
	if !ok {
		return false
	}

	for _, o := range produced {
		if o.Kind != renderedRef.Kind || o.Name != producedBinding.RoleRefName {
			continue
		}

		return grantsExactly(o, rendered)
	}

	return false
}

// grantsExactly reports whether a produced object grants what the rendered rules grant: its rules
// under a condition count too, since the render being compared may hold them.
func grantsExactly(o generate.Object, got tupleSet) bool {
	want, conditional := expandModelRules(o.Rules)
	for t := range conditional {
		want.add(t)
	}

	return len(want.minus(got)) == 0 && len(got.minus(want)) == 0
}

func subjectSet(list []rbacv1.Subject) string {
	parts := make([]string, 0, len(list))
	for _, s := range list {
		parts = append(parts, s.Kind+"/"+s.Namespace+"/"+s.Name)
	}

	sort.Strings(parts)

	return strings.Join(parts, ",")
}

func subjectSetOf(list []generate.Subject) string {
	subjects := make([]rbacv1.Subject, 0, len(list))
	for _, s := range list {
		subjects = append(subjects, rbacv1.Subject{Kind: s.Kind, Namespace: s.Namespace, Name: s.Name})
	}

	return subjectSet(subjects)
}

// managedObject is a rendered object the sync rule owns, with the class it was recognized by.
type managedObject struct {
	object storage.StoreObject
	class  generate.Class
}

// managedObjects selects the rendered objects of the three classes, keyed by identity.
func (r *SyncRule) managedObjects(model *generate.Model) map[string]managedObject {
	declared := map[string]struct{}{}

	for _, file := range model.Files {
		for _, o := range file.Objects {
			declared[o.Identity()] = struct{}{}
		}
	}

	out := map[string]managedObject{}

	for index, object := range r.module.GetStorage() {
		identity := index.AsString()
		labels := object.Unstructured.GetLabels()
		annotations := object.Unstructured.GetAnnotations()

		switch {
		case object.Unstructured.GetKind() == "ClusterRole" && annotations[rbaccontract.AccessLevelAnnotation] != "":
			out[identity] = managedObject{object: object, class: generate.ClassLegacy}
		case object.Unstructured.GetKind() == "ClusterRole" && labels[rbaccontract.LabelKind] == rbaccontract.KindCapability && labels[rbaccontract.LabelModule] == r.module.GetName() &&
			isModuleCapabilityName(object.Unstructured.GetName(), r.module.GetName()):
			// Only the capabilities the declaration can produce: the module's own, in the namespace
			// and system lineages. A module may also ship capabilities of the project lineage or
			// platform-wide ones named after a lineage rather than the module (user-authz,
			// multitenancy-manager); the format has no place for them, so they stay hand-written
			// and are neither generated nor "extra" (D2).
			out[identity] = managedObject{object: object, class: generate.ClassCapability}
		default:
			if _, ok := declared[identity]; ok {
				out[identity] = managedObject{object: object, class: generate.ClassDeclared}
			}
		}
	}

	return out
}

// hasAbsentObject reports whether any object the file declares is missing from the render.
func hasAbsentObject(file generate.File, actual map[string]managedObject) bool {
	for _, o := range file.Objects {
		if _, ok := actual[o.Identity()]; !ok {
			return true
		}
	}

	return false
}

// compareFile lists the divergences between the objects a declared file holds and the render.
func compareFile(file generate.File, actual map[string]managedObject, module string) []string {
	var out []string

	// A `when` excuses an absent object only while its condition is false: when another object of
	// the file under the same `when` rendered, the condition holds in this render, and the absence
	// is drift (review of #479, finding 47).
	holds := map[string]bool{}

	for _, o := range file.Objects {
		if _, ok := actual[o.Identity()]; ok && o.When != "" {
			holds[o.When] = true
		}
	}

	for _, expected := range file.Objects {
		act, ok := actual[expected.Identity()]
		if !ok {
			if expected.When == "" || holds[expected.When] {
				out = append(out, fmt.Sprintf("%s is declared but absent from the render", expected.Identity()))
			}

			continue
		}

		out = append(out, compareObject(expected, act.object, module)...)
	}

	return out
}

func compareObject(expected generate.Object, actual storage.StoreObject, module string) []string {
	var out []string

	if expected.Class == generate.ClassDeclared {
		out = append(out, compareAnnotations(expected, actual)...)
		out = append(out, compareLabels(expected, actual)...)
	}

	id := expected.Identity()
	content := actual.Unstructured.UnstructuredContent()

	switch expected.Kind {
	case "ClusterRole", "Role":
		var rules []rbacv1.PolicyRule

		var aggregation *rbacv1.AggregationRule

		if expected.Kind == "ClusterRole" {
			role := new(rbacv1.ClusterRole)
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(content, role); err != nil {
				return []string{fmt.Sprintf("%s cannot be read as a ClusterRole: %v", id, err)}
			}

			rules, aggregation = role.Rules, role.AggregationRule
		} else {
			role := new(rbacv1.Role)
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(content, role); err != nil {
				return []string{fmt.Sprintf("%s cannot be read as a Role: %v", id, err)}
			}

			rules = role.Rules
		}

		actualTuples := expandRenderedRules(rules)
		always, conditional := expandModelRules(expected.Rules)

		for _, t := range always.uncoveredBy(actualTuples) {
			out = append(out, fmt.Sprintf("%s: %s is declared but absent from the render", id, t))
		}

		for t := range conditional {
			always.add(t)
		}

		for _, t := range actualTuples.minus(always) {
			out = append(out, fmt.Sprintf("%s: %s is in the render but not declared", id, t))
		}

		if expected.Class == generate.ClassCapability {
			out = append(out, compareCapabilityLabels(expected, actual, module, aggregation)...)
		} else if aggregation != nil {
			// The generator writes no aggregationRule on any other role; one in the render collects
			// rights the declaration does not name, and a regeneration would drop it.
			out = append(out, fmt.Sprintf("%s: an aggregationRule is in the render but the declaration produces none", id))
		}

		// The access level decides which user-authz level a legacy role aggregates into; the same
		// rules under another level are other rights.
		if expected.Class == generate.ClassLegacy {
			want := expected.Annotations[rbaccontract.AccessLevelAnnotation]
			if got := actual.Unstructured.GetAnnotations()[rbaccontract.AccessLevelAnnotation]; got != want {
				out = append(out, fmt.Sprintf("%s: the %s annotation is %q in the render, the declaration produces %q", id, rbaccontract.AccessLevelAnnotation, got, want))
			}
		}
	case "ServiceAccount":
		sa := new(corev1.ServiceAccount)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(content, sa); err != nil {
			return []string{fmt.Sprintf("%s cannot be read as a ServiceAccount: %v", id, err)}
		}

		// Kubernetes mounts the token unless told otherwise; the generator writes what the
		// declaration says, false by default. A regeneration must not take a token away unnoticed.
		rendered := sa.AutomountServiceAccountToken == nil || *sa.AutomountServiceAccountToken
		declared := expected.AutomountToken != nil && *expected.AutomountToken

		if rendered != declared {
			out = append(out, fmt.Sprintf("%s: automountServiceAccountToken is %t in the render, the declaration produces %t", id, rendered, declared))
		}
	case "ClusterRoleBinding", "RoleBinding":
		var roleRef rbacv1.RoleRef

		var subjects []rbacv1.Subject

		if expected.Kind == "ClusterRoleBinding" {
			binding := new(rbacv1.ClusterRoleBinding)
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(content, binding); err != nil {
				return []string{fmt.Sprintf("%s cannot be read as a ClusterRoleBinding: %v", id, err)}
			}

			roleRef, subjects = binding.RoleRef, binding.Subjects
		} else {
			binding := new(rbacv1.RoleBinding)
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(content, binding); err != nil {
				return []string{fmt.Sprintf("%s cannot be read as a RoleBinding: %v", id, err)}
			}

			roleRef, subjects = binding.RoleRef, binding.Subjects
		}

		if roleRef.Kind != expected.RoleRefKind || roleRef.Name != expected.RoleRefName {
			out = append(out, fmt.Sprintf("%s binds %s %s, the declaration binds %s %s", id, roleRef.Kind, roleRef.Name, expected.RoleRefKind, expected.RoleRefName))
		}

		want := map[string]struct{}{}
		for _, s := range expected.Subjects {
			want[s.Kind+"/"+s.Namespace+"/"+s.Name] = struct{}{}
		}

		got := map[string]struct{}{}

		for _, s := range subjects {
			// Kubernetes puts a ServiceAccount subject without a namespace into the binding's.
			if s.Kind == "ServiceAccount" && s.Namespace == "" && expected.Kind == "RoleBinding" {
				s.Namespace = actual.Unstructured.GetNamespace()
			}

			got[s.Kind+"/"+s.Namespace+"/"+s.Name] = struct{}{}
		}

		for _, s := range sortedSetDiff(want, got) {
			out = append(out, fmt.Sprintf("%s: subject %s is declared but absent from the render", id, s))
		}

		for _, s := range sortedSetDiff(got, want) {
			out = append(out, fmt.Sprintf("%s: subject %s is in the render but not declared", id, s))
		}
	}

	return out
}

// compareCapabilityLabels checks the aggregation edges in both directions (R25a: a lost lineage is
// a lost right even when the rules agree) and the module-level contract the generator writes: the
// marker, the module label and the namespace label (D8 -- these live here, not in contract).
func compareCapabilityLabels(expected generate.Object, actual storage.StoreObject, module string, aggregation *rbacv1.AggregationRule) []string {
	var out []string

	id := expected.Identity()
	labels := actual.Unstructured.GetLabels()

	want := lineagesOfLabels(expected.Labels)
	got := lineagesOfLabels(labels)

	for _, l := range want.minus(got) {
		out = append(out, fmt.Sprintf("%s: aggregation into %s is declared but absent from the render", id, l))
	}

	for _, l := range got.minus(want) {
		out = append(out, fmt.Sprintf("%s: aggregation into %s is in the render but not declared", id, l))
	}

	for _, key := range []string{rbaccontract.LabelCapability, rbaccontract.LabelScope, rbaccontract.LabelNamespace} {
		if labels[key] != expected.Labels[key] {
			out = append(out, fmt.Sprintf("%s: label %s is %q in the render, the declaration produces %q", id, key, labels[key], expected.Labels[key]))
		}
	}

	if labels[rbaccontract.LabelModule] != module {
		out = append(out, fmt.Sprintf("%s: label %s is %q in the render, expected %q", id, rbaccontract.LabelModule, labels[rbaccontract.LabelModule], module))
	}

	if aggregation != nil {
		out = append(out, fmt.Sprintf("%s: a capability carries rules and is aggregated by roles; it must not define aggregationRule", id))
	}

	return out
}

func sortedSetDiff(a, b map[string]struct{}) []string {
	var out []string

	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}

	sort.Strings(out)

	return out
}

// crdScopes indexes the module's CRDs by "group/plural" for the declaration validator.
func crdScopes(crds []crdInfo) rbacyaml.CRDScopes {
	scopes := make(rbacyaml.CRDScopes, len(crds))
	for _, c := range crds {
		scopes[c.Key()] = c.Scope
	}

	return scopes
}

// regenerateFix returns the autofix for a file the declaration produces: write it from the
// declaration. Everything the fix needs is captured now, while the render exists -- the object store
// is released before --fix runs (R32). The declaration is the source of truth: a right it no longer
// names leaves the template (decided 2026-09-22, replacing D3); the finding that led here listed it,
// and the fix logs it, so a --fix run without a preceding dmt lint does not remove rights in silence.
func regenerateFix(modulePath string, file generate.File, changes []string) errors.AutofixFunc {
	content := generate.RenderFile(file)
	fullPath := filepath.Join(modulePath, file.Path)

	// Under --matrix the module is linted once per render variant and every variant collects its
	// own finding with its own closure. Each records what its render changes, while the store
	// exists; the closure that runs first writes and logs the union (R36).
	recordChanges(fullPath, changes)

	return func() error {
		return fixOnce(fullPath, func() error {
			// Another variant reported the file without a fix: its finding names the case, and the
			// lint after --fix reports it.
			if fixWithheld(fullPath) {
				return nil
			}

			existing, err := os.ReadFile(fullPath)
			if err != nil && !stderrors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("read %s: %w", file.Path, err)
			}

			if err == nil && string(existing) == content {
				return nil
			}

			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				return fmt.Errorf("create %s: %w", filepath.Dir(file.Path), err)
			}

			perm := os.FileMode(0o644)
			if info, err := os.Stat(fullPath); err == nil {
				perm = info.Mode().Perm()
			}

			if err := writeFileAtomic(fullPath, []byte(content), perm); err != nil {
				return fmt.Errorf("write %s: %w", file.Path, err)
			}

			// The declaration is the source: what it no longer names left the file. Say so where a
			// --fix run without a preceding lint would otherwise remove it in silence.
			if changes := recordedChanges(fullPath); len(changes) > 0 {
				// The divergences go both ways: what the render has and the declaration does not
				// leaves, what the declaration has and the render lacks arrives.
				added, removed := splitChanges(changes)
				log.Warn("rbac autofix rewrote a template from rbac.yaml: what the render had and the declaration does not name is removed, what the declaration names is added",
					slog.String("file", file.Path), slog.Any("removed", removed), slog.Any("added", added))
			} else {
				log.Info("rbac autofix rewrote a template from rbac.yaml", slog.String("file", file.Path))
			}

			return nil
		})
	}
}

// isModuleCapabilityName reports whether the name is one the generator builds for this module:
// d8:namespace-capability:<module>:<action> or d8:system-capability:<module>:<action>.
func isModuleCapabilityName(name, module string) bool {
	return strings.HasPrefix(name, rbaccontract.NamespaceCapabilityPrefix+module+":") || strings.HasPrefix(name, rbaccontract.SystemCapabilityPrefix+module+":")
}

// bootstrap is the entry of an existing module into the declaration: without rbac.yaml, the rule
// reports the file missing and --fix writes it from the RBAC objects the module renders today --
// the declaration a person would have transcribed from the templates, with a TODO wherever a
// decision is still theirs (decided 2026-09-22; R22 said "contract only", the ADR said "coverage
// creates the file"). From then on rbac.yaml is the source and the templates follow it.
func (r *SyncRule) bootstrap(declList *errors.LintRuleErrorsList) {
	modulePath := r.module.GetPath()
	store := r.module.GetStorage()

	if len(store) == 0 {
		return
	}

	meta, err := readModuleMetadata(modulePath)
	if err != nil {
		r.errorList.WithFilePath("module.yaml").Errorf("%v; the declaration is not written until it parses", err)
		return
	}

	crds, _ := moduleCRDs(modulePath)
	in := bootstrap.Input{Module: r.module.GetName(), Namespace: r.module.GetNamespace(), Subsystems: meta.Subsystems, CRDs: crdScopes(crds)}

	docs := map[string][]bootstrap.Doc{}

	for _, object := range store {
		if o, ok := bootstrapObject(object); ok {
			locateInTemplate(modulePath, &o, docs)
			in.Objects = append(in.Objects, o)
		}
	}

	if len(in.Objects) == 0 {
		return
	}

	markLibraryFiles(in.Objects)

	// The notes on what no render showed are for the written file: the fix reads the templates.
	result := bootstrap.Build(in)
	described := len(in.Objects) - len(result.Unmanaged)
	path := rbacyaml.Path(modulePath)

	// Under --matrix every variant renders its own set of objects; the fix builds from their union,
	// so an object rendered only under some values still reaches the first declaration.
	recordBootstrapObjects(path, in.Objects)

	declList.WithFix(func() error {
		return fixOnce(path, func() error {
			if _, err := os.Stat(path); err == nil {
				return nil
			}

			in.Objects = bootstrapObjectsOf(path)
			markLibraryFiles(in.Objects)
			in.Partial = bootstrapPartialOf(path)
			in.Variants = bootstrapVariantsOf(path)
			in.Unrendered = unrenderedObjects(modulePath, in.Objects)
			result := bootstrap.Build(in)

			content, err := bootstrap.Marshal(result)
			if err != nil {
				return fmt.Errorf("render %s: %w", rbacyaml.Filename, err)
			}

			return writeBootstrapped(path, content)
		})
	}).Errorf("%s is missing: `%s` writes it from the RBAC objects the module renders today (%d of %d objects described, the rest listed in the file as hand-written); every TODO and note in it is a decision for a person before --fix rewrites the templates from it",
		rbacyaml.Filename, FixCommand, described, len(in.Objects))
}

// bootstrapObject converts a rendered object of RBAC interest for the importer.
func bootstrapObject(object storage.StoreObject) (bootstrap.Object, bool) {
	u := object.Unstructured
	o := bootstrap.Object{Kind: u.GetKind(), Name: u.GetName(), Namespace: u.GetNamespace(), Path: object.ShortPath(), Labels: u.GetLabels(), Annotations: u.GetAnnotations()}
	content := u.UnstructuredContent()

	switch o.Kind {
	case "ClusterRole":
		role := new(rbacv1.ClusterRole)
		if runtime.DefaultUnstructuredConverter.FromUnstructured(content, role) != nil {
			return o, false
		}

		o.Rules, o.Aggregated = role.Rules, role.AggregationRule != nil
	case "Role":
		role := new(rbacv1.Role)
		if runtime.DefaultUnstructuredConverter.FromUnstructured(content, role) != nil {
			return o, false
		}

		o.Rules = role.Rules
	case "ClusterRoleBinding":
		b := new(rbacv1.ClusterRoleBinding)
		if runtime.DefaultUnstructuredConverter.FromUnstructured(content, b) != nil {
			return o, false
		}

		o.RoleRef, o.Subjects = b.RoleRef, b.Subjects
	case "RoleBinding":
		b := new(rbacv1.RoleBinding)
		if runtime.DefaultUnstructuredConverter.FromUnstructured(content, b) != nil {
			return o, false
		}

		o.RoleRef, o.Subjects = b.RoleRef, b.Subjects
	case "ServiceAccount":
		sa := new(corev1.ServiceAccount)
		if runtime.DefaultUnstructuredConverter.FromUnstructured(content, sa) != nil {
			return o, false
		}

		o.Automount = sa.AutomountServiceAccountToken
	default:
		return o, false
	}

	return o, true
}

// renderedElsewhere lists the objects the file produces that the render shows in another file.
func (r *SyncRule) renderedElsewhere(file generate.File) []string {
	produced := identitiesOf(file.Objects)

	var out []string

	for index, object := range r.module.GetStorage() {
		id := index.AsString()
		if _, ok := produced[id]; ok && object.ShortPath() != file.Path {
			out = append(out, id+" (renders from "+object.ShortPath()+")")
		}
	}

	sort.Strings(out)

	return out
}

// identitiesOf returns the identities of the objects.
func identitiesOf(objects []generate.Object) map[string]struct{} {
	out := make(map[string]struct{}, len(objects))
	for _, o := range objects {
		out[o.Identity()] = struct{}{}
	}

	return out
}

// textDocument is what the lint needs to know about one object of a template's text.
type textDocument struct {
	id         string
	kind, name string
	managed    bool // a legacy role or a module capability
	unreadable bool // an include, a range, a computed name: what it renders is not known
}

var (
	kindLineRe      = regexp.MustCompile(`^kind:\s*(\S+)\s*(#.*)?$`)
	nameLineRe      = regexp.MustCompile(`^  name:\s*"?([^"\s#]+)"?\s*(#.*)?$`)
	namespaceLineRe = regexp.MustCompile(`^  namespace:\s*"?([^"\s#]+)"?\s*(#.*)?$`)
	// wrapperLineRe matches the lines the declaration writes between objects: its conditions and
	// their ends. Anything else outside an object is content the lint does not understand.
	wrapperLineRe = regexp.MustCompile(`^\s*(\{\{-?\s*(if|else|end)\b[^}]*-?\}\}\s*)*$`)
	// abortingActionRe matches what a condition line may call that the declaration never writes:
	// an abort of the render or content of its own (a `when` is a Helm expression over the values).
	abortingActionRe = regexp.MustCompile(`\b(fail|required|include|tpl)\b`)
	// labelsLineRe is the only other template action the generator writes: the module labels, with
	// no labels of its own or a dict of quoted literals (generate.labelsInclude). Anything else on
	// that line -- another include, labels from the values -- is not the generator's (review of
	// #479, finding 31).
	labelsLineRe = regexp.MustCompile(`^  \{\{- include "helm_lib_module_labels" \(list \.(?: \(dict(?: "(?:[^"\\]|\\.)*" "(?:[^"\\]|\\.)*")+\))?\) \| nindent 2 \}\}$`)
)

// textDocuments parses the objects of a template from its text: the declaration writes kind,
// metadata.name and metadata.namespace on lines of their own. A document it cannot read -- an
// include, a templated name, anything that is neither an object nor the declaration's own
// wrapper lines -- is unreadable, and the lint keeps the fix off rather than guess.
func textDocuments(content string) []textDocument {
	content = strings.ReplaceAll(content, "\r\n", "\n")

	var out []textDocument

	for i, doc := range bootstrap.DocSeparatorRe.Split(content, -1) {
		var kind, name, namespace string

		inMetadata := false
		other := false
		// An action the generator does not write -- an include, a range, a value from the values --
		// makes the document someone else's wherever it stands, after the kind line too: what it
		// renders under other values is not in this render (review of #479, finding 31).
		foreign := false

		for _, line := range strings.Split(doc, "\n") {
			if strings.Contains(line, "{{") && (!wrapperLineRe.MatchString(line) || abortingActionRe.MatchString(line)) && !labelsLineRe.MatchString(line) {
				foreign = true
			}

			switch {
			case line == "metadata:":
				inMetadata = true
			case strings.HasPrefix(line, "  ") && inMetadata:
				if m := nameLineRe.FindStringSubmatch(line); m != nil && name == "" {
					name = m[1]
				}

				if m := namespaceLineRe.FindStringSubmatch(line); m != nil && namespace == "" {
					namespace = m[1]
				}
			default:
				inMetadata = false

				if m := kindLineRe.FindStringSubmatch(line); m != nil && kind == "" {
					kind = m[1]
					continue
				}

				trimmed := strings.TrimSpace(line)
				if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !wrapperLineRe.MatchString(line) && kind == "" {
					other = true
				}
			}
		}

		switch {
		case kind == "" && !other && !foreign:
			continue // the header, or the end of a conditional block
		case foreign || kind == "" || name == "" || strings.Contains(name, "{{"):
			out = append(out, textDocument{id: fmt.Sprintf("<unreadable document %d>", i), unreadable: true})
			continue
		}

		id := kind + "/" + name
		if namespace != "" {
			id = namespace + "/" + id
		}

		managed := strings.Contains(doc, rbaccontract.AccessLevelAnnotation+":") ||
			strings.Contains(doc, rbaccontract.LabelKind+": "+rbaccontract.KindCapability) ||
			// the generator writes the module labels through helm_lib_module_labels, as a dict
			strings.Contains(doc, strconv.Quote(rbaccontract.LabelKind)+" "+strconv.Quote(rbaccontract.KindCapability))

		out = append(out, textDocument{id: id, kind: kind, name: name, managed: managed})
	}

	return out
}

// templateTexts reads every template of the module, by path relative to the module.
func templateTexts(modulePath string) map[string]templateText {
	out := map[string]templateText{}

	for _, path := range templateFiles(modulePath) {
		rel, err := filepath.Rel(modulePath, path)
		if err != nil {
			continue // GetFiles lists paths under the module: a path outside it is not a template
		}

		content, err := os.ReadFile(path)
		if err != nil {
			out[filepath.ToSlash(rel)] = templateText{err: err}
			continue
		}

		out[filepath.ToSlash(rel)] = templateText{content: string(content), docs: textDocuments(string(content))}
	}

	return out
}

// templateFiles lists the templates Helm renders: every file under templates/ but a partial (a
// name starting with an underscore), which renders no objects.
func templateFiles(modulePath string) []string {
	return fsutils.GetFiles(filepath.Join(modulePath, "templates"), false, func(_, path string) bool {
		return !strings.HasPrefix(filepath.Base(path), "_")
	})
}

// heldElsewhere lists the objects the file produces that another template's text holds.
func heldElsewhere(file generate.File, templates map[string]templateText) []string {
	var out []string

	for _, path := range slices.Sorted(maps.Keys(templates)) {
		if path == file.Path {
			continue
		}

		held := map[string]bool{}
		for _, doc := range templates[path].docs {
			held[doc.id] = true
		}

		for _, o := range file.Objects {
			if held[o.Identity()] {
				out = append(out, o.Identity()+" (held by "+path+")")
			}
		}
	}

	sort.Strings(out)

	return out
}

// replacedCopies reports a rendered object the declaration does not own that grants exactly what a
// produced object grants while that produced object renders too: the object it replaced under
// another name, often in another file (the Prometheus access Roles bootstrap folds into
// access-to-<module>). Both render, so the old copy keeps the grant alive after the declaration
// drops it. No fix: the copy sits in a file the generator does not own.
func (r *SyncRule) replacedCopies(run *syncRun, divergences map[string][]string) map[string][]string {
	store := r.module.GetStorage()

	all := make([]generate.Object, 0, len(run.placed))
	for _, f := range run.model.Files {
		all = append(all, f.Objects...)
	}

	copies := map[string]string{}
	renderedGrantees := renderedRoleSubjects(store)
	declaredGrantees := modelRoleSubjects(all)

	for index, object := range store {
		id := index.AsString()
		if _, ok := run.placed[id]; ok {
			continue
		}

		if _, ok := run.actual[id]; ok {
			continue
		}

		kind := object.Unstructured.GetKind()
		if !isRBACKind(kind) || kind == "ServiceAccount" {
			continue
		}

		if twin := renderedTwin(object, all, run.rendered); twin != "" {
			// Equal rules alone do not make a copy: another account may need the same rights. A
			// replaced role is granted to the subjects the declaration grants its successor to.
			if (kind == "Role" || kind == "ClusterRole") && !shareGrantee(renderedGrantees[roleKey(kind, object.Unstructured.GetNamespace(), object.Unstructured.GetName())], declaredGrantees[twinRoleKey(all, twin)]) {
				continue
			}

			copies[object.Unstructured.GetKind()+"/"+object.Unstructured.GetName()] = twin
			divergences[object.ShortPath()] = append(divergences[object.ShortPath()],
				id+" grants what "+twin+" grants, and both render: the declaration replaced it -- delete it from the template")
		}
	}

	// A binding of such a copy is part of it.
	for index, object := range store {
		kind := object.Unstructured.GetKind()
		if kind != "RoleBinding" && kind != "ClusterRoleBinding" {
			continue
		}

		if _, ok := run.placed[index.AsString()]; ok {
			continue
		}

		binding := new(rbacv1.RoleBinding)
		if runtime.DefaultUnstructuredConverter.FromUnstructured(object.Unstructured.UnstructuredContent(), binding) != nil {
			continue
		}

		if twin, ok := copies[binding.RoleRef.Kind+"/"+binding.RoleRef.Name]; ok {
			divergences[object.ShortPath()] = append(divergences[object.ShortPath()],
				index.AsString()+" binds "+binding.RoleRef.Name+", the old copy of "+twin+" -- delete it with the copy")
		}
	}

	return divergences
}

// renderedTwin returns the identity of a produced object that renders and grants exactly what the
// object grants (same rules, or same roleRef and subjects), or "".
func renderedTwin(object storage.StoreObject, produced []generate.Object, rendered map[string]struct{}) string {
	content := object.Unstructured.UnstructuredContent()

	for _, o := range produced {
		if o.Kind != object.Unstructured.GetKind() || o.Namespace != object.Unstructured.GetNamespace() {
			continue
		}

		if _, ok := rendered[o.Identity()]; !ok {
			continue
		}

		switch o.Kind {
		case "ClusterRole", "Role":
			role := new(rbacv1.ClusterRole)
			if runtime.DefaultUnstructuredConverter.FromUnstructured(content, role) != nil || role.AggregationRule != nil || len(role.Rules) == 0 {
				continue
			}

			if grantsExactly(o, expandRenderedRules(role.Rules)) {
				return o.Identity()
			}
		case "ClusterRoleBinding", "RoleBinding":
			binding := new(rbacv1.RoleBinding)
			if runtime.DefaultUnstructuredConverter.FromUnstructured(content, binding) != nil {
				continue
			}

			if binding.RoleRef.Kind == o.RoleRefKind && binding.RoleRef.Name == o.RoleRefName && subjectSet(binding.Subjects) == subjectSetOf(o.Subjects) {
				return o.Identity()
			}
		}
	}

	return ""
}

// compareAnnotations compares the annotations of an object the declaration writes whole: a
// resource policy or a deploy hook dropped by a regeneration changes what Helm or werf do with it.
func compareAnnotations(expected generate.Object, actual storage.StoreObject) []string {
	id := expected.Identity()
	rendered := actual.Unstructured.GetAnnotations()

	var out []string

	for _, k := range slices.Sorted(maps.Keys(rendered)) {
		// Helm's release annotations and the generator's own are nobody's to declare.
		if strings.HasPrefix(k, "meta.helm.sh/") || strings.HasPrefix(k, "rbac.deckhouse.io/") {
			continue
		}

		if want, ok := expected.Annotations[k]; !ok {
			out = append(out, fmt.Sprintf("%s: annotation %s is in the render but not declared", id, k))
		} else if want != rendered[k] {
			out = append(out, fmt.Sprintf("%s: annotation %s is %q in the render, the declaration produces %q", id, k, rendered[k], want))
		}
	}

	for _, k := range slices.Sorted(maps.Keys(expected.Annotations)) {
		if _, ok := rendered[k]; !ok {
			out = append(out, fmt.Sprintf("%s: annotation %s is declared but absent from the render", id, k))
		}
	}

	return out
}

// locateInTemplate reads the template blocks around a rendered object from its template's text:
// the render only holds what rendered for the linter's values, the text holds the conditions.
func locateInTemplate(modulePath string, o *bootstrap.Object, cache map[string][]bootstrap.Doc) {
	// A subchart's template is written against the subchart's values, and the generator writes
	// the module's own templates: the declaration has no place for its objects.
	if strings.HasPrefix(o.Path, "charts/") {
		o.Located = true
		o.Unmanageable = "rendered by the subchart " + strings.SplitN(o.Path, "/", 3)[1] + ", whose templates and values are its own"

		return
	}

	docs, ok := cache[o.Path]
	if !ok {
		if content, err := os.ReadFile(filepath.Join(modulePath, o.Path)); err == nil {
			docs = bootstrap.TemplateDocs(string(content))
		}

		cache[o.Path] = docs
	}

	d, found := bootstrap.Locate(docs, *o)
	if !found {
		return
	}

	o.Located = true
	o.When, o.Unmanageable, o.Partial = d.When, d.Unmanageable, d.Partial

	o.Library = d.Library

	if d.Library && o.Unmanageable == "" {
		o.Unmanageable = "rendered by an include of a named template (helm_lib or another chart), which owns it"
	}
}

// rbacKinds are the kinds bootstrap describes.
var rbacKinds = map[string]bool{"ClusterRole": true, "ClusterRoleBinding": true, "Role": true, "RoleBinding": true, "ServiceAccount": true}

// unrenderedObjects lists the RBAC objects with a literal name that the module's templates hold
// and no render showed: an object under a condition false for the linter's values would
// otherwise be left out of the first declaration without a word, and the regeneration would drop
// it. Only the text can tell; a computed name is not followed.
func unrenderedObjects(modulePath string, rendered []bootstrap.Object) []string {
	var out []string

	for _, path := range templateFiles(modulePath) {
		content, err := os.ReadFile(path)
		if err != nil {
			continue // an unreadable template is not the importer's to report
		}

		rel, err := filepath.Rel(modulePath, path)
		if err != nil {
			continue
		}

		for _, doc := range bootstrap.TemplateDocs(string(content)) {
			if !rbacKinds[doc.Kind] || doc.Unmanageable != "" || renderedAs(doc, rel, rendered) {
				continue
			}

			// A computed name is listed as the template writes it: the person adds the objects it
			// produces (review of #479, finding 34).
			var entry string

			switch {
			case doc.Name != "":
				entry = fmt.Sprintf("%s/%s (%s", doc.Kind, doc.Name, rel)
			case doc.NameTemplate != "":
				entry = fmt.Sprintf("a %s with the computed name %s (%s", doc.Kind, doc.NameTemplate, rel)
			default:
				continue
			}

			if doc.When != "" {
				entry += ", under `" + doc.When + "`"
			}

			out = append(out, entry+")")
		}
	}

	sort.Strings(out)

	return out
}

// moduleLabels are the labels helm_lib_module_labels writes on every object; the declaration
// names the others.
var moduleLabels = map[string]bool{rbaccontract.LabelHeritage: true, rbaccontract.LabelModule: true}

// compareLabels compares the labels of an object the declaration writes whole: an aggregation
// label or a part-of label the declaration does not carry would be dropped by the next
// regeneration, and an aggregation label is a right.
func compareLabels(expected generate.Object, actual storage.StoreObject) []string {
	id := expected.Identity()
	rendered := actual.Unstructured.GetLabels()

	var out []string

	for _, k := range slices.Sorted(maps.Keys(rendered)) {
		if moduleLabels[k] {
			continue
		}

		if want, ok := expected.Labels[k]; !ok {
			out = append(out, fmt.Sprintf("%s: label %s is in the render but not declared", id, k))
		} else if want != rendered[k] {
			out = append(out, fmt.Sprintf("%s: label %s is %q in the render, the declaration produces %q", id, k, rendered[k], want))
		}
	}

	for _, k := range slices.Sorted(maps.Keys(expected.Labels)) {
		if _, ok := rendered[k]; !ok && !moduleLabels[k] {
			out = append(out, fmt.Sprintf("%s: label %s is declared but absent from the render", id, k))
		}
	}

	return out
}

// roleKey names a role as bindings refer to it: a Role with its namespace, a ClusterRole without.
func roleKey(kind, namespace, name string) string {
	if kind == "ClusterRole" {
		namespace = ""
	}

	return kind + "/" + namespace + "/" + name
}

// renderedRoleSubjects maps every role the render binds to the subject sets of its bindings.
func renderedRoleSubjects(objects map[storage.ResourceIndex]storage.StoreObject) map[string]map[string]bool {
	out := map[string]map[string]bool{}

	for _, object := range objects {
		kind := object.Unstructured.GetKind()
		if kind != "RoleBinding" && kind != "ClusterRoleBinding" {
			continue
		}

		binding := new(rbacv1.RoleBinding)
		if runtime.DefaultUnstructuredConverter.FromUnstructured(object.Unstructured.UnstructuredContent(), binding) != nil {
			continue
		}

		key := roleKey(binding.RoleRef.Kind, object.Unstructured.GetNamespace(), binding.RoleRef.Name)
		if out[key] == nil {
			out[key] = map[string]bool{}
		}

		out[key][subjectSet(binding.Subjects)] = true
	}

	return out
}

// modelRoleSubjects is renderedRoleSubjects for the objects the declaration produces.
func modelRoleSubjects(all []generate.Object) map[string]map[string]bool {
	out := map[string]map[string]bool{}

	for _, o := range all {
		if o.Kind != "RoleBinding" && o.Kind != "ClusterRoleBinding" {
			continue
		}

		key := roleKey(o.RoleRefKind, o.Namespace, o.RoleRefName)
		if out[key] == nil {
			out[key] = map[string]bool{}
		}

		out[key][subjectSetOf(o.Subjects)] = true
	}

	return out
}

// twinRoleKey is the roleKey of the produced object with the identity.
func twinRoleKey(all []generate.Object, identity string) string {
	for _, o := range all {
		if o.Identity() == identity {
			return roleKey(o.Kind, o.Namespace, o.Name)
		}
	}

	return ""
}

func shareGrantee(a, b map[string]bool) bool {
	for k := range a {
		if b[k] {
			return true
		}
	}

	return false
}

// splitChanges sorts the divergences of a regenerated file into what the regeneration adds (the
// declaration names it, the render lacks it) and everything else, which it removes or changes.
func splitChanges(divergences []string) ([]string, []string) {
	var added, removed []string

	for _, d := range divergences {
		if strings.Contains(d, "is declared but absent from the render") {
			added = append(added, d)
		} else {
			removed = append(removed, d)
		}
	}

	return added, removed
}

// writeBootstrapped writes the declaration bootstrap produced and says what is left for a person.
// A declaration that does not parse is a bug of dmt, yet it is written all the same: the module's
// developer fixes the line the error names and goes on, instead of waiting for a dmt release with
// nothing to look at. Bootstrap runs only while the file is missing, so the error says where to
// look.
func writeBootstrapped(path string, content []byte) error {
	if err := writeFileAtomic(path, content, 0o644); err != nil { //nolint:gosec // a source file of the module
		return err
	}

	// The TODOs and the notes in the written file are for the lint that follows --fix to report:
	// the fix did its work. A file that does not parse would be a bug of dmt; it is written all
	// the same, so the error can name the line.
	if _, err := rbacyaml.Parse(content); err != nil {
		return fmt.Errorf("%s is written, but it does not parse (%w); the declaration is complete apart from that line -- most likely a note in the header that lost its '#': fix or delete the line. This is a bug of dmt, report it with the module",
			rbacyaml.Filename, err)
	}

	return nil
}

// markLibraryFiles marks the objects of a template that also renders a library's objects: the
// render tells, not the text (review of #479, finding 50).
func markLibraryFiles(objects []bootstrap.Object) {
	library := map[string]bool{}

	for _, o := range objects {
		if o.Library {
			library[o.Path] = true
		}
	}

	for i := range objects {
		objects[i].LibraryFile = library[objects[i].Path]
	}
}

// renderedAs reports whether a rendered object comes from the document: rendered from the same
// template, in the namespace the document names if it names one, under its literal name or a name
// its computed name can produce. An object of another template is not the document's, whatever
// its name (review of #480).
func renderedAs(doc bootstrap.Doc, path string, rendered []bootstrap.Object) bool {
	for _, o := range rendered {
		if o.Kind != doc.Kind || o.Path != path {
			continue
		}

		if doc.Namespace != "" && o.Namespace != "" && doc.Namespace != o.Namespace {
			continue
		}

		if (doc.Name != "" && o.Name == doc.Name) || (doc.NamePattern != nil && doc.NamePattern.MatchString(o.Name)) {
			return true
		}
	}

	return false
}
