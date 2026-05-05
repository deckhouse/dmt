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
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/module"
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

func (o *OpenAPI) Run(m *module.Module) {
	errorLists := o.ErrorList.WithModule(m.GetName())

	// check openAPI files
	openAPIFiles := fsutils.GetFiles(m.GetPath(), true, filterOpenAPIfiles)

	enumValidator := rules.NewEnumRule(o.cfg, m.GetPath())
	haValidator := rules.NewHARule(o.cfg, m.GetPath())

	for _, file := range openAPIFiles {
		enumValidator.Run(file, errorLists)
		haValidator.Run(file, errorLists)
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

	// bilingual check: ensure doc-ru- translation files exist
	bilingualErrorList := errorLists.WithMaxLevel(o.cfg.Rules.BilingualRule.GetLevel())
	bilingualValidator := rules.NewBilingualRule(o.cfg, m.GetPath())

	// check openAPI files have translations (excluding values.yaml)
	for _, file := range openAPIFiles {
		if isValuesFile(file) {
			continue
		}
		bilingualValidator.Run(file, bilingualErrorList)
	}

	// check CRD files have translations
	for _, file := range crdFiles {
		bilingualValidator.Run(file, bilingualErrorList)
	}

	// check orphaned doc-ru- files (translation without base file)
	docRuOpenAPIFiles := fsutils.GetFiles(m.GetPath(), true, filterDocRuOpenAPIFiles)
	for _, file := range docRuOpenAPIFiles {
		bilingualValidator.Run(file, bilingualErrorList)
	}

	docRuCRDFiles := fsutils.GetFiles(m.GetPath(), true, filterDocRuCRDFiles)
	for _, file := range docRuCRDFiles {
		bilingualValidator.Run(file, bilingualErrorList)
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

func isValuesFile(path string) bool {
	filename := filepath.Base(path)
	return filename == "values.yaml" || filename == "values.yml"
}

func filterDocRuOpenAPIFiles(rootPath, path string) bool {
	relPath := fsutils.Rel(rootPath, path)
	filename := filepath.Base(relPath)

	if !strings.HasPrefix(filename, "doc-ru-") {
		return false
	}
	if strings.HasSuffix(filename, "-tests.yaml") {
		return false
	}

	return openapiYamlRegex.MatchString(relPath)
}

func filterDocRuCRDFiles(rootPath, path string) bool {
	relPath := fsutils.Rel(rootPath, path)
	filename := filepath.Base(relPath)

	if !strings.HasPrefix(filename, "doc-ru-") {
		return false
	}
	if strings.HasSuffix(filename, "-tests.yaml") {
		return false
	}

	return crdsYamlRegex.MatchString(relPath)
}
