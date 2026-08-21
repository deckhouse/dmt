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

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const DocRuYAMLRuleName = "doc-ru-yaml"

type DocRuYAMLRule struct {
	pkg.RuleMeta
	rootPath string
}

func NewDocRuYAMLRule(_ *pkg.OpenAPILinterConfig, rootPath string) *DocRuYAMLRule {
	return &DocRuYAMLRule{
		RuleMeta: pkg.RuleMeta{
			Name: DocRuYAMLRuleName,
		},
		rootPath: rootPath,
	}
}

// Run parses a doc-ru- translation file to verify it is syntactically valid YAML.
//
// Rationale: doc-ru- files are documentation-only and are explicitly excluded
// from the enum/ha/keys/deckhouse-crds parsing, so their YAML syntax is checked
// nowhere else. A broken doc-ru- file therefore slips through module linting and
// only surfaces later, when the documentation site is built from the released
// module, as an opaque build failure. This rule shifts that detection left: a
// syntactically broken translation file is reported here, with its path, before
// the module is released.
//
// The file is decoded as a YAML mapping, exactly like the base file's parser
// (internal/openapi getFileYAMLContent): a syntactically broken file — or one
// that is valid YAML but not a mapping, e.g. a bare scalar or a sequence between
// the delimiters — is reported. Deeper structural/semantic expectations still
// belong to the base file's own rules.
func (r *DocRuYAMLRule) Run(path string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	shortPath := fsutils.Rel(r.rootPath, path)

	data, err := os.ReadFile(path)
	if err != nil {
		errorList.WithFilePath(shortPath).Errorf("cannot read doc-ru file: %s", err)

		return
	}

	// Decode into a map, like the base openapi parser: this rejects both a syntax
	// error and valid-YAML-that-is-not-a-mapping (a scalar or sequence), which the
	// documentation build cannot consume as a doc-ru- overlay.
	m := make(map[string]any)
	if err := yaml.UnmarshalStrict(data, &m); err != nil {
		errorList.WithFilePath(shortPath).Errorf("doc-ru file is not valid YAML:\n%s", err)
	}
}
