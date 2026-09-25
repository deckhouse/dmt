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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/bootstrap"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbaccontract"
)

// fixState is what the autofixes of the coverage and sync rules share across render variants of
// one run. dmt lints a module once per variant under --matrix and every variant collects its own
// finding with its own closure; the state makes them behave as one fix per target (spec 005 R36):
//
//   - outcomes remembers the result of the first closure that ran for a target, so the others
//     return it instead of doing the work again, and every copy of the finding ends the run in
//     the same state;
//   - withheld names the files some variant reported without a fix: the lint found a case only a
//     change of the templates or the declaration closes, and the fix of another variant must not
//     rewrite the file under it.
var fixState = struct {
	sync.Mutex
	withheld  map[string]struct{}
	removals  map[string]map[string]struct{}
	bootstrap map[string]map[string]bootstrap.Object
	// variants counts the render variants that recorded bootstrap objects, seen how many of them
	// rendered each object: under --matrix an object seen in fewer renders only under some values.
	variants map[string]int
	seen     map[string]map[string]int
	// in names, per object, the variants that rendered it (review of #479, finding 52).
	in map[string]map[string]string
}{
	withheld:  map[string]struct{}{},
	removals:  map[string]map[string]struct{}{},
	bootstrap: map[string]map[string]bootstrap.Object{},
	variants:  map[string]int{},
	seen:      map[string]map[string]int{},
	in:        map[string]map[string]string{},
}

// fixOutcomes remembers the result of every fix that ran, by file. It has a lock of its own, held
// while the fix runs: a fix reads fixState, so the two must not share a mutex, and holding this one
// is what makes "once" hold under concurrent callers too, not only under the sequential
// Manager.ApplyFixes.
var fixOutcomes = struct {
	sync.Mutex
	done map[string]error
}{done: map[string]error{}}

// fixOnce runs fix for the key the first time it is asked and returns that outcome on every later
// call.
func fixOnce(key string, fix func() error) error {
	fixOutcomes.Lock()
	defer fixOutcomes.Unlock()

	if err, done := fixOutcomes.done[key]; done {
		return err
	}

	err := fix()
	fixOutcomes.done[key] = err

	return err
}

// withholdFix records that one render variant reported the file without a fix.
func withholdFix(file string) {
	fixState.Lock()
	defer fixState.Unlock()

	fixState.withheld[file] = struct{}{}
}

// fixWithheld reports whether any render variant reported the file without a fix.
func fixWithheld(file string) bool {
	fixState.Lock()
	defer fixState.Unlock()

	_, ok := fixState.withheld[file]

	return ok
}

// resetFixState forgets everything; tests call it between runs.
func resetFixState() {
	fixState.Lock()
	defer fixState.Unlock()

	fixState.withheld = map[string]struct{}{}
	fixState.removals = map[string]map[string]struct{}{}
	fixState.bootstrap = map[string]map[string]bootstrap.Object{}
	fixState.variants = map[string]int{}
	fixState.seen = map[string]map[string]int{}
	fixState.in = map[string]map[string]string{}

	fixOutcomes.Lock()
	defer fixOutcomes.Unlock()

	fixOutcomes.done = map[string]error{}
}

// recordBootstrapObjects adds the RBAC objects one render variant produced, for the first
// declaration to be written from the union of every variant.
func recordBootstrapObjects(path string, objects []bootstrap.Object) {
	fixState.Lock()
	defer fixState.Unlock()

	known := fixState.bootstrap[path]
	if known == nil {
		known = map[string]bootstrap.Object{}
		fixState.bootstrap[path] = known
	}

	fixState.variants[path]++
	if fixState.seen[path] == nil {
		fixState.seen[path] = map[string]int{}
		fixState.in[path] = map[string]string{}
	}

	for _, o := range objects {
		key := o.Kind + "/" + o.Namespace + "/" + o.Name
		known[key] = o
		fixState.seen[path][key]++
		fixState.in[path][key] += fmt.Sprintf("%d,", fixState.variants[path])
	}
}

// bootstrapObjectsOf returns, sorted by identity, every object any variant rendered.
func bootstrapObjectsOf(path string) []bootstrap.Object {
	fixState.Lock()
	defer fixState.Unlock()

	keys := make([]string, 0, len(fixState.bootstrap[path]))
	for k := range fixState.bootstrap[path] {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	out := make([]bootstrap.Object, 0, len(keys))
	for _, k := range keys {
		out = append(out, fixState.bootstrap[path][k])
	}

	return out
}

// editionOverlay returns the edition overlay a module directory lies in ("ee/modules",
// "ee/be/modules", ...) or "" for a module of the base tree or of an external repository. The
// declaration describes the union of editions and lives in modules/<module>/ only (spec 005 D7,
// R8a): CI merges the overlays over modules/ before linting, so a declaration in an overlay would
// either shadow the base one or go unseen.
func editionOverlay(modulePath string) string {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(modulePath)), "/")

	// parts[n-1] is the module directory, parts[n-2] must be "modules".
	n := len(parts)
	if n < 3 || parts[n-2] != "modules" {
		return ""
	}

	switch {
	case parts[n-3] == "ee":
		// ee/modules is merged over modules/ for a module that exists in both; an EE-only module
		// has no other directory, and ee/modules/<module> is its base.
		root := string(filepath.Separator) + filepath.Join(parts[:n-3]...)
		if !exists(filepath.Join(root, "modules", parts[n-1])) {
			return ""
		}

		return "ee/modules"
	case n >= 4 && parts[n-4] == "ee":
		// An edition directory is an overlay only for a module that has a base to merge over; a
		// module that lives in this edition alone (node-local-dns, cloud-provider-vsphere, ...) has
		// its base here.
		root := string(filepath.Separator) + filepath.Join(parts[:n-4]...)
		if !exists(filepath.Join(root, "modules", parts[n-1])) && !exists(filepath.Join(root, "ee", "modules", parts[n-1])) {
			return ""
		}

		return "ee/" + parts[n-3] + "/modules"
	default:
		return ""
	}
}

// templateHasGate reports whether the template a rendered object came from carries the version
// gate of rbacv2-migrate-module.sh, i.e. renders one of two role models depending on
// global.deckhouseVersion. An unreadable template counts as ungated.
func templateHasGate(modulePath, shortPath string) bool {
	content, err := os.ReadFile(filepath.Join(modulePath, shortPath))
	if err != nil {
		return false
	}

	return templateGated(string(content), "")
}

var (
	// gateActionRe matches a template action that tests the platform version. A mention of the
	// word in a comment or a value does not count.
	gateActionRe = regexp.MustCompile(`\{\{[^}]*deckhouseVersion`)
	// elseActionRe matches the other branch of a condition: a gate renders one model or the other,
	// a `when` renders its objects or nothing.
	elseActionRe = regexp.MustCompile(`\{\{-?\s*else\b`)
)

// templateGated reports whether a template chooses between two role models by platform version:
// it calls the helper rbacv2-migrate-module.sh writes, or tests deckhouseVersion in an action the
// declaration does not produce and renders something else otherwise. A `when` on a declared
// resource may test the version too -- today, or before the declaration dropped it -- and has no
// other branch.
func templateGated(existing, produced string) bool {
	if strings.Contains(existing, rbaccontract.GateMarker) {
		return true
	}

	return gateActionRe.MatchString(existing) && !gateActionRe.MatchString(produced) && elseActionRe.MatchString(existing)
}

// writeFileAtomic writes content to path through a temporary file in the same directory and a
// rename, so an interrupted --fix never leaves rbac.yaml or a template truncated.
func writeFileAtomic(path string, content []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}

	tmpName := tmp.Name()

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)

		return err
	}

	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}

	if err := os.Chmod(tmpName, perm); err != nil {
		_ = os.Remove(tmpName)
		return err
	}

	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}

	return nil
}

// recordRemovals adds what one render variant says a fix of the file takes away, so that the log
// of the fix names the removals of every variant, not only of the one whose closure runs.
func recordRemovals(file string, removals []string) {
	fixState.Lock()
	defer fixState.Unlock()

	known := fixState.removals[file]
	if known == nil {
		known = map[string]struct{}{}
		fixState.removals[file] = known
	}

	for _, r := range removals {
		known[r] = struct{}{}
	}
}

// recordedRemovals returns the union of the removals every variant recorded for the file, sorted.
func recordedRemovals(file string) []string {
	fixState.Lock()
	defer fixState.Unlock()

	out := make([]string, 0, len(fixState.removals[file]))
	for r := range fixState.removals[file] {
		out = append(out, r)
	}

	sort.Strings(out)

	return out
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// bootstrapPartialOf lists the objects that some render variants did not render, as
// Kind/namespace/name; empty without --matrix.
func bootstrapPartialOf(path string) []string {
	fixState.Lock()
	defer fixState.Unlock()

	var out []string

	for key, n := range fixState.seen[path] {
		if n < fixState.variants[path] {
			out = append(out, key)
		}
	}

	sort.Strings(out)

	return out
}

// bootstrapVariantsOf names, per object (Kind/namespace/name), the render variants that rendered
// it; empty without --matrix.
func bootstrapVariantsOf(path string) map[string]string {
	fixState.Lock()
	defer fixState.Unlock()

	if fixState.variants[path] < 2 {
		return nil
	}

	out := make(map[string]string, len(fixState.in[path]))
	for key, in := range fixState.in[path] {
		out[key] = in
	}

	return out
}
