/*
Copyright 2025 Flant JSC

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

package openapi

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/modules"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/openapi/rules"
)

// OpenAPI linter
type OpenAPI struct {
	name, desc string
	cfg        *pkg.OpenAPILinterConfig
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *pkg.OpenAPILinterConfig, errorList *errors.LintRuleErrorsList) *OpenAPI {
	return &OpenAPI{
		name:      "openapi",
		desc:      "Linter will check openapi values is correct",
		cfg:       cfg,
		ErrorList: errorList.WithLinterID("openapi").WithMaxLevel(cfg.Impact),
	}
}

func (o *OpenAPI) Run(m *modules.Module) {
	errorLists := o.ErrorList.WithModule(m.GetName())

	// check openAPI files
	openAPIFiles := fsutils.GetFiles(m.GetPath(), true, filterOpenAPIfiles)

	enumValidator := rules.NewEnumRule(o.cfg, m.GetPath())
	haValidator := rules.NewHARule(o.cfg, m.GetPath())

	for _, file := range openAPIFiles {
		enumValidator.Run(file, errorLists)
		haValidator.Run(file, errorLists)
	}

	// x-deckhouse-validations / x-kubernetes-validations check: these CEL-validation
	// extensions are consumed only by deckhouse-controller (config-values/values),
	// so a malformed block is caught nowhere else in the openapi rules. Scoped to
	// openapi/ files only — in crds/, x-kubernetes-validations is a legitimate,
	// API-server-honored construct and must not be flagged.
	validationsErrorList := errorLists.WithMaxLevel(o.cfg.Rules.DeckhouseValidationsRule.GetLevel())
	validationsValidator := rules.NewDeckhouseValidationsRule(o.cfg, m.GetPath())

	for _, file := range openAPIFiles {
		validationsValidator.Run(file, validationsErrorList)
	}

	// check only CRDs files
	crdFiles := fsutils.GetFiles(m.GetPath(), true, filterCRDsfiles)
	crdValidator := rules.NewDeckhouseCRDsRule(o.cfg, m.GetPath())
	keyValidator := rules.NewKeysRule(o.cfg, m.GetPath())

	for _, file := range crdFiles {
		enumValidator.Run(file, errorLists)
		haValidator.Run(file, errorLists)
		keyValidator.Run(file, errorLists)
		crdValidator.Run(m.GetName(), file, errorLists)
	}

	// bilingual check: ensure top-level crds/ files have doc-ru- translation
	bilingualErrorList := errorLists.WithMaxLevel(o.cfg.Rules.BilingualRule.GetLevel())
	bilingualValidator := rules.NewBilingualRule(o.cfg, m.GetPath())

	for _, file := range bilingualCRDFiles(m.GetPath()) {
		bilingualValidator.Run(file, bilingualErrorList)
	}

	// doc-ru- YAML syntax check: doc-ru- files are excluded from the enum/ha/keys
	// parsing above, so this is the only place their YAML syntax is validated.
	docRuErrorList := errorLists.WithMaxLevel(o.cfg.Rules.DocRuYAMLRule.GetLevel())
	docRuValidator := rules.NewDocRuYAMLRule(o.cfg, m.GetPath())

	for _, file := range fsutils.GetFiles(m.GetPath(), true, filterDocRuYAMLfiles) {
		docRuValidator.Run(file, docRuErrorList)
	}
}

func (o *OpenAPI) Name() string {
	return o.name
}

func (o *OpenAPI) Desc() string {
	return o.desc
}

var openapiYamlRegex = regexp.MustCompile(`^openapi/.*\.ya?ml$`)

func filterOpenAPIfiles(rootPath, path string) bool {
	path = fsutils.Rel(rootPath, path)

	filename := filepath.Base(path)
	if strings.HasSuffix(filename, "-tests.yaml") {
		return false
	}

	if strings.HasPrefix(filename, "doc-ru-") {
		return false
	}

	return openapiYamlRegex.MatchString(path)
}

var crdsYamlRegex = regexp.MustCompile(`^crds/.*\.ya?ml$`)

func filterCRDsfiles(rootPath, path string) bool {
	path = fsutils.Rel(rootPath, path)

	filename := filepath.Base(path)
	if strings.HasSuffix(filename, "-tests.yaml") {
		return false
	}

	if strings.HasPrefix(filename, "doc-ru-") {
		return false
	}

	return crdsYamlRegex.MatchString(path)
}

var docRuYamlRegex = regexp.MustCompile(`^(openapi|crds)/doc-ru-.*\.ya?ml$`)

// filterDocRuYAMLfiles selects doc-ru- translation files under openapi/ and crds/.
// These are the files the openapi/crds parsing above deliberately skips, so their
// YAML syntax is validated by the doc-ru-yaml rule instead.
func filterDocRuYAMLfiles(rootPath, path string) bool {
	path = fsutils.Rel(rootPath, path)

	if strings.HasSuffix(filepath.Base(path), "-tests.yaml") {
		return false
	}

	return docRuYamlRegex.MatchString(path)
}

func bilingualCRDFiles(rootPath string) []string {
	crdsPath := filepath.Join(rootPath, "crds")

	entries, err := os.ReadDir(crdsPath)
	if err != nil {
		return nil
	}

	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}

		filename := entry.Name()
		if !strings.HasSuffix(filename, ".yaml") && !strings.HasSuffix(filename, ".yml") {
			continue
		}

		if strings.HasSuffix(filename, "-tests.yaml") {
			continue
		}

		result = append(result, filepath.Join(crdsPath, filename))
	}

	return result
}
