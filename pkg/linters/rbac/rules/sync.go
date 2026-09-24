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
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template/parse"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/pkg/log"

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
		declList.WithFix(manualFix("move the declaration out of the edition overlay")).Errorf("%s lies in the edition overlay %s; the declaration describes the union of editions and belongs to modules/<module>/ only -- CI merges the overlays over modules/ before linting, so a copy here would shadow it or go unseen. Only a person can close this: move the file",
			rbacyaml.Filename, overlay)

		return
	}

	if err != nil {
		if content, readErr := os.ReadFile(rbacyaml.Path(modulePath)); readErr == nil && !strings.Contains(string(content), "apiVersion:") {
			declList.WithFix(manualFix("delete the rbac.yaml of an earlier shape")).Errorf("%s is not a declaration (no apiVersion): an rbac.yaml of an earlier shape that nothing reads; delete it and run `%s` to write the declaration from the render", rbacyaml.Filename, FixCommand)
			return
		}

		declList.WithFix(manualFix("make the declaration parse")).Errorf("%v; nothing is compared or generated until the declaration parses", err)

		return
	}

	// A CRD document that does not parse is reported by coverage; the declaration is judged
	// against the CRDs that do.
	crds, _ := moduleCRDs(modulePath)

	meta, err := readModuleMetadata(modulePath)
	if err != nil {
		r.errorList.WithFilePath("module.yaml").WithFix(manualFix("make module.yaml parse")).Errorf("%v; nothing is compared or generated until it parses: its subsystems decide the aggregation of every system capability", err)
		return
	}

	if errs := rbacyaml.Validate(decl, crdScopes(crds)); len(errs) > 0 {
		for _, e := range errs {
			declList.WithFix(manualFix("correct the declaration")).Errorf("%v; nothing is compared or generated until the declaration is valid", e)
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
		declList.WithFix(manualFix("correct the declaration")).Errorf("cannot derive the RBAC objects from the declaration: %v", err)
		return
	}

	actual := r.managedObjects(model)
	divergences := r.compareRender(model, actual)
	r.compareText(modulePath, model, actual, divergences)
	r.report(modulePath, model, divergences)
}

// compareRender judges the declaration against the rendered objects, both ways: every produced
// object must be rendered as produced, and every legacy role or module capability rendered must be
// produced. The findings are collected per template.
func (r *SyncRule) compareRender(model *generate.Model, actual map[string]managedObject) map[string][]string {
	modulePath := r.module.GetPath()
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

		found := compareFile(r.enabledObjects(file), actual, r.module.GetName())

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

	divergences = r.replacedCopies(model, actual, modelIdentities, divergences)

	for identity, obj := range actual {
		if _, produced := modelIdentities[identity]; produced || obj.class == generate.ClassDeclared {
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
			why = "it has no rules and grants nothing, so the regeneration drops it -- remove it from the template"
		}

		divergences[obj.object.ShortPath()] = append(divergences[obj.object.ShortPath()],
			fmt.Sprintf("%s is in the render but rbac.yaml does not produce it: %s", identity, why))
	}

	return divergences
}

// compareText judges the generated files by their text: a file that carries the generator header
// must be what the declaration renders now, and a file that does not exist while an object it
// holds is absent from the render was never written.
func (r *SyncRule) compareText(modulePath string, model *generate.Model, actual map[string]managedObject, divergences map[string][]string) {
	placed := map[string]string{}

	for _, f := range model.Files {
		for _, o := range f.Objects {
			placed[o.Identity()] = f.Path
		}
	}

	// A rule under `when` whose condition is false today is absent from the render without being
	// a divergence (D4), yet it still has to reach the template -- so for these files the text is
	// compared too. A file of another contract version is the same case (R40). A file without the
	// header is maintained by hand and is judged by its render only.
	for _, file := range model.Files {
		content, err := os.ReadFile(filepath.Join(modulePath, file.Path))
		if err != nil {
			// A file that does not exist while an object it holds is absent from the render: the
			// render cannot tell a conditional object whose condition is false from one whose
			// template was never written, but the text can -- nothing produces it (D4 covers the
			// render, not the file).
			switch {
			case !stderrors.Is(err, os.ErrNotExist):
				divergences[file.Path] = append(divergences[file.Path], fmt.Sprintf("the file cannot be read: %v", err))
			case hasAbsentObject(file, actual):
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

		if generated {
			produced := make(map[string]struct{}, len(file.Objects))
			for _, o := range file.Objects {
				produced[o.Identity()] = struct{}{}
			}

			divergences[file.Path] = append(divergences[file.Path], noLongerProduced(string(content), produced, placed)...)
		}
	}

	// A generated file the declaration produces nothing for any more renders objects no model
	// file names; it is reported under its own path so that the orphan fix can judge it.
	inModel := make(map[string]struct{}, len(model.Files))
	for _, f := range model.Files {
		inModel[f.Path] = struct{}{}
	}

	// The render shows only the files whose objects render under these values; a generated file
	// whose every object is under a false condition is found on disk.
	candidates := map[string]struct{}{}

	for _, object := range r.module.GetStorage() {
		candidates[object.ShortPath()] = struct{}{}
	}

	for _, path := range generatedTemplates(modulePath) {
		candidates[path] = struct{}{}
	}

	for _, path := range slices.Sorted(maps.Keys(candidates)) {
		if _, ok := inModel[path]; ok {
			continue
		}

		content, err := os.ReadFile(filepath.Join(modulePath, path))
		if err != nil {
			continue
		}

		if generated, _ := generate.ParseHeader(string(content)); generated {
			divergences[path] = append(divergences[path], noLongerProduced(string(content), nil, placed)...)
		}
	}
}

// noLongerProduced lists, as divergences, the objects the header of a generated file names as the
// generator's that the declaration no longer produces there.
func noLongerProduced(content string, produced map[string]struct{}, placed map[string]string) []string {
	owned, _ := generate.ParseOwned(content)

	out := make([]string, 0, len(owned))

	for id := range owned {
		if _, ok := produced[id]; ok {
			continue
		}

		if where, moved := placed[id]; moved {
			out = append(out, id+" is now declared in "+where+"; the fix does not move objects between files -- move it by hand")
			continue
		}

		out = append(out, id+" was generated into this file and the declaration no longer produces it")
	}

	sort.Strings(out)

	return out
}

// report emits one finding per template, with the fix that closes it when one exists: the
// regeneration of a produced file, the deletion of an orphaned generated file, or none.
func (r *SyncRule) report(modulePath string, model *generate.Model, divergences map[string][]string) {
	// What every generated file holds by its text, rendered or not: an object under a false
	// condition that the declaration moved is in its old file's text only, and writing it into the
	// new one would define it twice once the condition holds.
	held := map[string][]string{}

	for _, path := range generatedTemplates(modulePath) {
		content, err := os.ReadFile(filepath.Join(modulePath, path))
		if err != nil {
			continue
		}

		for _, doc := range textDocuments(string(content)) {
			held[doc.id] = append(held[doc.id], path)
		}
	}

	placed := map[string]string{}

	for _, f := range model.Files {
		for _, o := range f.Objects {
			placed[o.Identity()] = f.Path
		}
	}

	paths := make([]string, 0, len(divergences))
	for path, list := range divergences {
		if len(list) > 0 {
			paths = append(paths, path)
		}
	}

	sort.Strings(paths)

	var dropped map[string]string
	if store := r.module.GetObjectStore(); store != nil {
		dropped = store.Dropped
	}

	// Under --matrix another variant may regenerate a file this one could not render; record it
	// for every variant's fix, whether or not this variant reports the file.
	for path, cause := range dropped {
		recordDropped(filepath.Join(modulePath, path), cause)
	}

	for _, path := range paths {
		list := divergences[path]
		sort.Strings(list)

		fileList := r.errorList.WithFilePath(path).WithObjectID(path)

		// A template the render skipped is missing from the storage without being missing from
		// the chart: its objects -- the foreign ones included -- were never seen, so neither the
		// comparison nor a rewrite can be trusted.
		if cause, skipped := dropped[path]; skipped {
			recordDropped(filepath.Join(modulePath, path), cause)
			fileList.WithFix(manualFix("make "+path+" render")).Errorf("%s failed to render in this run (%s); nothing in it is compared or regenerated until it renders", path, cause)

			continue
		}

		if file := model.File(path); file != nil {
			// An object this file produces that still renders from another file stays there until a
			// person moves it; writing it here as well would render it twice.
			recordBlocked(filepath.Join(modulePath, path), r.renderedElsewhere(*file))
			recordBlocked(filepath.Join(modulePath, path), heldElsewhere(*file, held))
			fileList = fileList.WithFix(regenerateFix(modulePath, *file, placed, r.foreignObjects(*file, model), removalsOf(list)))
			fileList.Errorf("%s does not match %s: %s. Run `%s` to regenerate the file from the declaration",
				path, rbacyaml.Filename, strings.Join(list, "; "), FixCommand)

			continue
		}

		// A file the generator wrote earlier that the declaration produces nothing for any more -- a
		// legacy section dropped, every namespace level gone -- is an orphan: the declaration wins,
		// and the fix deletes it, as long as it holds nothing but objects of the owned classes.
		if orphan, reason := r.orphanGeneratedFile(path, model); orphan {
			// Another render variant may place an object in the file that this one does not see;
			// the fix judges the union, as the regeneration does.
			recordForeignObjects(filepath.Join(modulePath, path), nil)

			fileList.WithFix(removeFileFix(modulePath, path, placed, list)).Errorf("%s does not match %s: %s. The file carries the generator header and the declaration produces nothing for it; `%s` deletes it",
				path, rbacyaml.Filename, strings.Join(list, "; "), FixCommand)

			continue
		} else if reason != "" {
			list = append(list, reason)
		}

		fileList.WithFix(manualFix("edit "+path+" by hand")).Errorf("%s does not match %s: %s. Only a person can close this: the declaration does not produce this file",
			path, rbacyaml.Filename, strings.Join(list, "; "))
	}
}

// orphanGeneratedFile reports whether a template the declaration produces nothing for is the
// generator's (header present) and holds only objects of the owned classes, so deleting it loses
// nothing the declaration does not know about. Otherwise it returns why the file stays. Objects
// outside the owned classes are recorded for the fix, which judges the union over render variants.
func (r *SyncRule) orphanGeneratedFile(path string, model *generate.Model) (bool, string) {
	if model.File(path) != nil {
		return false, ""
	}

	fullPath := filepath.Join(r.module.GetPath(), path)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return false, ""
	}

	if generated, _ := generate.ParseHeader(string(content)); !generated {
		return false, ""
	}

	if templateGated(string(content), "") {
		return false, "the file serves both role models behind the version gate"
	}

	foreign := r.foreignIn(path, nil, nil, model)

	if len(foreign) > 0 {
		sort.Strings(foreign)
		recordForeignObjects(fullPath, foreign)

		return false, "the file also holds " + strings.Join(foreign, ", ") + ", which the declaration does not describe"
	}

	return true, ""
}

// removeFileFix deletes an orphaned generated file and logs what went with it. It re-reads the
// file when it runs and refuses when any render variant placed an object in it that the
// declaration does not describe, or when the header or the gate say the file is not the
// generator's to delete.
func removeFileFix(modulePath, path string, placed map[string]string, removed []string) errors.AutofixFunc {
	fullPath := filepath.Join(modulePath, path)
	recordRemovals(fullPath, removed)

	return func() error {
		return fixOnce(fullPath, func() error {
			if err := fixBlocked(modulePath, path); err != nil {
				return err
			}

			removed := recordedRemovals(fullPath)

			if foreign := foreignObjectsOf(fullPath); len(foreign) > 0 {
				return fmt.Errorf("%s also holds objects the declaration does not describe (%s), some only under other values; it is not deleted -- declare them in %s or move them to another template",
					path, strings.Join(foreign, ", "), rbacyaml.Filename)
			}

			content, err := os.ReadFile(fullPath)
			if err != nil {
				if stderrors.Is(err, os.ErrNotExist) {
					return nil
				}

				return fmt.Errorf("read %s: %w", path, err)
			}

			if generated, _ := generate.ParseHeader(string(content)); !generated || templateGated(string(content), "") {
				return fmt.Errorf("%s is not the generator's to delete any more (no header, or the version gate); remove it by hand if that is the intent", path)
			}

			// Found on disk, the file may hold objects no variant rendered; only a file whose every
			// object the generator lists as its own and the declaration no longer places anywhere
			// is deleted.
			if unknown := notTheGenerators(string(content), nil, placed, path); len(unknown) > 0 {
				return fmt.Errorf("%s holds objects the fix cannot account for: %s; it is not deleted -- move or remove them by hand", path, strings.Join(unknown, ", "))
			}

			if err := os.Remove(fullPath); err != nil {
				return fmt.Errorf("delete %s: %w", path, err)
			}

			log.Warn("rbac autofix deleted a generated template the declaration produces nothing for",
				slog.String("file", path), slog.Any("removed", removed))

			return nil
		})
	}
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

// foreignObjects lists the rendered objects of the file that a regeneration would drop without the
// declaration knowing them: everything that is neither produced now nor the generator's own.
func (r *SyncRule) foreignObjects(file generate.File, model *generate.Model) []string {
	produced := make(map[string]struct{}, len(file.Objects))
	for _, o := range file.Objects {
		produced[o.Identity()] = struct{}{}
	}

	return r.foreignIn(file.Path, produced, file.Objects, model)
}

// foreignIn judges every rendered object of the template at path, of any kind. An object the
// declaration produces is kept. An object the header lists as the generator's (contract 2) is a
// removal: the declaration no longer names it and wins (D14). In a file whose header lists no
// objects -- a contract 1 file or one maintained by hand -- a legacy role or a module capability
// the declaration does not produce is a removal for the same reason, and an RBAC object the
// generator now produces under another name is a rename. Everything else is someone else's: a
// ConfigMap, a Secret, a hand-written role -- and the fix refuses to drop it.
func (r *SyncRule) foreignIn(path string, produced map[string]struct{}, producedObjects []generate.Object, model *generate.Model) []string {
	fullPath := filepath.Join(r.module.GetPath(), path)
	owned, listed := ownedBy(fullPath)

	// Where the declaration puts every object it produces: an object rendered from another file
	// than that is misplaced rather than unknown, and the refusal says so.
	placed := map[string]string{}

	for _, f := range model.Files {
		for _, o := range f.Objects {
			placed[o.Identity()] = f.Path
		}
	}

	managed := r.managedObjects(model)
	storage := r.module.GetStorage()

	rendered := make(map[string]struct{}, len(storage))
	for index := range storage {
		rendered[index.AsString()] = struct{}{}
	}

	var out []string

	for index, object := range storage {
		if object.ShortPath() != path {
			continue
		}

		id := index.AsString()

		if _, ok := produced[id]; ok {
			continue
		}

		// Produced in another file of the model. The fix does not move objects between files --
		// the target may be maintained by hand, gated or refused, and the object would be lost or
		// rendered twice -- so this file is left alone until a person moves it.
		if where, declared := placed[id]; declared && where != path {
			out = append(out, id+" (the declaration now puts it in "+where+"; the fix does not move objects between files -- move it there by hand, or delete this file, then run the fix)")
			continue
		}

		if _, ok := owned[id]; ok {
			continue
		}

		if !listed && isRBACKind(object.Unstructured.GetKind()) {
			// exclude-rules.sync silences an object's finding; it must not turn the object into a
			// silent removal either.
			if m, ok := managed[id]; ok && m.class != generate.ClassDeclared && r.Enabled(object.Unstructured.GetKind(), object.Unstructured.GetName()) {
				continue
			}

			if replacedByProduced(object, producedObjects, renderedRolesOf(storage, path), rendered) {
				continue
			}
		}

		out = append(out, id)
	}

	sort.Strings(out)

	return out
}

// ownedBy reads the objects the header of a generated file lists as the generator's, and whether
// it lists any at all.
func ownedBy(fullPath string) (map[string]struct{}, bool) {
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, false
	}

	if generated, _ := generate.ParseHeader(string(content)); !generated {
		return nil, false
	}

	return generate.ParseOwned(string(content))
}

// hasHeader reports whether the file carries the generator header.
func hasHeader(fullPath string) bool {
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return false
	}

	generated, _ := generate.ParseHeader(string(content))

	return generated
}

// isRBACKind reports whether the kind is one the generator produces.
func isRBACKind(kind string) bool {
	switch kind {
	case "ClusterRole", "Role", "ClusterRoleBinding", "RoleBinding", "ServiceAccount":
		return true
	}

	return false
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

		always, conditional := expandModelRules(o.Rules)
		for t := range conditional {
			always.add(t)
		}

		return len(always.minus(rendered)) == 0 && len(rendered.minus(always)) == 0
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
//
// removalsOf picks, from a file's divergences, what a regeneration takes away: rights and objects
// the render has and the declaration does not name. They are logged when the file is written, so a
// --fix run without a preceding dmt lint does not remove rights in silence.
func removalsOf(divergences []string) []string {
	// Every divergence is something the regeneration changes in the cluster -- a right removed, a
	// token taken away, a level or a roleRef changed -- except the two that are only about the text.
	var out []string

	for _, d := range divergences {
		if strings.HasPrefix(d, "the file carries the generator header but is not what the declaration renders now") ||
			strings.HasPrefix(d, "the file was generated under contract version") {
			continue
		}

		out = append(out, d)
	}

	return out
}

func regenerateFix(modulePath string, file generate.File, placed map[string]string, foreign, removals []string) errors.AutofixFunc {
	content := generate.RenderFile(file)
	fullPath := filepath.Join(modulePath, file.Path)

	// Under --matrix the module is linted once per render variant and every variant collects its
	// own finding with its own closure. Each records what its render grants now, while the store
	// exists; the closure that runs first checks the union of them and writes, the others report
	// its outcome (R36). A right rendered only under some values is therefore not lost (D3).
	recordForeignObjects(fullPath, foreign)
	recordRemovals(fullPath, removals)

	return func() error {
		return fixOnce(fullPath, func() error {
			if err := fixBlocked(modulePath, file.Path); err != nil {
				return err
			}

			removals := recordedRemovals(fullPath)

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

				// The render shows only what renders under these values; the text shows everything the
				// file holds, objects under a false condition included.
				if generated, _ := generate.ParseHeader(string(existing)); generated {
					if unknown := notTheGenerators(string(existing), identitiesOf(file.Objects), placed, file.Path); len(unknown) > 0 {
						return fmt.Errorf("%s holds objects the fix cannot account for: %s; they are not what the generator wrote there, so regenerating the file would drop them -- declare them in %s, move them, or remove them by hand, then run `%s` again",
							file.Path, strings.Join(unknown, ", "), rbacyaml.Filename, FixCommand)
					}
				}

				if templateGated(string(existing), content) {
					return fmt.Errorf("%s renders one of two role models depending on the platform version (the %s gate of rbacv2-migrate-module.sh, or a deckhouseVersion test the declaration did not produce); regenerating it would drop the legacy branch -- edit the new branch by hand, or drop the gate and the legacy object once clusters below DKP 1.78 are no longer served, then run `%s`",
						file.Path, rbaccontract.GateMarker, FixCommand)
				}

				if generated, _ := generate.ParseHeader(string(existing)); !generated {
					aside := asidePath(fullPath)
					if err := writeFileAtomic(aside, []byte(content), 0o644); err != nil { //nolint:gosec // a source file of the module
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

			if err := writeFileAtomic(fullPath, []byte(content), perm); err != nil {
				return err
			}

			// The declaration is the source: what it no longer names left the file. Say so where a
			// --fix run without a preceding lint would otherwise remove it in silence.
			if len(removals) > 0 {
				// The divergences go both ways: what the render has and the declaration does not
				// leaves, what the declaration has and the render lacks arrives.
				added, removed := splitChanges(removals)
				log.Warn("rbac autofix regenerated a template: what the render had and the declaration does not name is removed, what the declaration names is added",
					slog.String("file", file.Path), slog.Any("removed", removed), slog.Any("added", added))
			} else {
				log.Info("rbac autofix regenerated a template from rbac.yaml", slog.String("file", file.Path))
			}

			return nil
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

	meta, err := readModuleMetadata(modulePath)
	if err != nil {
		r.errorList.WithFilePath("module.yaml").WithFix(manualFix("make module.yaml parse")).Errorf("%v; the declaration is not written until it parses", err)
		return
	}

	in := bootstrap.Input{Module: r.module.GetName(), Namespace: r.module.GetNamespace(), Subsystems: meta.Subsystems, CRDs: map[string]string{}}

	crds, _ := moduleCRDs(modulePath)
	for _, crd := range crds {
		in.CRDs[crd.Key()] = crd.Scope
	}

	texts := map[string]string{}

	for _, object := range storage {
		if o, ok := bootstrapObject(object); ok {
			text, read := texts[o.Path]
			if !read {
				if content, err := os.ReadFile(filepath.Join(modulePath, o.Path)); err == nil {
					text = string(content)
				}

				texts[o.Path] = text
			}

			o.Library = renderedByInclude(text, o.Kind, o.Name)
			o.Repeated = renderedInRange(text, o.Kind, o.Name)
			in.Objects = append(in.Objects, o)
		}
	}

	if len(in.Objects) == 0 {
		return
	}

	markLibraryFiles(in.Objects)

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
			result := bootstrap.Build(in)

			content, err := bootstrap.Marshal(result)
			if err != nil {
				return fmt.Errorf("render %s: %w", rbacyaml.Filename, err)
			}

			return writeBootstrapped(path, content, crds, in)
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

// openDecisions counts the TODO values in a written declaration; the header comment that explains
// them does not count.
func openDecisions(content string) int {
	n := 0

	for line := range strings.SplitSeq(content, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			n += strings.Count(line, "TODO")
		}
	}

	return n
}

// fixBlocked says why a fix must leave the file alone although this variant would rewrite it: a
// render variant skipped the template, or an object it would write still renders from another file.
func fixBlocked(modulePath, path string) error {
	fullPath := filepath.Join(modulePath, path)

	if cause, dropped := droppedCause(fullPath); dropped {
		return fmt.Errorf("%s failed to render under some values (%s); it is not rewritten until it renders in every variant", path, cause)
	}

	if elsewhere := blockedBy(fullPath); len(elsewhere) > 0 {
		return fmt.Errorf("%s would produce objects that still render from another file: %s; writing them here would render them twice -- move them by hand, then run `%s` again", path, strings.Join(elsewhere, ", "), FixCommand)
	}

	return nil
}

// generatedTemplates lists, relative to the module, the files under templates/ that carry the
// generator header.
func generatedTemplates(modulePath string) []string {
	var out []string

	root := filepath.Join(modulePath, "templates")

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // an unreadable entry is not a generated file
		}

		// A `_<file>.generated` aside is the generator's proposal beside a hand-maintained file, not
		// a template of the chart.
		if base := filepath.Base(path); strings.HasPrefix(base, "_") || strings.HasSuffix(base, ".generated") {
			return nil
		}

		if hasHeader(path) {
			if rel, relErr := filepath.Rel(modulePath, path); relErr == nil {
				out = append(out, filepath.ToSlash(rel))
			}
		}

		return nil
	})

	return out
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

// notTheGenerators reads the objects a generated file holds from its text -- rendered or not -- and
// returns those the fix cannot account for: not produced into this file now, and either placed in
// another file of the model (the fix does not move objects) or not the generator's at all. A
// contract 2 file lists the generator's objects in its header; in a contract 1 file only a legacy
// role or a module capability, recognized by its markers, counts as the generator's.
func notTheGenerators(content string, produced map[string]struct{}, placed map[string]string, path string) []string {
	owned, listed := generate.ParseOwned(content)

	var out []string

	for _, doc := range textDocuments(content) {
		if _, ok := produced[doc.id]; ok {
			continue
		}

		if where, ok := placed[doc.id]; ok && where != path {
			out = append(out, doc.id+" (now declared in "+where+")")
			continue
		}

		_, isOwned := owned[doc.id]

		switch {
		case listed && isOwned:
		case !listed && doc.managed:
		default:
			out = append(out, doc.id)
		}
	}

	sort.Strings(out)

	return out
}

// textDocument is what the fix needs to know about one object of a generated file's text.
type textDocument struct {
	id      string
	managed bool // a legacy role or a module capability
}

var (
	kindLineRe      = regexp.MustCompile(`^kind:\s*(\S+)\s*(#.*)?$`)
	nameLineRe      = regexp.MustCompile(`^  name:\s*"?([^"\s#]+)"?\s*(#.*)?$`)
	namespaceLineRe = regexp.MustCompile(`^  namespace:\s*"?([^"\s#]+)"?\s*(#.*)?$`)
	separatorRe     = regexp.MustCompile(`(?m)^---[ \t]*(#.*)?$`)
	// wrapperLineRe matches the lines the generator puts between objects: its conditions and
	// their ends. Anything else outside an object is content the fix does not understand.
	wrapperLineRe = regexp.MustCompile(`^\s*(\{\{-?\s*(if|else|end)\b[^}]*-?\}\}\s*)*$`)
	// labelsLineRe is the only other template action the generator writes: the module labels, with
	// no labels of its own or a dict of quoted literals (generate.labelsInclude). Anything else on
	// that line -- another include, labels from the values -- is not the generator's (review of
	// #479, finding 31).
	labelsLineRe = regexp.MustCompile(`^  \{\{- include "helm_lib_module_labels" \(list \.(?: \(dict(?: "(?:[^"\\]|\\.)*" "(?:[^"\\]|\\.)*")+\))?\) \| nindent 2 \}\}$`)
)

// textDocuments parses the objects of a generated file from its text: the generator writes kind,
// metadata.name and metadata.namespace on lines of their own. A document it cannot read -- an
// include, a templated name, anything that is neither an object nor the generator's own
// wrapper lines -- yields an identity that matches nothing, so the fix refuses rather than guesses.
func textDocuments(content string) []textDocument {
	content = strings.ReplaceAll(content, "\r\n", "\n")

	var out []textDocument

	for i, doc := range separatorRe.Split(content, -1) {
		var kind, name, namespace string

		inMetadata := false
		other := false
		// An action the generator does not write -- an include, a range, a value from the values --
		// makes the document someone else's wherever it stands, after the kind line too: what it
		// renders under other values is not in this render (review of #479, finding 31).
		foreign := false

		for _, line := range strings.Split(doc, "\n") {
			if strings.Contains(line, "{{") && !wrapperLineRe.MatchString(line) && !labelsLineRe.MatchString(line) {
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
			out = append(out, textDocument{id: fmt.Sprintf("<unreadable document %d>", i)})
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

		out = append(out, textDocument{id: id, managed: managed})
	}

	return out
}

// heldElsewhere lists the objects the file produces that another generated file's text holds.
func heldElsewhere(file generate.File, held map[string][]string) []string {
	var out []string

	for _, o := range file.Objects {
		for _, path := range held[o.Identity()] {
			if path != file.Path {
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
func (r *SyncRule) replacedCopies(model *generate.Model, actual map[string]managedObject, produced map[string]struct{}, divergences map[string][]string) map[string][]string {
	storage := r.module.GetStorage()

	rendered := make(map[string]struct{}, len(storage))
	for index := range storage {
		rendered[index.AsString()] = struct{}{}
	}

	n := 0
	for _, f := range model.Files {
		n += len(f.Objects)
	}

	all := make([]generate.Object, 0, n)
	for _, f := range model.Files {
		all = append(all, f.Objects...)
	}

	copies := map[string]string{}
	renderedGrantees := renderedRoleSubjects(storage)
	declaredGrantees := modelRoleSubjects(all)

	for index, object := range storage {
		id := index.AsString()
		if _, ok := produced[id]; ok {
			continue
		}

		if _, ok := actual[id]; ok {
			continue
		}

		kind := object.Unstructured.GetKind()
		if !isRBACKind(kind) || kind == "ServiceAccount" {
			continue
		}

		if twin := renderedTwin(object, all, rendered); twin != "" {
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
	for index, object := range storage {
		kind := object.Unstructured.GetKind()
		if kind != "RoleBinding" && kind != "ClusterRoleBinding" {
			continue
		}

		if _, ok := produced[index.AsString()]; ok {
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

			got := expandRenderedRules(role.Rules)
			always, conditional := expandModelRules(o.Rules)

			for t := range conditional {
				always.add(t)
			}

			if len(always.minus(got)) == 0 && len(got.minus(always)) == 0 {
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

// manualFix is the fix of a finding only a person can close: nothing is generated while it
// stands, so `--fix` must not report success (it fails with what to do).
func manualFix(what string) errors.AutofixFunc {
	return func() error {
		return fmt.Errorf("nothing was generated: %s first", what)
	}
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
// A declaration that does not parse would be a bug of dmt; it is written all the same, so the
// module's developer sees the file and the parse error rather than nothing.
func writeBootstrapped(path string, content []byte, crds []crdInfo, in bootstrap.Input) error {
	if err := writeFileAtomic(path, content, 0o644); err != nil { //nolint:gosec // a source file of the module
		return err
	}

	if _, err := rbacyaml.Parse(content); err != nil {
		return fmt.Errorf("%s is written, but it does not parse: %w; this is a bug of dmt, report it with the module", rbacyaml.Filename, err)
	}

	// The file is written, but a TODO in it is a decision nobody has made yet: the finding stays,
	// and so does the non-zero exit, until a person makes it (ADR, bootstrap). What the linter
	// would refuse in the written file is named here too, rather than on the next run.
	problems := writtenProblems(content, crds, in)

	if open := openDecisions(string(content)); open > 0 {
		return fmt.Errorf("%s is written; %d TODO in it are decisions only a person can make%s -- resolve them, then run `%s` to regenerate the templates", rbacyaml.Filename, open, problems, FixCommand)
	}

	if problems != "" {
		return fmt.Errorf("%s is written%s -- correct it, then run `%s` to regenerate the templates", rbacyaml.Filename, problems, FixCommand)
	}

	return nil
}

// writtenProblems lists what the linter refuses in a declaration bootstrap wrote, the TODO
// values aside: they are counted on their own.
func writtenProblems(content []byte, crds []crdInfo, in bootstrap.Input) string {
	decl, err := rbacyaml.Parse(content)
	if err != nil {
		return "; it does not parse: " + err.Error()
	}

	var problems []string

	for _, e := range rbacyaml.Validate(decl, crdScopes(crds)) {
		if !strings.Contains(e.Error(), "TODO") {
			problems = append(problems, e.Error())
		}
	}

	if len(problems) == 0 {
		if _, err := generate.Build(generate.Input{Module: in.Module, Namespace: in.Namespace, Subsystems: in.Subsystems, Decl: decl}); err != nil && !strings.Contains(err.Error(), "TODO") {
			problems = append(problems, err.Error())
		}
	}

	if len(problems) == 0 {
		return ""
	}

	return "; the linter refuses: " + strings.Join(problems, "; ")
}

var (
	// includeRe finds an include of a named template (or the template action) in a document.
	includeRe = regexp.MustCompile(`\{\{-?\s*(include|template)\s+"`)
	// rawNameLineRe is a metadata name as written, computed or not.
	rawNameLineRe = regexp.MustCompile(`^  name:\s*(.+?)\s*$`)
	actionRe      = regexp.MustCompile(`\{\{.*?\}\}`)
)

// renderedByInclude reports whether the template renders the object through an include of a
// named template -- helm_lib_csi_controller_rbac, typically -- rather than through a document of
// its own: no document of the object's kind names it (literally or with a computed name), and a
// document without a kind of its own includes a template.
func renderedByInclude(content, kind, name string) bool {
	include := false

	for _, doc := range separatorRe.Split(strings.ReplaceAll(content, "\r\n", "\n"), -1) {
		var docKind, docName string

		for _, line := range strings.Split(doc, "\n") {
			if m := kindLineRe.FindStringSubmatch(line); m != nil && docKind == "" {
				docKind = m[1]
			}

			if m := rawNameLineRe.FindStringSubmatch(line); m != nil && docName == "" {
				docName = m[1]
				if i := strings.Index(docName, " #"); i >= 0 && !strings.Contains(docName, "{{") {
					docName = strings.TrimSpace(docName[:i])
				}

				docName = strings.Trim(docName, `"'`)
			}
		}

		switch {
		case docKind == "":
			include = include || includeRe.MatchString(doc)
		case docKind == kind && namesMatch(docName, name):
			return false
		}
	}

	return include
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

// namesMatch reports whether a metadata name as the template writes it can be the rendered name:
// equal, or with every action of a computed name standing for any text.
func namesMatch(written, rendered string) bool {
	if !strings.Contains(written, "{{") {
		return written == rendered
	}

	var b strings.Builder

	b.WriteString("^")

	last := 0
	for _, m := range actionRe.FindAllStringIndex(written, -1) {
		b.WriteString(regexp.QuoteMeta(written[last:m[0]]))
		b.WriteString(".*")

		last = m[1]
	}

	b.WriteString(regexp.QuoteMeta(written[last:]) + "$")

	re, err := regexp.Compile(b.String())

	return err == nil && re.MatchString(rendered)
}

// renderedInRange reports whether the object's document sits inside a {{ range }} of its
// template: one document renders several objects, and the declaration would freeze the one this
// render produced under its literal name (istio's istiod-<version>). The template is read with
// text/template/parse, not with patterns; a template that does not parse tells nothing.
func renderedInRange(content, kind, name string) bool {
	content = strings.ReplaceAll(content, "\r\n", "\n")

	offset := documentOffset(content, kind, name)
	if offset < 0 {
		return false
	}

	tree := parse.New("template")
	tree.Mode = parse.SkipFuncCheck

	if _, err := tree.Parse(content, "", "", map[string]*parse.Tree{}); err != nil || tree.Root == nil {
		return false
	}

	return inRange(tree.Root, offset, false)
}

// documentOffset is the offset of the metadata name line of the object's document, or -1.
func documentOffset(content, kind, name string) int {
	start := 0

	for _, bounds := range append(separatorRe.FindAllStringIndex(content, -1), []int{len(content), len(content)}) {
		doc := content[start:bounds[0]]
		docStart := start
		start = bounds[1]

		var docKind string

		offset, lineStart := -1, docStart

		for _, line := range strings.SplitAfter(doc, "\n") {
			trimmed := strings.TrimRight(line, "\n")

			if m := kindLineRe.FindStringSubmatch(trimmed); m != nil && docKind == "" {
				docKind = m[1]
			}

			if m := rawNameLineRe.FindStringSubmatch(trimmed); m != nil && offset < 0 && namesMatch(strings.Trim(m[1], `"'`), name) {
				offset = lineStart
			}

			lineStart += len(line)
		}

		if docKind == kind && offset >= 0 {
			return offset
		}
	}

	return -1
}

// inRange reports whether the text at offset lies in the body of a range.
func inRange(node parse.Node, offset int, ranged bool) bool {
	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return false
		}

		for _, c := range n.Nodes {
			if inRange(c, offset, ranged) {
				return true
			}
		}
	case *parse.TextNode:
		start := int(n.Position())

		return ranged && offset >= start && offset < start+len(n.Text)
	case *parse.IfNode:
		return inRange(n.List, offset, ranged) || inRange(n.ElseList, offset, ranged)
	case *parse.WithNode:
		return inRange(n.List, offset, ranged) || inRange(n.ElseList, offset, ranged)
	case *parse.RangeNode:
		return inRange(n.List, offset, true) || inRange(n.ElseList, offset, ranged)
	}

	return false
}
