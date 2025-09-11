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

package config

import (
	"github.com/deckhouse/dmt/pkg/config/global"
)

// RemapGlobalLintersToLintersSettings converts global.Linters to LintersSettings
func RemapGlobalLintersToLintersSettings(globalLinters *global.Linters) *LintersSettings {
	if globalLinters == nil {
		return &LintersSettings{}
	}

	return &LintersSettings{
		Container:  ContainerSettings{Impact: globalLinters.Container.Impact},
		Hooks:      HooksSettings{Impact: globalLinters.Hooks.Impact},
		Images:     ImageSettings{Impact: globalLinters.Images.Impact},
		Module:     ModuleSettings{Impact: globalLinters.Module.Impact},
		NoCyrillic: NoCyrillicSettings{Impact: globalLinters.NoCyrillic.Impact},
		OpenAPI:    OpenAPISettings{Impact: globalLinters.OpenAPI.Impact},
		Rbac:       RbacSettings{Impact: globalLinters.Rbac.Impact},
		Templates:  TemplatesSettings{Impact: globalLinters.Templates.Impact},
	}
}
