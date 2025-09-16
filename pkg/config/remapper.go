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

import "github.com/deckhouse/dmt/pkg"

func NewRootConfig(userDTO *UserRootConfigDTO, globalDTO *GlobalRootConfigDTO) *DomainRootConfig {
	return &DomainRootConfig{
		LintersConfig: LintersConfig{
			Container:  NewContainerLinterConfig(&userDTO.LintersSettings.Container, &globalDTO.Global.Container),
			Hooks:      NewHooksLinterConfig(&userDTO.LintersSettings.Hooks, &globalDTO.Global.Hooks),
			Images:     NewImagesLinterConfig(&userDTO.LintersSettings.Images, &globalDTO.Global.Images),
			Module:     NewModuleLinterConfig(&userDTO.LintersSettings.Module, &globalDTO.Global.Module),
			NoCyrillic: NewNoCyrillicLinterConfig(&userDTO.LintersSettings.NoCyrillic, &globalDTO.Global.NoCyrillic),
			OpenAPI:    NewOpenAPILinterConfig(&userDTO.LintersSettings.OpenAPI, &globalDTO.Global.OpenAPI),
			Rbac:       NewRbacLinterConfig(&userDTO.LintersSettings.Rbac, &globalDTO.Global.Rbac),
			Templates:  NewTemplatesLinterConfig(&userDTO.LintersSettings.Templates, &globalDTO.Global.Templates),
		},
	}
}

func mergeImpact(userImpact, globalImpact *pkg.Level) *pkg.Level {
	if userImpact != nil {
		return userImpact
	}
	return globalImpact
}

func mergeExcludeRules(userRules, globalRules []string) []string {
	ruleMap := make(map[string]bool)
	var result []string

	for _, rule := range globalRules {
		if !ruleMap[rule] {
			ruleMap[rule] = true
			result = append(result, rule)
		}
	}

	for _, rule := range userRules {
		if !ruleMap[rule] {
			ruleMap[rule] = true
			result = append(result, rule)
		}
	}

	return result
}
