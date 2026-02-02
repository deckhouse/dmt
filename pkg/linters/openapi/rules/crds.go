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

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
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

func NewDeckhouseCRDsRule(cfg *pkg.OpenAPILinterConfig, rootPath string) *DeckhouseCRDsRule {
	return &DeckhouseCRDsRule{
		RuleMeta: pkg.RuleMeta{
			Name: "deckhouse-crds",
		},
		StringRule: pkg.StringRule{
			ExcludeRules: cfg.ExcludeRules.CRDNamesExcludes.Get(),
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

// validateDeprecatedKeyInYAML checks for deprecated keys specifically within the properties section
// of the CRD schema. It parses the YAML document and searches for deprecated keys only in the
// spec.versions[].schema.openAPIV3Schema.properties path, ignoring deprecated keys in other
// parts of the CRD structure.
func (*DeckhouseCRDsRule) validateDeprecatedKeyInYAML(yamlDoc string, crd *v1beta1.CustomResourceDefinition, errorList *errors.LintRuleErrorsList, shortPath string) {
	// Parse YAML as map to search for deprecated key in properties only
	var yamlMap map[string]any
	if err := yaml.Unmarshal([]byte(yamlDoc), &yamlMap); err != nil {
		return
	}

	// Check for deprecated key only in properties section
	checkPropertiesForDeprecated(yamlMap, errorList, shortPath, crd.Kind, crd.Name)
}

// aggregateVersionProperties extracts and aggregates the properties section from all CRD versions
// It merges properties from all versions to ensure comprehensive validation
func aggregateVersionProperties(data map[string]any) map[string]any {
	spec, ok := data["spec"].(map[string]any)
	if !ok {
		return nil
	}

	versions, ok := spec["versions"].([]any)
	if !ok {
		return nil
	}

	// Aggregate properties from all versions
	allProperties := make(map[string]any)

	for _, version := range versions {
		versionMap, ok := version.(map[string]any)
		if !ok {
			continue
		}

		schema, ok := versionMap["schema"].(map[string]any)
		if !ok {
			continue
		}

		openAPIV3Schema, ok := schema["openAPIV3Schema"].(map[string]any)
		if !ok {
			continue
		}

		props, ok := openAPIV3Schema["properties"].(map[string]any)
		if ok {
			// Deep merge properties from this version into the aggregated map
			// This ensures all nested schemas from every version are validated
			deepMergeProperties(allProperties, props)
		}
	}

	// Return aggregated properties if any were found
	if len(allProperties) > 0 {
		return allProperties
	}

	return nil
}

// deepMergeProperties performs a deep merge of property maps, ensuring all nested schemas
// from every version are included in the validation. This handles cases where the same
// property key is redefined across versions with different nested structures.
func deepMergeProperties(target, source map[string]any) {
	for key, sourceValue := range source {
		if existingValue, exists := target[key]; exists {
			// If both values are maps, recursively merge them
			if targetMap, ok := existingValue.(map[string]any); ok {
				if sourceMap, ok := sourceValue.(map[string]any); ok {
					deepMergeProperties(targetMap, sourceMap)
					continue
				}
			}
			// If values are different types or not maps, prefer the source value
			// This ensures we capture all variations across versions
			target[key] = sourceValue
		} else {
			// New key, add it directly
			target[key] = sourceValue
		}
	}
}

func checkPropertiesForDeprecated(data any, errorList *errors.LintRuleErrorsList, shortPath, kind, name string) {
	if yamlMap, ok := data.(map[string]any); ok {
		props := aggregateVersionProperties(yamlMap)
		if props != nil {
			// Start with the base path for properties
			basePath := "spec.versions[].schema.openAPIV3Schema.properties"
			checkDeprecatedInPropertiesRecursively(props, errorList, shortPath, kind, name, basePath)
		}
	}
}

// checkDeprecatedInPropertiesRecursively recursively checks for deprecated keys in properties
// and tracks the full path to provide detailed error messages
func checkDeprecatedInPropertiesRecursively(data any, errorList *errors.LintRuleErrorsList, shortPath, kind, name, currentPath string) {
	switch v := data.(type) {
	case map[string]any:
		// Check if current map has deprecated key (regardless of value)
		if _, hasDeprecated := v["deprecated"]; hasDeprecated {
			errorList.WithObjectID(fmt.Sprintf("kind = %s ; name = %s", kind, name)).
				WithFilePath(shortPath).
				WithValue(fmt.Sprintf("deprecated: present at path %s", currentPath)).
				Errorf(`CRD contains "deprecated" key at path "%s", use "x-doc-deprecated: true" instead`, currentPath)
		}

		// Recursively check all values in the map
		for key, value := range v {
			// Build the path for this property
			var newPath string
			if currentPath == "" {
				newPath = key
			} else {
				newPath = fmt.Sprintf("%s.%s", currentPath, key)
			}
			checkDeprecatedInPropertiesRecursively(value, errorList, shortPath, kind, name, newPath)
		}
	case []any:
		// Recursively check all items in the slice
		for i, item := range v {
			// Build the path for array items
			newPath := fmt.Sprintf("%s[%d]", currentPath, i)
			checkDeprecatedInPropertiesRecursively(item, errorList, shortPath, kind, name, newPath)
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
