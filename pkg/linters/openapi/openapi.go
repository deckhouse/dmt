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
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
	"github.com/deckhouse/dmt/pkg/linters/openapi/rules"
)

// OpenAPI linter
type OpenAPI struct {
	name, desc string
	cfg        *config.OpenAPISettings
	ErrorList  *errors.LintRuleErrorsList
	tracker    *exclusions.ExclusionTracker
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *OpenAPI {
	return &OpenAPI{
		name:      "openapi",
		desc:      "Linter will check openapi values is correct",
		cfg:       &cfg.LintersSettings.OpenAPI,
		ErrorList: errorList.WithLinterID("openapi").WithMaxLevel(cfg.LintersSettings.OpenAPI.Impact),
	}
}

func NewWithTracker(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList, tracker *exclusions.ExclusionTracker) *OpenAPI {
	return &OpenAPI{
		name:      "openapi",
		desc:      "Linter will check openapi values is correct (with exclusion tracking)",
		cfg:       &cfg.LintersSettings.OpenAPI,
		ErrorList: errorList.WithLinterID("openapi").WithMaxLevel(cfg.LintersSettings.OpenAPI.Impact),
		tracker:   tracker,
	}
}

func (o *OpenAPI) Run(m *module.Module) {
	errorLists := o.ErrorList.WithModule(m.GetName())

	if o.tracker != nil {
		o.runWithTracking(m, errorLists)
	} else {
		o.runWithoutTracking(m, errorLists)
	}
}

func (o *OpenAPI) runWithoutTracking(m *module.Module, errorLists *errors.LintRuleErrorsList) {
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
}

func (o *OpenAPI) runWithTracking(m *module.Module, errorList *errors.LintRuleErrorsList) {
	// CRDs
	trackedCRDsRule := exclusions.NewTrackedStringRule(
		o.cfg.OpenAPIExcludeRules.CRDNamesExcludes.Get(),
		o.tracker,
		"openapi",
		"crd-names",
	)
	crdsRule := rules.NewDeckhouseCRDsRuleTracked(o.cfg, m.GetPath(), trackedCRDsRule)

	// HA
	trackedHARule := exclusions.NewTrackedStringRule(
		o.cfg.OpenAPIExcludeRules.HAAbsoluteKeysExcludes.Get(),
		o.tracker,
		"openapi",
		"ha-absolute-keys",
	)
	haRule := rules.NewHARuleTracked(o.cfg, m.GetPath(), trackedHARule)

	// Keys
	keysRule := rules.NewKeysRule(o.cfg, m.GetPath())

	// Enum
	enumRule := rules.NewEnumRule(o.cfg, m.GetPath())

	// Run rules
	openAPIFiles := fsutils.GetFiles(m.GetPath(), true, filterOpenAPIfiles)
	for _, file := range openAPIFiles {
		enumRule.Run(file, errorList)
		haRule.Run(file, errorList)
	}

	// check only CRDs files
	crdFiles := fsutils.GetFiles(m.GetPath(), true, filterCRDsfiles)
	for _, file := range crdFiles {
		enumRule.Run(file, errorList)
		haRule.Run(file, errorList)
		keysRule.Run(file, errorList)
		crdsRule.Run(m.GetName(), file, errorList)
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
