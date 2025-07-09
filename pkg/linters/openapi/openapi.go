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

func New(cfg *config.ModuleConfig, tracker *exclusions.ExclusionTracker, errorList *errors.LintRuleErrorsList) *OpenAPI {
	return &OpenAPI{
		name:      "openapi",
		desc:      "Linter will check openapi values is correct",
		cfg:       &cfg.LintersSettings.OpenAPI,
		ErrorList: errorList.WithLinterID("openapi").WithMaxLevel(cfg.LintersSettings.OpenAPI.Impact),
		tracker:   tracker,
	}
}

func (o *OpenAPI) Run(m *module.Module) {
	errorLists := o.ErrorList.WithModule(m.GetName())
	o.run(m, m.GetName(), errorLists)
}

func (o *OpenAPI) run(m *module.Module, moduleName string, errorList *errors.LintRuleErrorsList) {
	if o.tracker != nil {
		// With tracking
		// CRDs
		trackedCRDsRule := exclusions.NewTrackedRule(
			pkg.NewStringRuleWithTracker(o.cfg.OpenAPIExcludeRules.CRDNamesExcludes.Get(), o.tracker, "openapi", "crds"),
			exclusions.StringRuleKeys(o.cfg.OpenAPIExcludeRules.CRDNamesExcludes.Get()),
			o.tracker, "openapi", "crds", moduleName,
		)
		crdsRule := rules.NewDeckhouseCRDsRuleTracked(o.cfg, m.GetPath(), trackedCRDsRule)

		// HA
		trackedHARule := exclusions.NewTrackedRule(
			pkg.NewStringRuleWithTracker(o.cfg.OpenAPIExcludeRules.HAAbsoluteKeysExcludes.Get(), o.tracker, "openapi", "ha"),
			exclusions.StringRuleKeys(o.cfg.OpenAPIExcludeRules.HAAbsoluteKeysExcludes.Get()),
			o.tracker, "openapi", "ha", moduleName,
		)
		haRule := rules.NewHARuleTracked(o.cfg, m.GetPath(), trackedHARule)

		// Keys
		keyBannedNames := make([]pkg.StringRuleExclude, len(o.cfg.OpenAPIExcludeRules.KeyBannedNames))
		for i, name := range o.cfg.OpenAPIExcludeRules.KeyBannedNames {
			keyBannedNames[i] = pkg.StringRuleExclude(name)
		}
		trackedKeysRule := exclusions.NewTrackedRule(
			pkg.NewStringRuleWithTracker(keyBannedNames, o.tracker, "openapi", "keys"),
			exclusions.StringRuleKeys(keyBannedNames),
			o.tracker, "openapi", "keys", moduleName,
		)
		keysRule := rules.NewKeysRuleTracked(o.cfg, m.GetPath(), trackedKeysRule)

		// Enum
		enumFileExcludes := make([]pkg.StringRuleExclude, len(o.cfg.OpenAPIExcludeRules.EnumFileExcludes))
		for i, exclude := range o.cfg.OpenAPIExcludeRules.EnumFileExcludes {
			enumFileExcludes[i] = pkg.StringRuleExclude(exclude)
		}
		trackedEnumRule := exclusions.NewTrackedRule(
			pkg.NewStringRuleWithTracker(enumFileExcludes, o.tracker, "openapi", "enum"),
			exclusions.StringRuleKeys(enumFileExcludes),
			o.tracker, "openapi", "enum", moduleName,
		)
		enumRule := rules.NewEnumRuleTracked(o.cfg, m.GetPath(), trackedEnumRule)

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
			crdsRule.Run(moduleName, file, errorList)
		}
	} else {
		// Without tracking
		// check openAPI files
		openAPIFiles := fsutils.GetFiles(m.GetPath(), true, filterOpenAPIfiles)

		enumValidator := rules.NewEnumRule(o.cfg, m.GetPath())
		haValidator := rules.NewHARule(o.cfg, m.GetPath())

		for _, file := range openAPIFiles {
			enumValidator.Run(file, errorList)
			haValidator.Run(file, errorList)
		}

		// check only CRDs files
		crdFiles := fsutils.GetFiles(m.GetPath(), true, filterCRDsfiles)
		crdValidator := rules.NewDeckhouseCRDsRule(o.cfg, m.GetPath())
		keyValidator := rules.NewKeysRule(o.cfg, m.GetPath())
		for _, file := range crdFiles {
			enumValidator.Run(file, errorList)
			haValidator.Run(file, errorList)
			keyValidator.Run(file, errorList)
			crdValidator.Run(m.GetName(), file, errorList)
		}
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
