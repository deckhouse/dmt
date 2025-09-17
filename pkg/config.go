package pkg

import "github.com/deckhouse/dmt/pkg"

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
		rc.impact = pkg.ParseStringToLevel(current)

		return
	}

	if backoff != "" {
		rc.impact = pkg.ParseStringToLevel(backoff)

		return
	}

	rc.impact = pkg.Error
}

func (lc *LinterConfig) SetLevel(level *Level) {
	lc.Impact = level
}

func (ilc *ImageLinterConfig) GetRuleImpact(ruleID string) *Level {
	switch ruleID {
	case "distroless":
		if level := ilc.Rules.DistrolessRule.GetLevel(); level != nil {
			return level
		}
	case "image":
		if level := ilc.Rules.ImageRule.GetLevel(); level != nil {
			return level
		}
	case "patches":
		if level := ilc.Rules.PatchesRule.GetLevel(); level != nil {
			return level
		}
	case "werf":
		if level := ilc.Rules.WerfRule.GetLevel(); level != nil {
			return level
		}
	}
	return ilc.Impact
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
	Container ContainerLinterConfig
	Image     ImageLinterConfig
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
