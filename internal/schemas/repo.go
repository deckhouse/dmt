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

package schemas

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const crdsDirName = "crds"

// repoCRDCache memoizes the compiled CRD overlay per repository root. The walk
// over every crds/ directory in the deckhouse tree is expensive, and the
// schema-validation rule runs for every module and every matrix variant, so the
// overlay is built exactly once per root and shared (read-only) thereafter.
var (
	repoCRDMu    sync.Mutex
	repoCRDCache = map[string]*repoCRDEntry{}
)

type repoCRDEntry struct {
	once    sync.Once
	schemas map[string]*jsonschema.Schema
}

// LoadRepoCRDs walks root for every CustomResourceDefinition shipped anywhere in
// the deckhouse repository (all crds/ directories) and returns their compiled
// schemas keyed by lookup key. These are authoritative: deckhouse runs the CRD
// versions it ships, so they must win over the bundled third-party catalog,
// which is only a lagging upstream snapshot. The result is memoized per root and
// safe for concurrent callers. A empty root (module linted outside a deckhouse
// checkout) yields nil, so callers transparently fall back to the catalog.
func LoadRepoCRDs(root string) map[string]*jsonschema.Schema {
	if root == "" {
		return nil
	}

	repoCRDMu.Lock()
	entry, ok := repoCRDCache[root]
	if !ok {
		entry = &repoCRDEntry{}
		repoCRDCache[root] = entry
	}
	repoCRDMu.Unlock()

	entry.once.Do(func() {
		entry.schemas = buildRepoCRDs(root)
	})

	return entry.schemas
}

// buildRepoCRDs scans every crds/ directory under root and compiles the CRDs it
// finds. It is best-effort: a CRD that fails to parse or compile is skipped
// (it will be reported through its own module's LoadModuleCRDs when that module
// is linted), and the first definition of a given GVK wins for determinism.
func buildRepoCRDs(root string) map[string]*jsonschema.Schema {
	out := map[string]*jsonschema.Schema{}

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // tolerate unreadable subtrees
		}

		if !d.IsDir() || d.Name() != crdsDirName {
			return nil
		}

		// Compile the whole crds/ subtree at once (deckhouse nests CRDs in
		// subdirectories, e.g. crds/cert-manager/), then skip re-descending it.
		mergeCRDTree(path, out)

		return filepath.SkipDir
	})

	return out
}

// mergeCRDTree compiles every CRD document found anywhere under a crds/ subtree
// into out, recursing into nested directories.
func mergeCRDTree(dir string, out map[string]*jsonschema.Schema) {
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		name := d.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		for _, doc := range splitYAML(string(content)) {
			// Notes are intentionally discarded here: a CRD's null-keyword
			// warnings belong to the lint of its own module, not to every module
			// that merely references the kind.
			compiled, _, cerr := compileCRDDoc([]byte(doc))
			if cerr != nil {
				continue
			}

			for key, sch := range compiled {
				if _, exists := out[key]; !exists {
					out[key] = sch
				}
			}
		}

		return nil
	})
}

// DeckhouseRoot walks up from dir to the deckhouse repository root, identified
// by the same markers the manager uses to locate global values. It returns ""
// when dir is not inside a deckhouse checkout (e.g. a standalone module), in
// which case there is no repository CRD overlay to load.
func DeckhouseRoot(dir string) string {
	for dir != "" {
		if isDir(filepath.Join(dir, "global-hooks", "openapi")) &&
			isDir(filepath.Join(dir, "modules")) &&
			isFile(filepath.Join(dir, "global-hooks", "openapi", "config-values.yaml")) &&
			isFile(filepath.Join(dir, "global-hooks", "openapi", "values.yaml")) {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return ""
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
