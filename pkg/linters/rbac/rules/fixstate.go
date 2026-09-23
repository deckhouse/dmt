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
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/bootstrap"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/generate"
	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbaccontract"
)

// fixState is what the autofixes of the coverage and sync rules share across render variants of
// one run. dmt lints a module once per variant under --matrix and every variant collects its own
// finding with its own closure; the state makes them behave as one fix per target (spec 005 R36):
//
//   - outcomes remembers the result of the first closure that ran for a target, so the others
//     return it instead of doing the work again, and every copy of the finding ends the run in
//     the same state;
//   - foreign accumulates, at lint time, the objects every variant's render placed in a file that
//     the declaration does not produce, so the refusal to rewrite such a file judges the union
//     rather than the render of whichever variant happened to run its closure first.
var fixState = struct {
	sync.Mutex
	foreign   map[string]map[string]struct{}
	removals  map[string]map[string]struct{}
	moves     map[string]map[string]string
	dropped   map[string]string
	bootstrap map[string]map[string]bootstrap.Object
}{
	foreign:   map[string]map[string]struct{}{},
	removals:  map[string]map[string]struct{}{},
	moves:     map[string]map[string]string{},
	dropped:   map[string]string{},
	bootstrap: map[string]map[string]bootstrap.Object{},
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

// recordForeignObjects adds the objects one render variant placed in the file that the declaration
// does not produce.
func recordForeignObjects(file string, objects []string) {
	fixState.Lock()
	defer fixState.Unlock()

	known := fixState.foreign[file]
	if known == nil {
		known = map[string]struct{}{}
		fixState.foreign[file] = known
	}

	for _, o := range objects {
		known[o] = struct{}{}
	}
}

// foreignObjectsOf returns, sorted, every object any render variant placed in the file that the
// declaration does not produce.
func foreignObjectsOf(file string) []string {
	fixState.Lock()
	defer fixState.Unlock()

	out := make([]string, 0, len(fixState.foreign[file]))
	for o := range fixState.foreign[file] {
		out = append(out, o)
	}

	sort.Strings(out)

	return out
}

// resetFixState forgets everything; tests call it between runs.
func resetFixState() {
	fixState.Lock()
	defer fixState.Unlock()

	fixState.foreign = map[string]map[string]struct{}{}
	fixState.removals = map[string]map[string]struct{}{}
	fixState.moves = map[string]map[string]string{}
	fixState.dropped = map[string]string{}
	fixState.bootstrap = map[string]map[string]bootstrap.Object{}

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

	for _, o := range objects {
		known[o.Kind+"/"+o.Namespace+"/"+o.Name] = o
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

// gateActionRe matches a template action that tests the platform version. A mention of the word
// in a comment or a value does not count.
var gateActionRe = regexp.MustCompile(`\{\{[^}]*deckhouseVersion`)

// templateGated reports whether a template chooses between two role models by platform version:
// it calls the helper rbacv2-migrate-module.sh writes, or tests deckhouseVersion in an action that
// the declaration itself did not produce. A `when` on a declared resource may test the version too;
// that action appears in the produced content as well and is not a gate.
func templateGated(existing, produced string) bool {
	if strings.Contains(existing, rbaccontract.GateMarker) {
		return true
	}

	return gateActionRe.MatchString(existing) && !gateActionRe.MatchString(produced)
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

// recordMoves adds the objects one render variant saw in the file that the declaration now puts in
// another file, with that file's full path.
func recordMoves(file string, moves map[string]string) {
	if len(moves) == 0 {
		return
	}

	fixState.Lock()
	defer fixState.Unlock()

	known := fixState.moves[file]
	if known == nil {
		known = map[string]string{}
		fixState.moves[file] = known
	}

	for id, target := range moves {
		known[id] = target
	}
}

// unfinishedMoves returns, sorted, the objects moving out of the file whose target does not hold
// them yet: its header does not list them. Such an object must not leave the file.
func unfinishedMoves(file, modulePath string) []string {
	fixState.Lock()
	moves := fixState.moves[file]
	fixState.Unlock()

	var out []string

	for id, target := range moves {
		content, err := os.ReadFile(filepath.Join(modulePath, target))
		if err == nil {
			if owned, _ := generate.ParseOwned(string(content)); owned != nil {
				if _, there := owned[id]; there {
					continue
				}
			}
		}

		out = append(out, id+" (to "+target+")")
	}

	sort.Strings(out)

	return out
}

// recordDropped marks a template the render skipped in some variant: no variant's fix may rewrite
// or delete it, since the objects that variant renders there were never seen.
func recordDropped(file, cause string) {
	fixState.Lock()
	defer fixState.Unlock()

	fixState.dropped[file] = cause
}

// droppedCause returns why a variant skipped the template, if one did.
func droppedCause(file string) (string, bool) {
	fixState.Lock()
	defer fixState.Unlock()

	cause, ok := fixState.dropped[file]

	return cause, ok
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
