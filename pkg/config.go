package pkg

type LinterConfig struct {
	Impact *Level
}

type RuleConfig struct {
	impact *Level
}

func (rc *RuleConfig) GetLevel() *Level {
	return rc.impact
}

func (rc *RuleConfig) SetLevel(level *Level) {
	rc.impact = level
}

func (rc *RuleConfig) SetStringLevel(current, backoff string) {
	if current != "" {
		level := ParseStringToLevel(current)
		rc.impact = &level

		return
	}

	if backoff != "" {
		level := ParseStringToLevel(backoff)
		rc.impact = &level

		return
	}

	level := Error
	rc.impact = &level
}

func (lc *LinterConfig) SetLevel(level *Level) {
	lc.Impact = level
}

func (clc *ContainerLinterConfig) GetRuleImpact(ruleID string) *Level {
	switch ruleID {
	case "recommended-labels":
		if level := clc.Rules.RecommendedLabelsRule.GetLevel(); level != nil {
			return level
		}
	}
	return clc.Impact
}

type LintersSettings struct {
	Container  ContainerLinterConfig
	Image      ImageLinterConfig
	NoCyrillic NoCyrillicLinterConfig
	OpenAPI    OpenAPILinterConfig
	Templates  TemplatesLinterConfig
	RBAC       RBACLinterConfig
	Hooks      HooksLinterConfig
	Module     ModuleLinterConfig
}

type NoCyrillicLinterConfig struct {
	LinterConfig
	Rules        NoCyrillicLinterRules
	ExcludeRules NoCyrillicExcludeRules
}
type NoCyrillicLinterRules struct {
	NoCyrillicRule RuleConfig
}

type NoCyrillicExcludeRules struct {
	Files       StringRuleExcludeList
	Directories PrefixRuleExcludeList
}

type OpenAPILinterConfig struct {
	LinterConfig
	Rules        OpenAPILinterRules
	ExcludeRules OpenAPIExcludeRules
}
type OpenAPILinterRules struct {
	EnumRule RuleConfig
	HARule   RuleConfig
	CRDsRule RuleConfig
	KeysRule RuleConfig
}

type OpenAPIExcludeRules struct {
	KeyBannedNames         []string
	EnumFileExcludes       []string
	HAAbsoluteKeysExcludes StringRuleExcludeList
	CRDNamesExcludes       StringRuleExcludeList
}

type TemplatesLinterConfig struct {
	LinterConfig
	Rules                     TemplatesLinterRules
	ExcludeRules              TemplatesExcludeRules
	PrometheusRuleSettings    PrometheusRuleSettings
	GrafanaDashboardsSettings GrafanaDashboardsSettings
}
type TemplatesLinterRules struct {
	VPARule           RuleConfig
	PDBRule           RuleConfig
	IngressRule       RuleConfig
	PrometheusRule    RuleConfig
	GrafanaRule       RuleConfig
	KubeRBACProxyRule RuleConfig
	ServicePortRule   RuleConfig
	ClusterDomainRule RuleConfig
}

type PrometheusRuleSettings struct {
	Disable bool
}

type GrafanaDashboardsSettings struct {
	Disable bool
}
type TemplatesExcludeRules struct {
	VPAAbsent     KindRuleExcludeList
	PDBAbsent     KindRuleExcludeList
	ServicePort   ServicePortExcludeList
	KubeRBACProxy StringRuleExcludeList
	Ingress       KindRuleExcludeList
}

type ServicePortExcludeList []ServicePortExclude

func (l ServicePortExcludeList) Get() []ServicePortExclude {
	result := make([]ServicePortExclude, 0, len(l))

	for idx := range l {
		result = append(result, *remapServicePortRuleExclude(&l[idx]))
	}

	return result
}

func remapServicePortRuleExclude(input *ServicePortExclude) *ServicePortExclude {
	return &ServicePortExclude{
		Name: input.Name,
		Port: input.Port,
	}
}

type RBACLinterConfig struct {
	LinterConfig
	Rules        RBACLinterRules
	ExcludeRules RBACExcludeRules
}
type RBACLinterRules struct {
	UserAuthRule  RuleConfig
	BindingRule   RuleConfig
	PlacementRule RuleConfig
	WildcardsRule RuleConfig
}

type RBACExcludeRules struct {
	BindingSubject StringRuleExcludeList
	Placement      KindRuleExcludeList
	Wildcards      KindRuleExcludeList
}
type HooksLinterConfig struct {
	LinterConfig
	Rules               HooksLinterRules
	IngressRuleSettings IngressRuleSettings
}
type HooksLinterRules struct {
	HooksRule RuleConfig
}
type IngressRuleSettings struct {
	Disable bool
}

type ModuleLinterConfig struct {
	LinterConfig
	Rules                      ModuleLinterRules
	OSSRuleSettings            OSSRuleSettings
	DefinitionFileRuleSettings DefinitionFileRuleSettings
	ConversionsRuleSettings    ConversionsRuleSettings
	HelmignoreRuleSettings     HelmignoreRuleSettings
	ExcludeRules               ModuleExcludeRules
}
type ModuleLinterRules struct {
	DefinitionFileRule RuleConfig
	OSSRule            RuleConfig
	ConversionRule     RuleConfig
	HelmignoreRule     RuleConfig
	LicenseRule        RuleConfig
	RequarementsRule   RuleConfig
}
type OSSRuleSettings struct {
	Disable bool
}

type DefinitionFileRuleSettings struct {
	Disable bool
}
type ConversionsRuleSettings struct {
	Disable bool
}
type HelmignoreRuleSettings struct {
	Disable bool
}
type ModuleExcludeRules struct {
	License LicenseExcludeRule
}

type LicenseExcludeRule struct {
	Files       StringRuleExcludeList `mapstructure:"files"`
	Directories PrefixRuleExcludeList `mapstructure:"directories"`
}

type ContainerLinterConfig struct {
	LinterConfig
	Rules        ContainerLinterRules
	ExcludeRules ContainerExcludeRules
}

type ImageLinterConfig struct {
	LinterConfig
	Rules        ImageLinterRules
	ExcludeRules ImageExcludeRules
	Patches      PatchesRuleSettings
	Werf         WerfRuleSettings
}

type PatchesRuleSettings struct {
	Disable bool
}

type WerfRuleSettings struct {
	Disable bool
}

type ImageLinterRules struct {
	DistrolessRule RuleConfig
	ImageRule      RuleConfig
	PatchesRule    RuleConfig
	WerfRule       RuleConfig
}

type ImageExcludeRules struct {
	SkipImageFilePathPrefix      PrefixRuleExcludeList
	SkipDistrolessFilePathPrefix PrefixRuleExcludeList
}

type PrefixRuleExcludeList []string

func (l PrefixRuleExcludeList) Get() []PrefixRuleExclude {
	result := make([]PrefixRuleExclude, 0, len(l))

	for idx := range l {
		result = append(result, PrefixRuleExclude(l[idx]))
	}

	return result
}

type ContainerLinterRules struct {
	RecommendedLabelsRule RuleConfig
}

type ContainerExcludeRules struct {
	ControllerSecurityContext KindRuleExcludeList
	DNSPolicy                 KindRuleExcludeList

	HostNetworkPorts       ContainerRuleExcludeList
	Ports                  ContainerRuleExcludeList
	ReadOnlyRootFilesystem ContainerRuleExcludeList
	ImageDigest            ContainerRuleExcludeList
	Resources              ContainerRuleExcludeList
	SecurityContext        ContainerRuleExcludeList
	Liveness               ContainerRuleExcludeList
	Readiness              ContainerRuleExcludeList

	Description StringRuleExcludeList
}

type StringRuleExcludeList []string

func (l StringRuleExcludeList) Get() []StringRuleExclude {
	result := make([]StringRuleExclude, 0, len(l))
	for idx := range l {
		result = append(result, StringRuleExclude(l[idx]))
	}
	return result
}

type KindRuleExcludeList []KindRuleExclude

func (l KindRuleExcludeList) Get() []KindRuleExclude {
	result := make([]KindRuleExclude, 0, len(l))
	for idx := range l {
		result = append(result, l[idx])
	}
	return result
}

// ContainerRuleExcludeList represents a list of container exclusions
type ContainerRuleExcludeList []ContainerRuleExclude

func (l ContainerRuleExcludeList) Get() []ContainerRuleExclude {
	result := make([]ContainerRuleExclude, 0, len(l))
	for idx := range l {
		result = append(result, l[idx])
	}
	return result
}
