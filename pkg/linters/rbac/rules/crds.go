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

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/fsutils"
)

// crdInfo is what the rbac rules need to know about a CRD the module ships: its identity and
// scope, and the file it came from for the finding.
type crdInfo struct {
	Group  string
	Plural string
	Scope  string
	// File is the path relative to the module root.
	File string
}

// Key returns "group/plural", the identity an rbac.yaml entry is matched by.
func (c crdInfo) Key() string { return c.Group + "/" + c.Plural }

// crdsYamlRegex selects the files of the module's crds/ directory, at any depth: cert-manager
// keeps its CRDs in crds/cert-manager/, operator-trivy in crds/native/. The anchor keeps
// images/**/testdata/crds/ and other look-alikes out; a file is a CRD by its kind, not by its
// directory (spec 005 R8).
var crdsYamlRegex = regexp.MustCompile(`^crds/.*\.ya?ml$`)

func filterCRDFiles(rootPath, path string) bool {
	path = fsutils.Rel(rootPath, path)

	filename := filepath.Base(path)
	if strings.HasSuffix(filename, "-tests.yaml") || strings.HasPrefix(filename, "doc-ru-") {
		return false
	}

	return crdsYamlRegex.MatchString(path)
}

// crdDocument is the part of a CustomResourceDefinition the rules read.
type crdDocument struct {
	Kind string `json:"kind"`
	Spec struct {
		Group string `json:"group"`
		Names struct {
			Plural string `json:"plural"`
		} `json:"names"`
		Scope string `json:"scope"`
	} `json:"spec"`
}

// moduleCRDs returns the CRDs the module ships under crds/, sorted by group and plural.
// Documents that are not a CustomResourceDefinition are skipped: a crds/ directory may hold a
// README or other manifests. A file that does not parse is an error: a CRD the rule cannot read
// is a CRD whose access nobody decided on.
func moduleCRDs(modulePath string) ([]crdInfo, error) {
	var out []crdInfo

	for _, file := range fsutils.GetFiles(modulePath, true, filterCRDFiles) {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", fsutils.Rel(modulePath, file), err)
		}

		for _, doc := range fsutils.SplitManifests(string(data)) {
			if strings.TrimSpace(doc) == "" {
				continue
			}

			var crd crdDocument
			if err := yaml.Unmarshal([]byte(doc), &crd); err != nil {
				return nil, fmt.Errorf("parse %s: %w", fsutils.Rel(modulePath, file), err)
			}

			if crd.Kind != "CustomResourceDefinition" {
				continue
			}

			out = append(out, crdInfo{
				Group:  crd.Spec.Group,
				Plural: crd.Spec.Names.Plural,
				Scope:  crd.Spec.Scope,
				File:   fsutils.Rel(modulePath, file),
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Group != out[j].Group {
			return out[i].Group < out[j].Group
		}

		if out[i].Plural != out[j].Plural {
			return out[i].Plural < out[j].Plural
		}

		return out[i].File < out[j].File
	})

	return out, nil
}
