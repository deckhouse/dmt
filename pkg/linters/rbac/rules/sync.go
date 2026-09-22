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
	"os"
	"path/filepath"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

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
// directions, and regenerates the templates from the declaration on --fix. It owns three classes
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

func readModuleMetadata(modulePath string) moduleMetadata {
	var meta moduleMetadata

	data, err := os.ReadFile(filepath.Join(modulePath, "module.yaml"))
	if err != nil {
		return meta
	}

	_ = yaml.Unmarshal(data, &meta)

	return meta
}

func (r *SyncRule) Check(_ context.Context) {
	modulePath := r.module.GetPath()
	declList := r.errorList.WithFilePath(rbacyaml.Filename)

	decl, err := rbacyaml.Load(modulePath)
	if stderrors.Is(err, rbacyaml.ErrNotFound) {
		r.bootstrap(declList)
		return
	}

	if overlay := editionOverlay(modulePath); overlay != "" {
		declList.Errorf("%s lies in the edition overlay %s; the declaration describes the union of editions and belongs to modules/<module>/ only -- CI merges the overlays over modules/ before linting, so a copy here would shadow it or go unseen. Only a person can close this: move the file",
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

	crds, err := moduleCRDs(modulePath)
	if err != nil {
		r.errorList.WithFilePath("crds").Errorf("cannot read the module CRDs: %v", err)
		return
	}

	if errs := rbacyaml.Validate(decl, crdScopes(crds)); len(errs) > 0 {
		for _, e := range errs {
			declList.Errorf("%v; nothing is compared or generated until the declaration is valid", e)
		}

		return
	}

	model, err := generate.Build(generate.Input{
		Module:     r.module.GetName(),
		Namespace:  r.module.GetNamespace(),
		Subsystems: readModuleMetadata(modulePath).Subsystems,
		Decl:       decl,
	})
	if err != nil {
		declList.Errorf("cannot derive the RBAC objects from the declaration: %v", err)
		return
	}

	actual := r.managedObjects(model)
	legacy := r.legacyFiles()
	divergences := map[string][]string{}

	for _, file := range model.Files {
		kind, isLegacy := legacy[file.Path]

		// A template that serves both models behind the version gate rendered its legacy branch:
		// the values of this run say the cluster is below 1.78. The 1.78 objects it declares are
		// conditional on the gate and are compared in the run where the gate answers "new" -- the
		// default one -- so this is not a divergence (the same reading as a rule under when, D4).
		if isLegacy && templateHasGate(modulePath, file.Path) {
			continue
		}

		found := compareFile(file, actual, r.module.GetName())

		// The template renders the scheme before 1.78 where the declaration produces the new one:
		// the objects the declaration names cannot be there. Say so once instead of listing them.
		if isLegacy && len(found) > 0 {
			found = append(found, fmt.Sprintf("the template renders the legacy RBACv2 scheme (%s: %s, the manage/use model before DKP 1.78) where the declaration produces the 1.78 model; migrate the module with rbacv2-migrate-module.sh to serve both, or delete the file and run `%s` to serve the new one only",
				rbaccontract.LabelKind, kind, FixCommand))
		}

		divergences[file.Path] = append(divergences[file.Path], found...)
	}

	// A legacy role or a module capability the declaration does not produce is an object the
	// declaration must own: it is reported under the template it came from.
	modelIdentities := map[string]struct{}{}

	for _, file := range model.Files {
		for _, o := range file.Objects {
			modelIdentities[o.Identity()] = struct{}{}
		}
	}

	for identity, obj := range actual {
		if _, produced := modelIdentities[identity]; produced || obj.class == generate.ClassDeclared {
			continue
		}

		divergences[obj.object.ShortPath()] = append(divergences[obj.object.ShortPath()],
			fmt.Sprintf("%s is in the render but rbac.yaml does not produce it: declare its rights in rbac.yaml or remove it from the template", identity))
	}

	// A file that carries the generator header is the generator's: its text must be what the
	// declaration renders now. The render alone cannot tell -- a rule under `when` whose condition
	// is false today is absent from the render without being a divergence (D4), yet it still has
	// to reach the template -- so for these files the text is compared too. A file of another
	// contract version is the same case (R40). A file without the header is maintained by hand and
	// is judged by its render only.
	for _, file := range model.Files {
		content, err := os.ReadFile(filepath.Join(modulePath, file.Path))
		if err != nil {
			// A file that does not exist while an object it holds is absent from the render: the
			// render cannot tell a conditional object whose condition is false from one whose
			// template was never written, but the text can -- nothing produces it (D4 covers the
			// render, not the file).
			if stderrors.Is(err, os.ErrNotExist) && hasAbsentObject(file, actual) {
				divergences[file.Path] = append(divergences[file.Path],
					"the file does not exist, and objects the declaration puts in it are absent from the render (objects under `when` included: no template produces them)")
			}

			continue
		}

		generated, version := generate.ParseHeader(string(content))

		switch {
		case !generated:
		case version != rbaccontract.ContractVersion:
			divergences[file.Path] = append(divergences[file.Path],
				fmt.Sprintf("the file was generated under contract version %q; the current contract is %q", version, rbaccontract.ContractVersion))
		case string(content) != generate.RenderFile(file):
			divergences[file.Path] = append(divergences[file.Path],
				"the file carries the generator header but is not what the declaration renders now (a rule under `when`, a text edit or an older generator); remove the header to maintain it by hand")
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

		if file := model.File(path); file != nil {
			fileList = fileList.WithFix(regenerateFix(modulePath, *file, r.foreignObjects(*file, model)))
			fileList.Errorf("%s does not match %s: %s. Run `%s` to regenerate the file from the declaration",
				path, rbacyaml.Filename, strings.Join(list, "; "), FixCommand)

			continue
		}

		fileList.Errorf("%s does not match %s: %s. Only a person can close this: the declaration does not produce this file",
			path, rbacyaml.Filename, strings.Join(list, "; "))
	}
}

// legacyFiles maps the templates that rendered an object of the scheme before 1.78 to its kind.
// Those objects belong to no class the declaration produces; they are the module's old model.
func (r *SyncRule) legacyFiles() map[string]string {
	out := map[string]string{}

	for _, object := range r.module.GetStorage() {
		if kind := object.Unstructured.GetLabels()[rbaccontract.LabelKind]; object.Unstructured.GetKind() == "ClusterRole" && rbaccontract.IsLegacyKind(kind) {
			out[object.ShortPath()] = kind
		}
	}

	return out
}

// foreignObjects lists the RBAC objects the render placed in the file that the declaration does not
// produce -- a controller ClusterRole beside a declared ServiceAccount, a hand-written binding. The
// generator writes the whole file, so regenerating it would drop them; they are the reason a
// regeneration is refused until they are declared or moved.
func (r *SyncRule) foreignObjects(file generate.File, model *generate.Model) []string {
	produced := map[string]struct{}{}
	for _, o := range file.Objects {
		produced[o.Identity()] = struct{}{}
	}

	// Where the declaration puts every object it produces: an object rendered from another file
	// than that is misplaced rather than unknown, and the refusal says so.
	placed := map[string]string{}
	for _, f := range model.Files {
		for _, o := range f.Objects {
			placed[o.Identity()] = f.Path
		}
	}

	var out []string

	for index, object := range r.module.GetStorage() {
		if object.ShortPath() != file.Path {
			continue
		}

		switch object.Unstructured.GetKind() {
		case "ClusterRole", "Role", "ClusterRoleBinding", "RoleBinding", "ServiceAccount":
		default:
			continue
		}

		if _, ok := produced[index.AsString()]; ok {
			continue
		}

		// An object the generator produces under another name -- a binding with the same roleRef
		// and subjects, a role with the same rules -- is replaced, not lost; the declaration carries
		// its rights on. Only what has no counterpart is foreign.
		if replacedByProduced(object, file.Objects) {
			continue
		}

		if path, declared := placed[index.AsString()]; declared {
			out = append(out, index.AsString()+" (the declaration puts it in "+path+"; move it there or delete both files and run the fix)")
			continue
		}

		out = append(out, index.AsString())
	}

	sort.Strings(out)

	return out
}

// replacedByProduced reports whether a rendered object has a produced counterpart of the same kind
// and content under another name.
func replacedByProduced(object storage.StoreObject, produced []generate.Object) bool {
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

			if roleRefMatches(o.RoleRefName, binding.RoleRef.Name, produced) && subjectSetOf(o.Subjects) == got {
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

			always, conditional := expandModelRules(o.Rules)
			for t := range conditional {
				always.add(t)
			}

			if len(always.minus(got)) == 0 && len(got.minus(always)) == 0 {
				return true
			}
		}
	}

	return false
}

// roleRefMatches accepts the produced roleRef itself or the rendered role it replaces by content.
func roleRefMatches(producedRef, renderedRef string, produced []generate.Object) bool {
	if producedRef == renderedRef {
		return true
	}

	// The rendered binding pointed at a role the generator now produces under producedRef: accept
	// when producedRef is a produced role and renderedRef is not.
	for _, o := range produced {
		if (o.Kind == "ClusterRole" || o.Kind == "Role") && o.Name == renderedRef {
			return false
		}
	}

	for _, o := range produced {
		if (o.Kind == "ClusterRole" || o.Kind == "Role") && o.Name == producedRef {
			return true
		}
	}

	return false
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
	parts := make([]string, 0, len(list))
	for _, s := range list {
		parts = append(parts, s.Kind+"/"+s.Namespace+"/"+s.Name)
	}

	sort.Strings(parts)

	return strings.Join(parts, ",")
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
		if !r.Enabled(object.Unstructured.GetKind(), object.Unstructured.GetName()) {
			continue
		}

		identity := index.AsString()
		labels := object.Unstructured.GetLabels()
		annotations := object.Unstructured.GetAnnotations()

		switch {
		case object.Unstructured.GetKind() == "ClusterRole" && annotations[rbaccontract.AccessLevelAnnotation] != "":
			out[identity] = managedObject{object, generate.ClassLegacy}
		case object.Unstructured.GetKind() == "ClusterRole" && labels[rbaccontract.LabelKind] == rbaccontract.KindCapability && labels[rbaccontract.LabelModule] == r.module.GetName() &&
			isModuleCapabilityName(object.Unstructured.GetName(), r.module.GetName()):
			// Only the capabilities the declaration can produce: the module's own, in the namespace
			// and system lineages. A module may also ship capabilities of the project lineage or
			// platform-wide ones named after a lineage rather than the module (user-authz,
			// multitenancy-manager); the format has no place for them, so they stay hand-written
			// and are neither generated nor "extra" (D2).
			out[identity] = managedObject{object, generate.ClassCapability}
		default:
			if _, ok := declared[identity]; ok {
				out[identity] = managedObject{object, generate.ClassDeclared}
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

// compareFile lists the divergences between the objects a generated file declares and the render.
func compareFile(file generate.File, actual map[string]managedObject, module string) []string {
	var out []string

	for _, expected := range file.Objects {
		act, ok := actual[expected.Identity()]
		if !ok {
			if expected.When == "" {
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

		for _, t := range always.minus(actualTuples) {
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

// regenerateFix returns the autofix for a generated file: write it from the declaration. Everything
// the fix needs is captured now, while the render exists -- the object store is released before
// --fix runs (R32). The declaration is the source of truth: a right it no longer names leaves the
// template (decided 2026-09-22, replacing D3), and the finding that led here listed it. Three
// things are never written over:
//
//   - a file that also holds objects the declaration does not produce -- the generator writes the
//     whole file and they would vanish;
//   - a template that serves both role models behind the version gate (R30);
//   - a file without the generator header, maintained by hand: the generated text is written
//     beside it as _<file>.generated and the finding stays (R16, US-F2).
func regenerateFix(modulePath string, file generate.File, foreign []string) errors.AutofixFunc {
	content := generate.RenderFile(file)
	fullPath := filepath.Join(modulePath, file.Path)

	// Under --matrix the module is linted once per render variant and every variant collects its
	// own finding with its own closure. Each records what its render grants now, while the store
	// exists; the closure that runs first checks the union of them and writes, the others report
	// its outcome (R36). A right rendered only under some values is therefore not lost (D3).
	recordForeignObjects(fullPath, foreign)

	return func() error {
		return fixOnce(fullPath, func() error {
			existing, err := os.ReadFile(fullPath)
			exists := err == nil

			if err != nil && !stderrors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("read %s: %w", file.Path, err)
			}

			if exists {
				// Objects in the file that the declaration does not produce would vanish with the rewrite,
				// header or not -- and "delete the file and run --fix again" would lose them too, so this
				// comes before every other answer. (A foreign object under `when` that did not render this
				// time is caught by the run where it renders; every variant's list is joined.)
				if foreign := foreignObjectsOf(fullPath); len(foreign) > 0 {
					return fmt.Errorf("%s also holds objects the declaration does not produce: %s; regenerating the file would drop them, and so would deleting it -- declare them in %s or move them to another template, then run `%s` again",
						file.Path, strings.Join(foreign, ", "), rbacyaml.Filename, FixCommand)
				}

				if strings.Contains(string(existing), rbaccontract.GateMarker) || strings.Contains(string(existing), "deckhouseVersion") {
					return fmt.Errorf("%s renders one of two role models depending on the platform version (the %s gate of rbacv2-migrate-module.sh); regenerating it would drop the legacy branch -- edit the new branch by hand, or drop the gate and the legacy object once clusters below DKP 1.78 are no longer served, then run `%s`",
						file.Path, rbaccontract.GateMarker, FixCommand)
				}

				if generated, _ := generate.ParseHeader(string(existing)); !generated {
					aside := asidePath(fullPath)
					if err := os.WriteFile(aside, []byte(content), 0o600); err != nil {
						return fmt.Errorf("write %s: %w", asidePath(file.Path), err)
					}

					return fmt.Errorf("%s is maintained by hand (no generator header); the generated version is beside it as %s -- compare, then either delete the file and run `%s` again, or keep maintaining it by hand",
						file.Path, asidePath(file.Path), FixCommand)
				}
			}

			if exists && string(existing) == content {
				return nil
			}

			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				return fmt.Errorf("create %s: %w", filepath.Dir(file.Path), err)
			}

			perm := os.FileMode(0o644)
			if info, err := os.Stat(fullPath); err == nil {
				perm = info.Mode().Perm()
			}

			return os.WriteFile(fullPath, []byte(content), perm)
		})
	}
}

// asidePath is where the generated text of a hand-maintained file is written for comparison:
// _<name>.generated in the same directory. Helm renders every file under templates/ whatever its
// extension, so a plain copy would render a second set of objects; a name starting with an
// underscore is a partial to Helm and produces no objects.
func asidePath(path string) string {
	return filepath.Join(filepath.Dir(path), "_"+filepath.Base(path)+".generated")
}

// isModuleCapabilityName reports whether the name is one the generator builds for this module:
// d8:namespace-capability:<module>:<action> or d8:system-capability:<module>:<action>.
func isModuleCapabilityName(name, module string) bool {
	return strings.HasPrefix(name, "d8:namespace-capability:"+module+":") || strings.HasPrefix(name, "d8:system-capability:"+module+":")
}

// bootstrap is the entry of an existing module into the declaration: without rbac.yaml, the rule
// reports the file missing and --fix writes it from the RBAC objects the module renders today --
// the declaration a person would have transcribed from the templates, with a TODO wherever a
// decision is still theirs (decided 2026-09-22; R22 said "contract only", the ADR said "coverage
// creates the file"). From then on rbac.yaml is the source and the templates follow it.
func (r *SyncRule) bootstrap(declList *errors.LintRuleErrorsList) {
	modulePath := r.module.GetPath()
	storage := r.module.GetStorage()

	if len(storage) == 0 {
		return
	}

	in := bootstrap.Input{Module: r.module.GetName(), Namespace: r.module.GetNamespace(), Subsystems: readModuleMetadata(modulePath).Subsystems, CRDs: map[string]string{}}

	if crds, err := moduleCRDs(modulePath); err == nil {
		for _, crd := range crds {
			in.CRDs[crd.Key()] = crd.Scope
		}
	}

	for _, object := range storage {
		if o, ok := bootstrapObject(object); ok {
			in.Objects = append(in.Objects, o)
		}
	}

	if len(in.Objects) == 0 {
		return
	}

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
			result := bootstrap.Build(in)

			content, err := bootstrap.Marshal(result)
			if err != nil {
				return fmt.Errorf("render %s: %w", rbacyaml.Filename, err)
			}

			return os.WriteFile(path, content, 0o644) //nolint:gosec // a source file of the module
		})
	}).Errorf("%s is missing: `%s` writes it from the RBAC objects the module renders today (%d of %d objects described, the rest listed in the file as hand-written); every TODO and note in it is a decision for a person before the templates are regenerated from it",
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

		o.Rules = role.Rules
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
