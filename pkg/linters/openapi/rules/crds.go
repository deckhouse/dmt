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

func (*DeckhouseCRDsRule) validateLabel(crd *v1beta1.CustomResourceDefinition, labelName, expectedValue string, errorList *errors.LintRuleErrorsList, shortPath string) {
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

func (*DeckhouseCRDsRule) validateDeprecatedKeyInYAML(yamlDoc string, crd *v1beta1.CustomResourceDefinition, errorList *errors.LintRuleErrorsList, shortPath string) {
	// Parse YAML as map to search for deprecated key in properties only
	var yamlMap map[string]any
	if err := yaml.Unmarshal([]byte(yamlDoc), &yamlMap); err != nil {
		return
	}

	// Check for deprecated key only in properties section
	checkPropertiesForDeprecated(yamlMap, errorList, shortPath, crd.Kind, crd.Name)
}

func checkPropertiesForDeprecated(data any, errorList *errors.LintRuleErrorsList, shortPath, kind, name string) {
	if v, ok := data.(map[string]any); ok {
		// Navigate to spec.versions[].schema.openAPIV3Schema.properties
		if spec, ok := v["spec"].(map[string]any); ok {
			if versions, ok := spec["versions"].([]any); ok {
				for _, version := range versions {
					if versionMap, ok := version.(map[string]any); ok {
						if schema, ok := versionMap["schema"].(map[string]any); ok {
							if openAPIV3Schema, ok := schema["openAPIV3Schema"].(map[string]any); ok {
								// Check properties recursively for deprecated key
								checkMapPropertiesForDeprecated(openAPIV3Schema, errorList, shortPath, kind, name)
							}
						}
					}
				}
			}
		}
	}
}

func checkMapPropertiesForDeprecated(data any, errorList *errors.LintRuleErrorsList, shortPath, kind, name string) {
	switch v := data.(type) {
	case map[string]any:
		// Check if current map has deprecated key (regardless of value)
		if _, hasDeprecated := v["deprecated"]; hasDeprecated {
			errorList.WithObjectID(fmt.Sprintf("kind = %s ; name = %s", kind, name)).
				WithFilePath(shortPath).
				WithValue("deprecated: present").
				Errorf(`CRD contains "deprecated" key, use "x-doc-deprecated: true" instead`)
		}

		// Recursively check all values in the map, especially properties
		for key, value := range v {
			if key == "properties" {
				// Special handling for properties - check each property
				if props, ok := value.(map[string]any); ok {
					for _, propValue := range props {
						if propMap, ok := propValue.(map[string]any); ok {
							checkMapPropertiesForDeprecated(propMap, errorList, shortPath, kind, name)
						}
					}
				}
			} else {
				// For other keys, continue recursive search
				checkMapPropertiesForDeprecated(value, errorList, shortPath, kind, name)
			}
		}
	case []any:
		// Recursively check all items in the slice
		for _, item := range v {
			checkMapPropertiesForDeprecated(item, errorList, shortPath, kind, name)
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

		r.validateLabel(&crd, "module", moduleName, errorList, shortPath)
		r.validateDeprecatedKeyInYAML(d, &crd, errorList, shortPath)
	}
}

func splitManifests(bigFile string) []string {
	bigFileTmp := strings.TrimSpace(bigFile)
	return sep.Split(bigFileTmp, -1)
}
