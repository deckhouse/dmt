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

func (r *RuntimeLintersSettings) ToLintersSettings() *LintersSettings {
	return &LintersSettings{
		Container:  *r.Container.toContainerSettings(),
		Hooks:      *r.Hooks.toHooksSettings(),
		Images:     *r.Images.toImageSettings(),
		Module:     *r.Module.toModuleSettings(),
		NoCyrillic: *r.NoCyrillic.toNoCyrillicSettings(),
		OpenAPI:    *r.OpenAPI.toOpenAPISettings(),
		Rbac:       *r.Rbac.toRbacSettings(),
		Templates:  *r.Templates.toTemplatesSettings(),
	}
}

func (r *RuntimeLinterSettings) toContainerSettings() *ContainerSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleSettings := range r.RulesSettings {
		rulesSettings[ruleName] = RuleSettings{
			Impact: ruleSettings.Impact,
		}
	}
	return &ContainerSettings{
		Impact:        r.Impact,
		RulesSettings: rulesSettings,
	}
}

func (r *RuntimeLinterSettings) toHooksSettings() *HooksSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleSettings := range r.RulesSettings {
		rulesSettings[ruleName] = RuleSettings{
			Impact: ruleSettings.Impact,
		}
	}
	return &HooksSettings{
		Impact:        r.Impact,
		RulesSettings: rulesSettings,
	}
}

func (r *RuntimeLinterSettings) toImageSettings() *ImageSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleSettings := range r.RulesSettings {
		rulesSettings[ruleName] = RuleSettings{
			Impact: ruleSettings.Impact,
		}
	}
	return &ImageSettings{
		Impact:        r.Impact,
		RulesSettings: rulesSettings,
	}
}

func (r *RuntimeLinterSettings) toModuleSettings() *ModuleSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleSettings := range r.RulesSettings {
		rulesSettings[ruleName] = RuleSettings{
			Impact: ruleSettings.Impact,
		}
	}
	return &ModuleSettings{
		Impact:        r.Impact,
		RulesSettings: rulesSettings,
	}
}

func (r *RuntimeLinterSettings) toNoCyrillicSettings() *NoCyrillicSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleSettings := range r.RulesSettings {
		rulesSettings[ruleName] = RuleSettings{
			Impact: ruleSettings.Impact,
		}
	}
	return &NoCyrillicSettings{
		Impact:        r.Impact,
		RulesSettings: rulesSettings,
	}
}

func (r *RuntimeLinterSettings) toOpenAPISettings() *OpenAPISettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleSettings := range r.RulesSettings {
		rulesSettings[ruleName] = RuleSettings{
			Impact: ruleSettings.Impact,
		}
	}
	return &OpenAPISettings{
		Impact:        r.Impact,
		RulesSettings: rulesSettings,
	}
}

func (r *RuntimeLinterSettings) toRbacSettings() *RbacSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleSettings := range r.RulesSettings {
		rulesSettings[ruleName] = RuleSettings{
			Impact: ruleSettings.Impact,
		}
	}
	return &RbacSettings{
		Impact:        r.Impact,
		RulesSettings: rulesSettings,
	}
}

func (r *RuntimeLinterSettings) toTemplatesSettings() *TemplatesSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleSettings := range r.RulesSettings {
		rulesSettings[ruleName] = RuleSettings{
			Impact: ruleSettings.Impact,
		}
	}
	return &TemplatesSettings{
		Impact:        r.Impact,
		RulesSettings: rulesSettings,
	}
}

func (r *RuntimeLinterSettings) ToLegacyModuleSettings() *ModuleSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleSettings := range r.RulesSettings {
		rulesSettings[ruleName] = RuleSettings{
			Impact: ruleSettings.Impact,
		}
	}

	return &ModuleSettings{
		Impact:        r.Impact,
		RulesSettings: rulesSettings,
	}
}

func (r *RuntimeLinterSettings) ToLegacyContainerSettings() *ContainerSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleSettings := range r.RulesSettings {
		rulesSettings[ruleName] = RuleSettings{
			Impact: ruleSettings.Impact,
		}
	}

	return &ContainerSettings{
		Impact:        r.Impact,
		RulesSettings: rulesSettings,
	}
}
