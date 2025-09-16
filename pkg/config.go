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

type LintersSettings struct {
	Container ContainerLinterConfig
	Image     ImageLinterConfig
}

type ImageLinterConfig struct {
	LinterConfig
	Rules        ImageLinterRules
	ExcludeRules ImageExcludeRules
}

type ImageLinterRules struct {
	DistrolessRule RuleConfig
	ImageRule      RuleConfig
	PatchesRule    RuleConfig
	WerfRule       RuleConfig
}

type ImageExcludeRules struct {
}

type ContainerLinterConfig struct {
	LinterConfig
	ExcludeRules ContainerExcludeRules
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

// ContainerRuleExcludeList represents a list of container exclusions
type ContainerRuleExcludeList []ContainerRuleExclude
