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
	Container     ContainerSettings     `mapstructure:"container"`
	Documentation DocumentationSettings `mapstructure:"documentation"`
	Hooks         HooksSettings         `mapstructure:"hooks"`
	Images        ImageSettings         `mapstructure:"images"`
	Module        ModuleSettings        `mapstructure:"module"`
	NoCyrillic    NoCyrillicSettings    `mapstructure:"no-cyrillic"`
	OpenAPI       OpenAPISettings       `mapstructure:"openapi"`
	Rbac          RbacSettings          `mapstructure:"rbac"`
	Templates     TemplatesSettings     `mapstructure:"templates"`
}

type RuleConfig struct {
	Impact string `mapstructure:"impact"`
}

func (cfg *LintersSettings) MergeGlobal(lcfg *global.Linters) {
	cfg.OpenAPI.Impact = calculateImpact(cfg.OpenAPI.Impact, lcfg.OpenAPI.Impact)
	cfg.NoCyrillic.Impact = calculateImpact(cfg.NoCyrillic.Impact, lcfg.NoCyrillic.Impact)
	cfg.Container.Impact = calculateImpact(cfg.Container.Impact, lcfg.Container.Impact)
	cfg.Documentation.Impact = calculateImpact(cfg.Documentation.Impact, lcfg.Documentation.Impact)
	cfg.Templates.Impact = calculateImpact(cfg.Templates.Impact, lcfg.Templates.Impact)
	cfg.Images.Impact = calculateImpact(cfg.Images.Impact, lcfg.Images.Impact)
	cfg.Rbac.Impact = calculateImpact(cfg.Rbac.Impact, lcfg.Rbac.Impact)
	cfg.Hooks.Impact = calculateImpact(cfg.Hooks.Impact, lcfg.Hooks.Impact)
	cfg.Module.Impact = calculateImpact(cfg.Module.Impact, lcfg.Module.Impact)
}

type ContainerSettings struct {
	ExcludeRules ContainerExcludeRules `mapstructure:"exclude-rules"`

	Impact string `mapstructure:"impact"`
}

type ContainerExcludeRules struct {
	ControllerSecurityContext KindRuleExcludeList `mapstructure:"controller-security-context"`
	DNSPolicy                 KindRuleExcludeList `mapstructure:"dns-policy"`

	HostNetworkPorts       ContainerRuleExcludeList `mapstructure:"host-network-ports"`
	Ports                  ContainerRuleExcludeList `mapstructure:"ports"`
	ReadOnlyRootFilesystem ContainerRuleExcludeList `mapstructure:"read-only-root-filesystem"`
	NoNewPrivileges        ContainerRuleExcludeList `mapstructure:"no-new-privileges"`
	SeccompProfile         ContainerRuleExcludeList `mapstructure:"seccomp-profile"`
	ImageDigest            ContainerRuleExcludeList `mapstructure:"image-digest"`
	Resources              ContainerRuleExcludeList `mapstructure:"resources"`
	SecurityContext        ContainerRuleExcludeList `mapstructure:"security-context"`
	Liveness               ContainerRuleExcludeList `mapstructure:"liveness-probe"`
	Readiness              ContainerRuleExcludeList `mapstructure:"readiness-probe"`

	Description StringRuleExcludeList `mapstructure:"description"`
}

type HooksSettings struct {
	Ingress HooksIngressRuleSetting `mapstructure:"ingress"`

	Impact string `mapstructure:"impact"`
}

type HooksIngressRuleSetting struct {
	// disable ingress rule completely
	Disable bool `mapstructure:"disable"`
}

type ImageSettings struct {
	ExcludeRules ImageExcludeRules `mapstructure:"exclude-rules"`

	Patches PatchesRuleSettings `mapstructure:"patches"`
	Werf    WerfRuleSettings    `mapstructure:"werf"`

	Impact string `mapstructure:"impact"`
}

type ImageExcludeRules struct {
	SkipImageFilePathPrefix      PrefixRuleExcludeList `mapstructure:"skip-image-file-path-prefix"`
	SkipDistrolessFilePathPrefix PrefixRuleExcludeList `mapstructure:"skip-distroless-file-path-prefix"`
}

type ModuleSettings struct {
	ExcludeRules ModuleExcludeRules `mapstructure:"exclude-rules"`

	OSS               ModuleOSSRuleSettings            `mapstructure:"oss"`
	DefinitionFile    ModuleDefinitionFileRuleSettings `mapstructure:"definition-file"`
	Conversions       ConversionsRuleSettings          `mapstructure:"conversions"`
	Helmignore        HelmignoreRuleSettings           `mapstructure:"helmignore"`
	LegacyReleaseFile RuleConfig                       `mapstructure:"legacy-release-file"`

	Impact string `mapstructure:"impact"`
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

type HelmignoreRuleSettings struct {
	// disable helmignore rule completely
	Disable bool `mapstructure:"disable"`
}

type PatchesRuleSettings struct {
	// disable conversions rule completely
	Disable bool `mapstructure:"disable"`
}

type WerfRuleSettings struct {
	// disable werf rule completely
	Disable bool `mapstructure:"disable"`
}

type LicenseExcludeRule struct {
	Files       StringRuleExcludeList `mapstructure:"files"`
	Directories PrefixRuleExcludeList `mapstructure:"directories"`
}

type NoCyrillicSettings struct {
	NoCyrillicExcludeRules NoCyrillicExcludeRules `mapstructure:"exclude-rules"`

	Impact string `mapstructure:"impact"`
}

type NoCyrillicExcludeRules struct {
	Files       StringRuleExcludeList `mapstructure:"files"`
	Directories PrefixRuleExcludeList `mapstructure:"directories"`
}

type OpenAPISettings struct {
	OpenAPIExcludeRules OpenAPIExcludeRules `mapstructure:"exclude-rules"`

	Impact string `mapstructure:"impact"`
}

type OpenAPIExcludeRules struct {
	KeyBannedNames         []string              `mapstructure:"key-banned-names"`
	EnumFileExcludes       []string              `mapstructure:"enum"`
	HAAbsoluteKeysExcludes StringRuleExcludeList `mapstructure:"ha-absolute-keys"`
	CRDNamesExcludes       StringRuleExcludeList `mapstructure:"crd-names"`
}

type RbacSettings struct {
	ExcludeRules RBACExcludeRules `mapstructure:"exclude-rules"`

	Impact string `mapstructure:"impact"`
}

type RBACExcludeRules struct {
	BindingSubject StringRuleExcludeList `mapstructure:"binding-subject"`
	Placement      KindRuleExcludeList   `mapstructure:"placement"`
	Wildcards      KindRuleExcludeList   `mapstructure:"wildcards"`
}

type TemplatesSettings struct {
	ExcludeRules      TemplatesExcludeRules        `mapstructure:"exclude-rules"`
	GrafanaDashboards GrafanaDashboardsExcludeList `mapstructure:"grafana-dashboards"`
	PrometheusRules   PrometheusRulesExcludeList   `mapstructure:"prometheus-rules"`
	Rules             TemplatesLinterRules         `mapstructure:"rules"`

	Impact string `mapstructure:"impact"`
}

type TemplatesLinterRules struct {
	VPARule           RuleConfig `mapstructure:"vpa"`
	PDBRule           RuleConfig `mapstructure:"pdb"`
	IngressRule       RuleConfig `mapstructure:"ingress"`
	PrometheusRule    RuleConfig `mapstructure:"prometheus-rules"`
	GrafanaRule       RuleConfig `mapstructure:"grafana-dashboards"`
	KubeRBACProxyRule RuleConfig `mapstructure:"kube-rbac-proxy"`
	ServicePortRule   RuleConfig `mapstructure:"service-port"`
	ClusterDomainRule RuleConfig `mapstructure:"cluster-domain"`
	RegistryRule      RuleConfig `mapstructure:"registry"`
}

type TemplatesExcludeRules struct {
	VPAAbsent     KindRuleExcludeList    `mapstructure:"vpa"`
	PDBAbsent     KindRuleExcludeList    `mapstructure:"pdb"`
	ServicePort   ServicePortExcludeList `mapstructure:"service-port"`
	KubeRBACProxy StringRuleExcludeList  `mapstructure:"kube-rbac-proxy"`
	Ingress       KindRuleExcludeList    `mapstructure:"ingress"`
}

type GrafanaDashboardsExcludeList struct {
	Disable bool `mapstructure:"disable"`
}

type PrometheusRulesExcludeList struct {
	Disable bool `mapstructure:"disable"`
}

type ServicePortExcludeList []ServicePortExclude

func (l ServicePortExcludeList) Get() []pkg.ServicePortExclude {
	result := make([]pkg.ServicePortExclude, 0, len(l))

	for idx := range l {
		result = append(result, *remapServicePortRuleExclude(&l[idx]))
	}

	return result
}

type StringRuleExcludeList []string

func (l StringRuleExcludeList) Get() []pkg.StringRuleExclude {
	result := make([]pkg.StringRuleExclude, 0, len(l))

	for idx := range l {
		result = append(result, pkg.StringRuleExclude(l[idx]))
	}

	return result
}

type PrefixRuleExcludeList []string

func (l PrefixRuleExcludeList) Get() []pkg.PrefixRuleExclude {
	result := make([]pkg.PrefixRuleExclude, 0, len(l))

	for idx := range l {
		result = append(result, pkg.PrefixRuleExclude(l[idx]))
	}

	return result
}

type KindRuleExcludeList []KindRuleExclude

func (l KindRuleExcludeList) Get() []pkg.KindRuleExclude {
	result := make([]pkg.KindRuleExclude, 0, len(l))

	for idx := range l {
		result = append(result, *remapKindRuleExclude(&l[idx]))
	}

	return result
}

type ContainerRuleExcludeList []ContainerRuleExclude

func (l ContainerRuleExcludeList) Get() []pkg.ContainerRuleExclude {
	result := make([]pkg.ContainerRuleExclude, 0, len(l))

	for idx := range l {
		result = append(result, *remapContainerRuleExclude(&l[idx]))
	}

	return result
}

type KindRuleExclude struct {
	Kind string `mapstructure:"kind"`
	Name string `mapstructure:"name"`
}

type ContainerRuleExclude struct {
	Kind      string `mapstructure:"kind"`
	Name      string `mapstructure:"name"`
	Container string `mapstructure:"container"`
}

type ServicePortExclude struct {
	Name string `mapstructure:"name"`
	Port string `mapstructure:"port"`
}

func remapKindRuleExclude(input *KindRuleExclude) *pkg.KindRuleExclude {
	return &pkg.KindRuleExclude{
		Name: input.Name,
		Kind: input.Kind,
	}
}

func remapServicePortRuleExclude(input *ServicePortExclude) *pkg.ServicePortExclude {
	return &pkg.ServicePortExclude{
		Name: input.Name,
		Port: input.Port,
	}
}

func remapContainerRuleExclude(input *ContainerRuleExclude) *pkg.ContainerRuleExclude {
	return &pkg.ContainerRuleExclude{
		Kind:      input.Kind,
		Name:      input.Name,
		Container: input.Container,
	}
}

type DocumentationSettings struct {
	Impact string `mapstructure:"impact"`
}
