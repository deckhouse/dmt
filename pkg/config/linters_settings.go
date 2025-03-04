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
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config/global"
)

type LintersSettings struct {
	Container  ContainerSettings  `mapstructure:"container"`
	Hooks      HooksSettings      `mapstructure:"hooks"`
	Images     ImageSettings      `mapstructure:"images"`
	Module     ModuleSettings     `mapstructure:"module"`
	NoCyrillic NoCyrillicSettings `mapstructure:"no-cyrillic"`
	OpenAPI    OpenAPISettings    `mapstructure:"openapi"`
	Rbac       RbacSettings       `mapstructure:"rbac"`
	Templates  TemplatesSettings  `mapstructure:"templates"`
}

func (cfg *LintersSettings) MergeGlobal(lcfg *global.Linters) {
	cfg.OpenAPI.Impact = calculateImpact(cfg.OpenAPI.Impact, lcfg.OpenAPI.Impact)
	cfg.NoCyrillic.Impact = calculateImpact(cfg.NoCyrillic.Impact, lcfg.NoCyrillic.Impact)
	cfg.Container.Impact = calculateImpact(cfg.Container.Impact, lcfg.Container.Impact)
	cfg.Templates.Impact = calculateImpact(cfg.Templates.Impact, lcfg.Templates.Impact)
	cfg.Images.Impact = calculateImpact(cfg.Images.Impact, lcfg.Images.Impact)
	cfg.Rbac.Impact = calculateImpact(cfg.Rbac.Impact, lcfg.Rbac.Impact)
	cfg.Hooks.Impact = calculateImpact(cfg.Hooks.Impact, lcfg.Hooks.Impact)
	cfg.Module.Impact = calculateImpact(cfg.Module.Impact, lcfg.Module.Impact)
}

type ContainerSettings struct {
	SkipContainers []string              `mapstructure:"skip-containers"`
	ExcludeRules   ContainerExcludeRules `mapstructure:"exclude-rules"`

	Impact *pkg.Level `mapstructure:"impact"`
}

type ContainerExcludeRules struct {
	ControllerSecurityContext KindRuleExcludeList `mapstructure:"controller-security-context"`
	DNSPolicy                 KindRuleExcludeList `mapstructure:"dns-policy"`

	HostNetworkPorts       ContainerRuleExcludeList `mapstructure:"host-network-ports"`
	Ports                  ContainerRuleExcludeList `mapstructure:"ports"`
	ReadOnlyRootFilesystem ContainerRuleExcludeList `mapstructure:"read-only-root-filesystem"`
	ImageDigest            ContainerRuleExcludeList `mapstructure:"image-digest"`
	Resources              ContainerRuleExcludeList `mapstructure:"resources"`
	SecurityContext        ContainerRuleExcludeList `mapstructure:"security-context"`
	Liveness               ContainerRuleExcludeList `mapstructure:"liveness-probe"`
	Readiness              ContainerRuleExcludeList `mapstructure:"readiness-probe"`

	Description StringRuleExcludeList `mapstructure:"description"`
}

type HooksSettings struct {
	Ingress HooksIngressRuleSetting `mapstructure:"ingress"`

	Impact *pkg.Level `mapstructure:"impact"`
}

type HooksIngressRuleSetting struct {
	// disable ingress rule completely
	Disable bool `mapstructure:"disable"`
}

type ImageSettings struct {
	SkipModuleImageName      PrefixRuleExcludeList `mapstructure:"skip-module-image-name"`
	SkipDistrolessImageCheck PrefixRuleExcludeList `mapstructure:"skip-distroless-image-check"`

	Impact *pkg.Level `mapstructure:"impact"`
}

type ModuleSettings struct {
	ExcludeRules ModuleExcludeRules `mapstructure:"exclude-rules"`

	OSS            ModuleOSSRuleSettings            `mapstructure:"oss"`
	DefinitionFile ModuleDefinitionFileRuleSettings `mapstructure:"definition-file"`
	Conversions    ConversionsRuleSettings          `mapstructure:"conversions"`

	Impact *pkg.Level `mapstructure:"impact"`
}

type ModuleExcludeRules struct {
	License LicenseExcludeRule `mapstructure:"license"`
}

type ModuleOSSRuleSettings struct {
	// disable oss rule completely
	Disable bool `mapstructure:"disable"`
}

type ModuleDefinitionFileRuleSettings struct {
	// disable definition-file rule completely
	Disable bool `mapstructure:"disable"`
}

type ConversionsRuleSettings struct {
	// disable conversions rule completely
	Disable bool `mapstructure:"disable"`
}

type LicenseExcludeRule struct {
	Files       StringRuleExcludeList `mapstructure:"files"`
	Directories PrefixRuleExcludeList `mapstructure:"directories"`
}

type NoCyrillicSettings struct {
	NoCyrillicExcludeRules NoCyrillicExcludeRules `mapstructure:"exclude-rules"`

	Impact *pkg.Level `mapstructure:"impact"`
}

type NoCyrillicExcludeRules struct {
	Files       StringRuleExcludeList `mapstructure:"files"`
	Directories PrefixRuleExcludeList `mapstructure:"directories"`
}

type OpenAPISettings struct {
	OpenAPIExcludeRules OpenAPIExcludeRules `mapstructure:"exclude-rules"`

	Impact *pkg.Level `mapstructure:"impact"`
}

type OpenAPIExcludeRules struct {
	KeyBannedNames         []string              `mapstructure:"key-banned-names"`
	EnumFileExcludes       []string              `mapstructure:"enum"`
	HAAbsoluteKeysExcludes StringRuleExcludeList `mapstructure:"ha-absolute-keys"`
}

type RbacSettings struct {
	SkipCheckWildcards     map[string][]string `mapstructure:"skip-check-wildcards"`
	SkipModuleCheckBinding []string            `mapstructure:"skip-module-check-binding"`
	SkipObjectCheckBinding []string            `mapstructure:"skip-object-check-binding"`
	ExcludeRules           RBACExcludeRules    `mapstructure:"exclude-rules"`

	Impact *pkg.Level `mapstructure:"impact"`
}

type RBACExcludeRules struct {
	Placement KindRuleExcludeList `mapstructure:"placement"`
	Wildcards KindRuleExcludeList `mapstructure:"wildcards"`
}

type TemplatesSettings struct {
	SkipVPAChecks []string              `mapstructure:"skip-vpa-checks"`
	ExcludeRules  TemplatesExcludeRules `mapstructure:"exclude-rules"`

	Impact *pkg.Level `mapstructure:"impact"`
}

type TemplatesExcludeRules struct {
	VPAAbsent     KindRuleExcludeList    `mapstructure:"vpa"`
	PDBAbsent     KindRuleExcludeList    `mapstructure:"pdb"`
	ServicePort   ServicePortExcludeList `mapstructure:"service-port"`
	KubeRBACProxy StringRuleExcludeList  `mapstructure:"kube-rbac-proxy"`
}

type ServicePortExcludeList []pkg.ServicePortExclude

type StringRuleExcludeList []pkg.StringRuleExclude

type PrefixRuleExcludeList []pkg.PrefixRuleExclude

type KindRuleExcludeList []pkg.KindRuleExclude

type ContainerRuleExcludeList []pkg.ContainerRuleExclude
