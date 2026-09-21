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

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
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
		return
	}

	if overlay := editionOverlay(modulePath); overlay != "" {
		declList.Errorf("%s lies in the edition overlay %s; the declaration describes the union of editions and belongs to modules/<module>/ only -- CI merges the overlays over modules/ before linting, so a copy here would shadow it or go unseen. Only a person can close this: move the file",
			rbacyaml.Filename, overlay)

		return
	}

	if err != nil {
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
		found := compareFile(file, actual, r.module.GetName())

		// The template renders the scheme before 1.78 where the declaration produces the new one:
		// the objects the declaration names cannot be there. Say so once instead of listing them.
		if kind, isLegacy := legacy[file.Path]; isLegacy && len(found) > 0 {
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

	// A generated file that names another contract version was written by a dmt of another
	// contract; the objects may be labelled or named differently now, so it is regenerated (R40).
	for _, file := range model.Files {
		content, err := os.ReadFile(filepath.Join(modulePath, file.Path))
		if err != nil {
			continue
		}

		if generated, version := generate.ParseHeader(string(content)); generated && version != rbaccontract.ContractVersion {
			divergences[file.Path] = append(divergences[file.Path],
				fmt.Sprintf("the file was generated under contract version %q; the current contract is %q", version, rbaccontract.ContractVersion))
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
			fileList = fileList.WithFix(regenerateFix(modulePath, *file, actual))
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
		case object.Unstructured.GetKind() == "ClusterRole" && labels[rbaccontract.LabelKind] == rbaccontract.KindCapability && labels[rbaccontract.LabelModule] == r.module.GetName():
			out[identity] = managedObject{object, generate.ClassCapability}
		default:
			if _, ok := declared[identity]; ok {
				out[identity] = managedObject{object, generate.ClassDeclared}
			}
		}
	}

	return out
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
// --fix runs (R32). Two safeguards decide whether the file is written at all:
//
//   - a file without the generator header is maintained by hand: the generated text is written
//     beside it as <file>.generated and the finding stays (R16, US-F2);
//   - the regenerated file must grant everything the current render of that file grants (rules and
//     aggregation edges); if anything would disappear the file is left alone and the finding names
//     what would be lost (R25, D3). Removing a right is always a person's decision.
func regenerateFix(modulePath string, file generate.File, actual map[string]managedObject) errors.AutofixFunc {
	content := generate.RenderFile(file)
	expected := expectedRights(file)
	fullPath := filepath.Join(modulePath, file.Path)

	// Under --matrix the module is linted once per render variant and every variant collects its
	// own finding with its own closure. Each records what its render grants now, while the store
	// exists; the closure that runs first checks the union of them and writes, the others report
	// its outcome (R36). A right rendered only under some values is therefore not lost (D3).
	recordRenderedRights(fullPath, currentRights(file.Path, actual))

	return func() error {
		return fixOnce(fullPath, func() error {
			existing, err := os.ReadFile(fullPath)
			exists := err == nil

			if err != nil && !stderrors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("read %s: %w", file.Path, err)
			}

			if exists {
				if strings.Contains(string(existing), rbaccontract.GateMarker) || strings.Contains(string(existing), "deckhouseVersion") {
					return fmt.Errorf("%s renders one of two role models depending on the platform version (the %s gate of rbacv2-migrate-module.sh); regenerating it would drop the legacy branch -- edit the new branch by hand, or drop the gate and the legacy object once clusters below DKP 1.78 are no longer served, then run `%s`",
						file.Path, rbaccontract.GateMarker, FixCommand)
				}

				if generated, _ := generate.ParseHeader(string(existing)); !generated {
					aside := fullPath + ".generated"
					if err := os.WriteFile(aside, []byte(content), 0o600); err != nil {
						return fmt.Errorf("write %s: %w", file.Path+".generated", err)
					}

					return fmt.Errorf("%s is maintained by hand (no generator header); the generated version is beside it as %s.generated -- compare, then either delete the file and run `%s` again, or keep maintaining it by hand",
						file.Path, file.Path, FixCommand)
				}
			}

			if dropped := sortedSetDiff(renderedRights(fullPath), expected); len(dropped) > 0 {
				return fmt.Errorf("regenerating %s would drop rights the render grants today: %s; declare them in %s or remove them from the template by hand",
					file.Path, strings.Join(dropped, ", "), rbacyaml.Filename)
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

// currentRights collects what the render grants from the objects of this file: every tuple and
// aggregation edge, keyed by object, from the objects the model declares for the file and from
// the managed objects the render placed in the same template.
func currentRights(path string, actual map[string]managedObject) map[string]struct{} {
	out := map[string]struct{}{}

	for identity, obj := range actual {
		if obj.object.ShortPath() != path {
			continue
		}

		content := obj.object.Unstructured.UnstructuredContent()

		switch obj.object.Unstructured.GetKind() {
		case "ClusterRole":
			role := new(rbacv1.ClusterRole)
			if runtime.DefaultUnstructuredConverter.FromUnstructured(content, role) == nil {
				for t := range expandRenderedRules(role.Rules) {
					out[identity+": "+t.String()] = struct{}{}
				}

				for l := range lineagesOfLabels(role.Labels) {
					out[identity+": aggregation into "+l] = struct{}{}
				}
			}
		case "Role":
			role := new(rbacv1.Role)
			if runtime.DefaultUnstructuredConverter.FromUnstructured(content, role) == nil {
				for t := range expandRenderedRules(role.Rules) {
					out[identity+": "+t.String()] = struct{}{}
				}
			}
		}
	}

	return out
}

// expectedRights collects what the generated file will grant, conditional rules included: they
// are in the text, so they are not lost by regeneration.
func expectedRights(file generate.File) map[string]struct{} {
	out := map[string]struct{}{}

	for _, o := range file.Objects {
		always, conditional := expandModelRules(o.Rules)

		for t := range always {
			out[o.Identity()+": "+t.String()] = struct{}{}
		}

		for t := range conditional {
			out[o.Identity()+": "+t.String()] = struct{}{}
		}

		for l := range lineagesOfLabels(o.Labels) {
			out[o.Identity()+": aggregation into "+l] = struct{}{}
		}
	}

	return out
}
