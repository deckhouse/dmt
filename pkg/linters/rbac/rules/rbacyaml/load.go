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

package rbacyaml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// ErrNotFound is returned by Load when the module has no rbac.yaml. The coverage and sync rules
// treat it as "nothing to check"; only the contract rule runs on such a module.
var ErrNotFound = errors.New("rbac.yaml not found")

// Path returns the declaration's path for a module directory.
func Path(modulePath string) string {
	return filepath.Join(modulePath, Filename)
}

// Load reads and parses modules/<module>/rbac.yaml. Unknown keys are an error: a misspelled
// key would otherwise silently drop the rights it was meant to grant. Parsing errors are
// returned as one error; the semantic checks are Validate's.
func Load(modulePath string) (*Declaration, error) {
	data, err := os.ReadFile(Path(modulePath))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("read %s: %w", Filename, err)
	}

	return Parse(data)
}

// Parse parses the declaration from its bytes and normalizes it (see Normalize).
func Parse(data []byte) (*Declaration, error) {
	decl := new(Declaration)

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	if err := decoder.Decode(decl); err != nil {
		return nil, fmt.Errorf("parse %s: %w", Filename, err)
	}

	// A second document in the file would be silently ignored otherwise.
	switch err := decoder.Decode(new(Declaration)); {
	case err == nil:
		return nil, fmt.Errorf("parse %s: the file must hold a single YAML document", Filename)
	case !errors.Is(err, io.EOF):
		return nil, fmt.Errorf("parse %s: %w", Filename, err)
	}

	for i := range decl.Resources {
		decl.Resources[i].Position = i
	}

	decl.parsed = true

	decl.Normalize()

	return decl, nil
}

// Normalize puts the declaration into its canonical order, so that the generator's output and
// the sync comparison do not depend on how the author ordered the file: resources by group and
// resource, verbs sorted, subsystems sorted. Duplicates are left in place for Validate to
// report.
func (d *Declaration) Normalize() {
	sort.SliceStable(d.Resources, func(i, j int) bool {
		if d.Resources[i].Group != d.Resources[j].Group {
			return d.Resources[i].Group < d.Resources[j].Group
		}

		return d.Resources[i].Resource < d.Resources[j].Resource
	})

	for i := range d.Resources {
		for _, levels := range []map[string][]string{d.Resources[i].Namespace, d.Resources[i].System, d.Resources[i].Legacy} {
			for level := range levels {
				sort.Strings(levels[level])
			}
		}
	}

	sort.Strings(d.Subsystems)
}
