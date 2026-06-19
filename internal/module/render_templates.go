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

package module

import (
	"fmt"

	"github.com/deckhouse/dmt/internal/helm"
)

// helmLibOverrides returns the deterministic helm_lib helper stubs dmt injects
// during rendering so that image and module-name references resolve to stable
// values regardless of the helm_lib version a module ships with.
func helmLibOverrides() map[string][]byte {
	return map[string][]byte{
		"_module_name.tpl":  moduleNameTemplate,
		"_module_image.tpl": moduleImageTemplate,
	}
}

// RenderModuleWithValues renders the module at modulePath with the supplied
// user values (the ".Values" tree, e.g. holding "global" and the module's own
// section) and returns the rendered manifests keyed by chart-relative source
// file path.
//
// Unlike the linter render path, value generation is deterministic: only the
// provided values plus dmt's fixed image/registry stubs are used, so the output
// is suitable for golden-snapshot template testing. Rendering is strict
// (LintMode is off), so `required` and `fail` calls fail the render.
func RenderModuleWithValues(modulePath string, userValues map[string]any) (map[string]string, error) {
	mod, err := newModuleFromPath(modulePath)
	if err != nil {
		return nil, err
	}

	if userValues == nil {
		userValues = map[string]any{}
	}

	renderValues, err := helmFormatModuleImages(mod, userValues)
	if err != nil {
		return nil, fmt.Errorf("prepare render values: %w", err)
	}

	renderer := helm.Renderer{
		Name:             mod.GetName(),
		Namespace:        mod.GetNamespace(),
		LintMode:         false,
		HelmLibOverrides: helmLibOverrides(),
	}

	files, err := renderer.RenderChartFromDir(mod.GetPath(), renderValues)
	if err != nil {
		return nil, fmt.Errorf("render module: %w", err)
	}

	return files, nil
}
