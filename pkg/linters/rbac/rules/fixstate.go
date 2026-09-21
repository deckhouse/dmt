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
	"strings"
	"sync"

	"github.com/deckhouse/dmt/pkg/linters/rbac/rules/rbaccontract"
)

// fixState is what the autofixes of the coverage and sync rules share across render variants of
// one run. dmt lints a module once per variant under --matrix and every variant collects its own
// finding with its own closure; the state makes them behave as one fix per target (spec 005 R36):
//
//   - outcomes remembers the result of the first closure that ran for a target, so the others
//     return it instead of doing the work again, and every copy of the finding ends the run in
//     the same state;
//   - rendered accumulates, at lint time, the rights every variant's render grants in a file, so
//     the drop guard of the sync autofix judges the union rather than the render of whichever
//     variant happened to run its closure first (D3).
var fixState = struct {
	sync.Mutex
	outcomes map[string]error
	rendered map[string]map[string]struct{}
}{
	outcomes: map[string]error{},
	rendered: map[string]map[string]struct{}{},
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

// recordRenderedRights adds what one render variant grants in the file to what is known about it.
func recordRenderedRights(file string, rights map[string]struct{}) {
	fixState.Lock()
	defer fixState.Unlock()

	known := fixState.rendered[file]
	if known == nil {
		known = map[string]struct{}{}
		fixState.rendered[file] = known
	}

	for r := range rights {
		known[r] = struct{}{}
	}
}

// renderedRights returns everything any render variant granted in the file.
func renderedRights(file string) map[string]struct{} {
	fixState.Lock()
	defer fixState.Unlock()

	out := make(map[string]struct{}, len(fixState.rendered[file]))
	for r := range fixState.rendered[file] {
		out[r] = struct{}{}
	}

	return out
}

// resetFixState forgets everything; tests call it between runs.
func resetFixState() {
	fixState.Lock()
	defer fixState.Unlock()

	fixState.outcomes = map[string]error{}
	fixState.rendered = map[string]map[string]struct{}{}
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
