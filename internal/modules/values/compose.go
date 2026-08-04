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

package values

import (
	"fmt"

	"dario.cat/mergo"
	"github.com/go-openapi/spec"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/dmt/internal/modules/values/schema/defaults"
)

func applyDigests(moduleName string, digests, helmValues map[string]any) {
	moduleName = ModuleCamelName(moduleName)
	obj := map[string]any{
		"global": map[string]any{
			"modulesImages": map[string]any{
				"digests": digests,
				"registry": map[string]any{
					"base": "registry.example.com/deckhouse",
				},
			},
		},
		moduleName: map[string]any{
			"registry": map[string]any{
				"dockercfg": "ZG9ja2VyY2Zn",
			},
		},
	}

	_ = mergo.Merge(&helmValues, obj, mergo.WithOverride)
}

// HelmFormatModuleImages injects dmt's render stubs into the module's values:
// image digests (scanned from images/, keyed by the module's camelCase name),
// a fake registry, and the gateway-API discovery stub. It returns the .Values
// tree fed to the renderer; .Release and .Capabilities are supplied by nelm
// (extra API versions travel separately as render.ExtraAPIVersions).
func HelmFormatModuleImages(modulePath, moduleName string, rawValues map[string]any) (chartutil.Values, error) {
	// Start from the global stubs so every .Values.global.* key deckhouse_lib_helm
	// indexes exists offline, then overlay the module's own generated values (they
	// win on conflict; stub-only keys survive).
	vals, err := globalStubs()
	if err != nil {
		return nil, err
	}

	if err := mergo.Merge(&vals, rawValues, mergo.WithOverride); err != nil {
		return nil, fmt.Errorf("merge module values over global stubs: %w", err)
	}

	moduleDigests, err := loadDigests(modulePath)
	if err != nil {
		return nil, fmt.Errorf("load image digests: %w", err)
	}

	digests := map[string]any{
		// Common images live on the platform, not in the module, so they can't be
		// scanned from images/; stub the known set so helm_lib_module_common_image
		// resolves.
		"common": commonDigests(),
		// The module's own images, scanned from images/ and keyed by the module's
		// camelCase name — the shape helm_lib_module_image reads.
		ModuleCamelName(moduleName): moduleDigests,
	}

	applyDigests(moduleName, digests, vals)

	return vals, nil
}

// ComposeValuesFromSchemas generates a module's render values from its openapi
// schemas (default "values.yaml"), keyed by the module's path and name.
func ComposeValuesFromSchemas(modulePath, moduleName string, globalSchema *spec.Schema) (chartutil.Values, error) {
	return ComposeValuesFromSchemasForValuesFile(modulePath, moduleName, globalSchema, "values.yaml")
}

// ComposeValuesFromSchemasForValuesFile is like ComposeValuesFromSchemas but
// generates the module values from the given openapi values schema file name
// (e.g. "values_ce.yaml") instead of the default "values.yaml".
func ComposeValuesFromSchemasForValuesFile(modulePath, moduleName string, globalSchema *spec.Schema, valuesFile string) (chartutil.Values, error) {
	if globalSchema == nil {
		globalSchema = &spec.Schema{}
	}

	moduleValues, err := GetModuleValuesForValuesFile(modulePath, valuesFile)
	if err != nil {
		return nil, fmt.Errorf("cannot find openapi values schema for module %q: %w", moduleName, err)
	}

	moduleSchema := *moduleValues
	moduleSchema.Default = make(map[string]any)

	camelizedModuleName := ModuleCamelName(moduleName)
	combinedSchema := spec.Schema{}
	combinedSchema.Properties = map[string]spec.Schema{camelizedModuleName: moduleSchema, "global": *globalSchema}

	rawValues, err := defaults.Generate(&combinedSchema)
	if err != nil {
		return nil, fmt.Errorf("generate values: %w", err)
	}

	return HelmFormatModuleImages(modulePath, moduleName, rawValues)
}
