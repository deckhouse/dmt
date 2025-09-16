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

func (r *DomainRootConfig) ToLintersSettings() *LintersSettings {
	return &LintersSettings{
		Container:  r.LintersConfig.Container.ToContainerSettings(),
		Hooks:      r.LintersConfig.Hooks.ToHooksSettings(),
		Images:     r.LintersConfig.Images.ToImagesSettings(),
		Module:     r.LintersConfig.Module.ToModuleSettings(),
		NoCyrillic: r.LintersConfig.NoCyrillic.ToNoCyrillicSettings(),
		OpenAPI:    r.LintersConfig.OpenAPI.ToOpenAPISettings(),
		Rbac:       r.LintersConfig.Rbac.ToRbacSettings(),
		Templates:  r.LintersConfig.Templates.ToTemplatesSettings(),
	}
}

func (c *ContainerLinterConfig) ToContainerSettings() ContainerSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleID, ruleConfig := range c.RulesSettings {
		rulesSettings[ruleID] = RuleSettings(ruleConfig)
	}

	excludeRules := ContainerExcludeRules{}

	return ContainerSettings{
		Impact:        c.Impact,
		RulesSettings: rulesSettings,
		ExcludeRules:  excludeRules,
	}
}

func (h *HooksLinterConfig) ToHooksSettings() HooksSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleID, ruleConfig := range h.RulesSettings {
		rulesSettings[ruleID] = RuleSettings(ruleConfig)
	}

	return HooksSettings{
		Impact:        h.Impact,
		RulesSettings: rulesSettings,
	}
}

func (i *ImagesLinterConfig) ToImagesSettings() ImageSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleID, ruleConfig := range i.RulesSettings {
		rulesSettings[ruleID] = RuleSettings(ruleConfig)
	}

	excludeRules := ImageExcludeRules{}

	return ImageSettings{
		Impact:        i.Impact,
		RulesSettings: rulesSettings,
		ExcludeRules:  excludeRules,
	}
}

func (m *ModuleLinterConfig) ToModuleSettings() ModuleSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleID, ruleConfig := range m.RulesSettings {
		rulesSettings[ruleID] = RuleSettings(ruleConfig)
	}

	excludeRules := ModuleExcludeRules{}

	return ModuleSettings{
		Impact:        m.Impact,
		RulesSettings: rulesSettings,
		ExcludeRules:  excludeRules,
	}
}

func (n *NoCyrillicLinterConfig) ToNoCyrillicSettings() NoCyrillicSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleID, ruleConfig := range n.RulesSettings {
		rulesSettings[ruleID] = RuleSettings(ruleConfig)
	}

	return NoCyrillicSettings{
		Impact:        n.Impact,
		RulesSettings: rulesSettings,
	}
}

func (o *OpenAPILinterConfig) ToOpenAPISettings() OpenAPISettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleID, ruleConfig := range o.RulesSettings {
		rulesSettings[ruleID] = RuleSettings(ruleConfig)
	}

	return OpenAPISettings{
		Impact:        o.Impact,
		RulesSettings: rulesSettings,
	}
}

func (r *RbacLinterConfig) ToRbacSettings() RbacSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleID, ruleConfig := range r.RulesSettings {
		rulesSettings[ruleID] = RuleSettings(ruleConfig)
	}

	return RbacSettings{
		Impact:        r.Impact,
		RulesSettings: rulesSettings,
	}
}

func (t *TemplatesLinterConfig) ToTemplatesSettings() TemplatesSettings {
	rulesSettings := make(map[string]RuleSettings)
	for ruleID, ruleConfig := range t.RulesSettings {
		rulesSettings[ruleID] = RuleSettings(ruleConfig)
	}

	return TemplatesSettings{
		Impact:        t.Impact,
		RulesSettings: rulesSettings,
	}
}
