package probes

import (
	"github.com/deckhouse/dmt/pkg"
)

const (
	LivenessRuleName  = "liveness"
	ReadinessRuleName = "readiness"
)

func NewLivenessRule(excludeRules []pkg.ContainerRuleExclude) *LivenessRule {
	return &LivenessRule{
		RuleMeta: pkg.RuleMeta{
			Name: LivenessRuleName,
		},
		ContainerRule: pkg.ContainerRule{
			ExcludeRules: excludeRules,
		},
	}
}

type LivenessRule struct {
	pkg.RuleMeta
	pkg.ContainerRule
}

func NewReadinessRule(excludeRules []pkg.ContainerRuleExclude) *ReadinessRuleNameRule {
	return &ReadinessRuleNameRule{
		RuleMeta: pkg.RuleMeta{
			Name: ReadinessRuleName,
		},
		ContainerRule: pkg.ContainerRule{
			ExcludeRules: excludeRules,
		},
	}
}

type ReadinessRuleNameRule struct {
	pkg.RuleMeta
	pkg.ContainerRule
}
