package pkg

type LinterConfig struct {
	Impact *Level
}

type RuleConfig struct {
	Impact *Level
}

func (rc *RuleConfig) GetLevel() *Level {
	return rc.Impact
}

type LintersSettings struct {
	Container ContainerLinterConfig
}

type ContainerLinterConfig struct {
	LinterConfig
	Rules        ContainerLinterRules
	ExcludeRules ContainerExcludeRules
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

// ContainerRuleExcludeList represents a list of container exclusions
type ContainerRuleExcludeList []ContainerRuleExclude
