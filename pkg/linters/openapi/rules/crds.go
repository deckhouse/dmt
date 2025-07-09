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

package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"sigs.k8s.io/yaml"
)

type DeckhouseCRDsRule struct {
	pkg.RuleMeta
	pkg.StringRule
	rootPath string
}

const CrdsDir = "crds"

var (
	sep = regexp.MustCompile("(?:^|\\s*\n)---\\s*")
)

func NewDeckhouseCRDsRule(cfg *config.OpenAPISettings, rootPath string) *DeckhouseCRDsRule {
	return &DeckhouseCRDsRule{
		RuleMeta: pkg.RuleMeta{
			Name: "deckhouse-crds",
		},
		StringRule: pkg.StringRule{
			ExcludeRules: cfg.OpenAPIExcludeRules.CRDNamesExcludes.Get(),
		},
		rootPath: rootPath,
	}
}

func NewDeckhouseCRDsRuleTracked(_ *config.OpenAPISettings, rootPath string, trackedRule *pkg.StringRule) *DeckhouseCRDsRule {
	return &DeckhouseCRDsRule{
		RuleMeta: pkg.RuleMeta{
			Name: "deckhouse-crds",
		},
		StringRule: *trackedRule,
		rootPath:   rootPath,
	}
}

func (*DeckhouseCRDsRule) validateLabel(crd *v1beta1.CustomResourceDefinition, labelName, expectedValue, shortPath string, errorList *errors.LintRuleErrorsList) {
	if value, ok := crd.Labels[labelName]; ok {
		if value != expectedValue {
			errorList.WithObjectID(fmt.Sprintf("kind = %s ; name = %s", crd.Kind, crd.Name)).
				WithFilePath(shortPath).
				WithValue(fmt.Sprintf("%s = %s", labelName, value)).
				Errorf(`CRD should contain "%s = %s" label, but got "%s = %s"`, labelName, expectedValue, labelName, value)
		}
	} else {
		errorList.WithObjectID(fmt.Sprintf("kind = %s ; name = %s", crd.Kind, crd.Name)).
			WithFilePath(shortPath).
			WithValue(fmt.Sprintf("%s = %s", labelName, expectedValue)).
			Errorf(`CRD should contain "%s = %s" label`, labelName, expectedValue)
	}
}

func (*DeckhouseCRDsRule) validateDeprecatedKeyInYAML(yamlDoc string, crd *v1beta1.CustomResourceDefinition, shortPath string, errorList *errors.LintRuleErrorsList) {
	// Parse YAML as map to search for deprecated key
	var yamlMap map[string]any
	if err := yaml.Unmarshal([]byte(yamlDoc), &yamlMap); err != nil {
		return
	}

	// Search for deprecated key in the YAML structure
	checkMapForDeprecated(yamlMap, shortPath, crd.Kind, crd.Name, errorList)
}

func checkMapForDeprecated(data any, shortPath, kind, name string, errorList *errors.LintRuleErrorsList) {
	switch v := data.(type) {
	case map[string]any:
		// Check if current map has deprecated key (regardless of value)
		if _, hasDeprecated := v["deprecated"]; hasDeprecated {
			errorList.WithObjectID(fmt.Sprintf("kind = %s ; name = %s", kind, name)).
				WithFilePath(shortPath).
				WithValue("deprecated: present").
				Errorf(`CRD contains "deprecated" key, use "x-doc-deprecated: true" instead`)
		}

		// Recursively check all values in the map
		for _, value := range v {
			checkMapForDeprecated(value, shortPath, kind, name, errorList)
		}
	case []any:
		// Recursively check all items in the slice
		for _, item := range v {
			checkMapForDeprecated(item, shortPath, kind, name, errorList)
		}
	}
}

func (r *DeckhouseCRDsRule) Run(moduleName, path string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	shortPath, _ := filepath.Rel(r.rootPath, path)
	fileContent, err := os.ReadFile(path)
	if err != nil {
		errorList.Errorf("Can't read file %s: %s", shortPath, err)
		return
	}

	docs := splitManifests(string(fileContent))
	for _, d := range docs {
		var crd v1beta1.CustomResourceDefinition

		if err := yaml.Unmarshal([]byte(d), &crd); err != nil {
			errorList.Errorf("Can't parse manifests in %s folder: %s", CrdsDir, err)
			continue
		}

		if !strings.Contains(crd.Name, "deckhouse.io") {
			continue
		}

		if crd.APIVersion != "apiextensions.k8s.io/v1" {
			errorList.WithObjectID(fmt.Sprintf("kind = %s ; name = %s", crd.Kind, crd.Name)).
				WithFilePath(shortPath).
				WithValue(crd.APIVersion).
				Errorf(`CRD specified using deprecated api version, wanted "apiextensions.k8s.io/v1"`)
		}

		if !r.Enabled(crd.Name) {
			continue
		}

		r.validateLabel(&crd, "module", moduleName, shortPath, errorList)
		r.validateDeprecatedKeyInYAML(d, &crd, shortPath, errorList)
	}
}

func splitManifests(bigFile string) []string {
	bigFileTmp := strings.TrimSpace(bigFile)
	return sep.Split(bigFileTmp, -1)
}
