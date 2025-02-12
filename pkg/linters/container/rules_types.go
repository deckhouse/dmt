package container

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
)

const (
	CheckReadOnlyRootFilesystemRuleName = "read-only-root-filesystem"
	SecurityContextRuleName             = "security-context"
)

func NewCheckReadOnlyRootFilesystemRule(excludeRules []pkg.ContainerRuleExclude) *CheckReadOnlyRootFilesystemRule {
	return &CheckReadOnlyRootFilesystemRule{
		ContainerRule: ContainerRule{
			name:         CheckReadOnlyRootFilesystemRuleName,
			excludeRules: excludeRules,
		},
	}
}

type CheckReadOnlyRootFilesystemRule struct {
	ContainerRule
}

func NewSecurityContextRule(excludeRules []pkg.ContainerRuleExclude) *SecurityContextRule {
	return &SecurityContextRule{
		ContainerRule: ContainerRule{
			name:         SecurityContextRuleName,
			excludeRules: excludeRules,
		},
	}
}

type SecurityContextRule struct {
	ContainerRule
}

type ContainerRule struct {
	name string

	excludeRules []pkg.ContainerRuleExclude
}

func (r *ContainerRule) Name() string {
	return r.name
}

func (r *ContainerRule) Enabled(object storage.StoreObject, container *corev1.Container) bool {
	for _, rule := range r.excludeRules {
		return rule.Enabled(object, container)
	}

	return true
}
