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
//   - foreign accumulates, at lint time, the objects every variant's render placed in a file that
//     the declaration does not produce, so the refusal to rewrite such a file judges the union
//     rather than the render of whichever variant happened to run its closure first.
var fixState = struct {
	sync.Mutex
	outcomes  map[string]error
	foreign   map[string]map[string]struct{}
	bootstrap map[string]map[string]bootstrap.Object
}{
	outcomes:  map[string]error{},
	foreign:   map[string]map[string]struct{}{},
	bootstrap: map[string]map[string]bootstrap.Object{},
}

// fixOnce runs fix for the key the first time it is asked and returns that outcome on every later
// call. Fixes run one at a time (Manager.ApplyFixes is sequential), and the fix itself reads the
// state, so it runs outside the lock.
func fixOnce(key string, fix func() error) error {
	fixState.Lock()
	err, done := fixState.outcomes[key]
	fixState.Unlock()

	if done {
		return err
	}

	err = fix()

	fixState.Lock()
	fixState.outcomes[key] = err
	fixState.Unlock()

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

	fixState.outcomes = map[string]error{}
	fixState.foreign = map[string]map[string]struct{}{}
	fixState.bootstrap = map[string]map[string]bootstrap.Object{}
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
		return "ee/modules"
	case n >= 4 && parts[n-4] == "ee":
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

	return strings.Contains(string(content), rbaccontract.GateMarker)
}
