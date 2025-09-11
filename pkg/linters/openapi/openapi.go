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
	"github.com/deckhouse/dmt/pkg/linters/openapi/rules"
)

// OpenAPI linter
type OpenAPI struct {
	name, desc string
	cfg        *config.OpenAPISettings
	ErrorList  *errors.LintRuleErrorsList
	moduleCfg  *config.ModuleConfig
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *OpenAPI {
	return &OpenAPI{
		name:      "openapi",
		desc:      "Linter will check openapi values is correct",
		cfg:       &cfg.LintersSettings.OpenAPI,
		ErrorList: errorList.WithLinterID("openapi").WithMaxLevel(cfg.LintersSettings.OpenAPI.Impact),
		moduleCfg: cfg,
	}
}

func (o *OpenAPI) GetRuleImpact(ruleName string) *pkg.Level {
	if o.moduleCfg != nil {
		return o.moduleCfg.LintersSettings.GetRuleImpact("openapi", ruleName)
	}
	return o.cfg.Impact
}

func (o *OpenAPI) Run(m *module.Module) {
	errorLists := o.ErrorList.WithModule(m.GetName())

	// check openAPI files
	openAPIFiles := fsutils.GetFiles(m.GetPath(), true, filterOpenAPIfiles)

	enumValidator := rules.NewEnumRule(o.cfg, m.GetPath())
	haValidator := rules.NewHARule(o.cfg, m.GetPath())

	// Apply rule-specific impact for enum and ha rules
	enumRuleImpact := o.GetRuleImpact("enum")
	haRuleImpact := o.GetRuleImpact("ha")

	for _, file := range openAPIFiles {
		if enumRuleImpact != nil {
			enumErrorList := errorLists.WithMaxLevel(enumRuleImpact)
			enumValidator.Run(file, enumErrorList)
		} else {
			enumValidator.Run(file, errorLists)
		}

		if haRuleImpact != nil {
			haErrorList := errorLists.WithMaxLevel(haRuleImpact)
			haValidator.Run(file, haErrorList)
		} else {
			haValidator.Run(file, errorLists)
		}
	}

	// check only CRDs files
	crdFiles := fsutils.GetFiles(m.GetPath(), true, filterCRDsfiles)
	crdValidator := rules.NewDeckhouseCRDsRule(o.cfg, m.GetPath())
	keyValidator := rules.NewKeysRule(o.cfg, m.GetPath())

	// Apply rule-specific impact for crd and keys rules
	crdRuleImpact := o.GetRuleImpact("crds")
	keysRuleImpact := o.GetRuleImpact("keys")

	for _, file := range crdFiles {
		if enumRuleImpact != nil {
			enumErrorList := errorLists.WithMaxLevel(enumRuleImpact)
			enumValidator.Run(file, enumErrorList)
		} else {
			enumValidator.Run(file, errorLists)
		}

		if haRuleImpact != nil {
			haErrorList := errorLists.WithMaxLevel(haRuleImpact)
			haValidator.Run(file, haErrorList)
		} else {
			haValidator.Run(file, errorLists)
		}

		if keysRuleImpact != nil {
			keysErrorList := errorLists.WithMaxLevel(keysRuleImpact)
			keyValidator.Run(file, keysErrorList)
		} else {
			keyValidator.Run(file, errorLists)
		}

		if crdRuleImpact != nil {
			crdErrorList := errorLists.WithMaxLevel(crdRuleImpact)
			crdValidator.Run(m.GetName(), file, crdErrorList)
		} else {
			crdValidator.Run(m.GetName(), file, errorLists)
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
