package config

import (
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config/global"
)

type LintersSettings struct {
	Container  ContainerSettings  `mapstructure:"container"`
	Hooks      HooksSettings      `mapstructure:"hooks"`
	Images     ImageSettings      `mapstructure:"images"`
	License    LicenseSettings    `mapstructure:"license"`
	Module     ModuleSettings     `mapstructure:"module"`
	NoCyrillic NoCyrillicSettings `mapstructure:"nocyrillic"`
	OpenAPI    OpenAPISettings    `mapstructure:"openapi"`
	Rbac       RbacSettings       `mapstructure:"rbac"`
	Templates  TemplatesSettings  `mapstructure:"templates"`
}

func (cfg *LintersSettings) MergeGlobal(lcfg *global.Linters) {
	assignIfNotEmpty(&cfg.OpenAPI.Impact, lcfg.OpenAPI.Impact)
	assignIfNotEmpty(&cfg.NoCyrillic.Impact, lcfg.NoCyrillic.Impact)
	assignIfNotEmpty(&cfg.License.Impact, lcfg.License.Impact)
	assignIfNotEmpty(&cfg.Container.Impact, lcfg.Container.Impact)
	assignIfNotEmpty(&cfg.Templates.Impact, lcfg.Templates.Impact)
	assignIfNotEmpty(&cfg.Images.Impact, lcfg.Images.Impact)
	assignIfNotEmpty(&cfg.Rbac.Impact, lcfg.Rbac.Impact)
	assignIfNotEmpty(&cfg.Hooks.Impact, lcfg.Hooks.Impact)
	assignIfNotEmpty(&cfg.Module.Impact, lcfg.Module.Impact)
}

type ContainerSettings struct {
	SkipContainers []string              `mapstructure:"skip-containers"`
	ExcludeRules   ContainerExcludeRules `mapstructure:"exclude-rules"`

	Impact pkg.Level `mapstructure:"impact"`
}

type ContainerExcludeRules struct {
	ReadOnlyRootFilesystem ContainerRuleExcludeList `mapstructure:"read-only-root-filesystem"`
	Resources              ContainerRuleExcludeList `mapstructure:"resources"`
	SecurityContext        ContainerRuleExcludeList `mapstructure:"security-context"`
	Liveness               ContainerRuleExcludeList `mapstructure:"liveness-probe"`
	Readiness              ContainerRuleExcludeList `mapstructure:"readiness-probe"`

	DNSPolicy KindRuleExcludeList `mapstructure:"dns-policy"`

	Description StringRuleExcludeList `mapstructure:"description"`
}

type HooksSettings struct {
	Ingress HooksIngressRuleSetting `mapstructure:"ingress"`

	Impact pkg.Level `mapstructure:"impact"`
}

type HooksIngressRuleSetting struct {
	// disable ingress rule completely
	Disable bool `mapstructure:"disable"`
}

type ImageSettings struct {
	SkipModuleImageName      []string `mapstructure:"skip-module-image-name"`
	SkipDistrolessImageCheck []string `mapstructure:"skip-distroless-image-check"`
	SkipNamespaceCheck       []string `mapstructure:"skip-namespace-check"`

	Impact pkg.Level `mapstructure:"impact"`
}

type LicenseSettings struct {
	CopyrightExcludes []string            `mapstructure:"copyright-excludes"`
	ExcludeRules      LicenseExcludeRules `mapstructure:"exclude-rules"`

	Impact pkg.Level `mapstructure:"impact"`
}

type LicenseExcludeRules struct {
	Files StringRuleExcludeList `mapstructure:"files"`
}

type ModuleSettings struct {
	SkipCheckModuleYaml []string `mapstructure:"skip-check-module-yaml"`

	OSS            ModuleOSSRuleSettings            `mapstructure:"oss"`
	DefinitionFile ModuleDefinitionFileRuleSettings `mapstructure:"definition-file"`
	Conversions    ConversionsRuleSettings          `mapstructure:"conversions"`

	Impact pkg.Level `mapstructure:"impact"`
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

type NoCyrillicSettings struct {
	NoCyrillicFileExcludes []string `mapstructure:"no-cyrillic-file-excludes"`

	Impact pkg.Level `mapstructure:"impact"`
}

type OpenAPISettings struct {
	OpenAPIExcludeRules `mapstructure:"exclude-rules"`

	Impact pkg.Level `mapstructure:"impact"`
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

	Impact pkg.Level `mapstructure:"impact"`
}

type RBACExcludeRules struct {
	Placement KindRuleExcludeList `mapstructure:"placement"`
	Wildcards KindRuleExcludeList `mapstructure:"wildcards"`
}

type TemplatesSettings struct {
	SkipVPAChecks []string              `mapstructure:"skip-vpa-checks"`
	ExcludeRules  TemplatesExcludeRules `mapstructure:"exclude-rules"`

	Impact pkg.Level `mapstructure:"impact"`
}

type TemplatesExcludeRules struct {
	VPAAbsent     KindRuleExcludeList   `mapstructure:"vpa"`
	PDBAbsent     KindRuleExcludeList   `mapstructure:"pdb"`
	ServicePort   StringRuleExcludeList `mapstructure:"service-port"`
	KubeRBACProxy StringRuleExcludeList `mapstructure:"kube-rbac-proxy"`
}

type StringRuleExcludeList []string

func (l StringRuleExcludeList) Get() []pkg.StringRuleExclude {
	result := make([]pkg.StringRuleExclude, 0, len(l))

	for idx := range l {
		result = append(result, pkg.StringRuleExclude(l[idx]))
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

func remapKindRuleExclude(input *KindRuleExclude) *pkg.KindRuleExclude {
	return &pkg.KindRuleExclude{
		Kind: input.Kind,
		Name: input.Name,
	}
}

func remapContainerRuleExclude(input *ContainerRuleExclude) *pkg.ContainerRuleExclude {
	return &pkg.ContainerRuleExclude{
		Kind:      input.Kind,
		Name:      input.Name,
		Container: input.Container,
	}
}
