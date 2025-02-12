package container

import (
	"github.com/deckhouse/dmt/pkg"
)

const (
	CheckReadOnlyRootFilesystemRuleName = "read-only-root-filesystem"
	SecurityContextRuleName             = "security-context"
	DNSPolicyRuleName                   = "dns-policy"
	ServicePortRuleName                 = "service-port"
	ResourcesRuleName                   = "resources"
)

func NewCheckReadOnlyRootFilesystemRule(excludeRules []pkg.ContainerRuleExclude) *CheckReadOnlyRootFilesystemRule {
	return &CheckReadOnlyRootFilesystemRule{
		RuleMeta: pkg.RuleMeta{
			Name: CheckReadOnlyRootFilesystemRuleName,
		},
		ContainerRule: pkg.ContainerRule{
			ExcludeRules: excludeRules,
		},
	}
}

type CheckReadOnlyRootFilesystemRule struct {
	pkg.RuleMeta
	pkg.ContainerRule
}

func NewSecurityContextRule(excludeRules []pkg.ContainerRuleExclude) *SecurityContextRule {
	return &SecurityContextRule{
		RuleMeta: pkg.RuleMeta{
			Name: SecurityContextRuleName,
		},
		ContainerRule: pkg.ContainerRule{
			ExcludeRules: excludeRules,
		},
	}
}

type SecurityContextRule struct {
	pkg.RuleMeta
	pkg.ContainerRule
}

func NewResourcesRule(excludeRules []pkg.ContainerRuleExclude) *ResourcesRule {
	return &ResourcesRule{
		RuleMeta: pkg.RuleMeta{
			Name: ResourcesRuleName,
		},
		ContainerRule: pkg.ContainerRule{
			ExcludeRules: excludeRules,
		},
	}
}

type ResourcesRule struct {
	pkg.RuleMeta
	pkg.ContainerRule
}

func NewDNSPolicyRule(excludeRules []pkg.KindRuleExclude) *DNSPolicyRule {
	return &DNSPolicyRule{
		RuleMeta: pkg.RuleMeta{
			Name: DNSPolicyRuleName,
		},
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
	}
}

type DNSPolicyRule struct {
	pkg.RuleMeta
	pkg.KindRule
}

func NewServicePortRule(excludeRules []pkg.StringRuleExclude) *ServicePortRule {
	return &ServicePortRule{
		RuleMeta: pkg.RuleMeta{
			Name: ServicePortRuleName,
		},
		StringRule: pkg.StringRule{
			ExcludeRules: excludeRules,
		},
	}
}

type ServicePortRule struct {
	pkg.RuleMeta
	pkg.StringRule
}
