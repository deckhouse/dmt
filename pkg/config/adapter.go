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

func (domain *DomainRootConfig) ToLintersSettings() *LintersSettings {
	return &LintersSettings{
		Container:  *domain.LintersConfig.Container.ToContainerSettings(),
		Hooks:      *domain.LintersConfig.Hooks.ToHooksSettings(),
		Images:     *domain.LintersConfig.Images.ToImageSettings(),
		Module:     *domain.LintersConfig.Module.ToModuleSettings(),
		NoCyrillic: *domain.LintersConfig.NoCyrillic.ToNoCyrillicSettings(),
		OpenAPI:    *domain.LintersConfig.OpenAPI.ToOpenAPISettings(),
		Rbac:       *domain.LintersConfig.Rbac.ToRbacSettings(),
		Templates:  *domain.LintersConfig.Templates.ToTemplatesSettings(),
	}
}

func (c *ContainerConfig) ToContainerSettings() *ContainerSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleConfig := range c.RulesSettings {
		rulesSettings[ruleName] = RuleSettings(ruleConfig)
	}
	return &ContainerSettings{
		Impact:        c.Impact,
		RulesSettings: rulesSettings,
		ExcludeRules:  ContainerExcludeRules{},
	}
}

func (h *HooksConfig) ToHooksSettings() *HooksSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleConfig := range h.RulesSettings {
		rulesSettings[ruleName] = RuleSettings(ruleConfig)
	}
	return &HooksSettings{
		Impact:        h.Impact,
		RulesSettings: rulesSettings,
	}
}

func (i *ImagesConfig) ToImageSettings() *ImageSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleConfig := range i.RulesSettings {
		rulesSettings[ruleName] = RuleSettings(ruleConfig)
	}
	return &ImageSettings{
		Impact:        i.Impact,
		RulesSettings: rulesSettings,
	}
}

func (m *ModuleLinterConfig) ToModuleSettings() *ModuleSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleConfig := range m.RulesSettings {
		rulesSettings[ruleName] = RuleSettings(ruleConfig)
	}
	return &ModuleSettings{
		Impact:        m.Impact,
		RulesSettings: rulesSettings,
	}
}

func (n *NoCyrillicConfig) ToNoCyrillicSettings() *NoCyrillicSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleConfig := range n.RulesSettings {
		rulesSettings[ruleName] = RuleSettings(ruleConfig)
	}
	return &NoCyrillicSettings{
		Impact:        n.Impact,
		RulesSettings: rulesSettings,
	}
}

func (o *OpenAPIConfig) ToOpenAPISettings() *OpenAPISettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleConfig := range o.RulesSettings {
		rulesSettings[ruleName] = RuleSettings(ruleConfig)
	}
	return &OpenAPISettings{
		Impact:        o.Impact,
		RulesSettings: rulesSettings,
	}
}

func (r *RbacConfig) ToRbacSettings() *RbacSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleConfig := range r.RulesSettings {
		rulesSettings[ruleName] = RuleSettings(ruleConfig)
	}
	return &RbacSettings{
		Impact:        r.Impact,
		RulesSettings: rulesSettings,
	}
}

func (t *TemplatesConfig) ToTemplatesSettings() *TemplatesSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleName, ruleConfig := range t.RulesSettings {
		rulesSettings[ruleName] = RuleSettings(ruleConfig)
	}
	return &TemplatesSettings{
		Impact:        t.Impact,
		RulesSettings: rulesSettings,
	}
}
