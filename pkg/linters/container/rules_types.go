package container

import (
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/config"
	corev1 "k8s.io/api/core/v1"
)

const (
	CheckReadOnlyRootFilesystemRuleName = "read-only-root-filesystem"
)

func NewCheckReadOnlyRootFilesystemRule(excludeRules []config.ContainerRuleExclude) *CheckReadOnlyRootFilesystemRule {
	return &CheckReadOnlyRootFilesystemRule{
		name:         CheckReadOnlyRootFilesystemRuleName,
		excludeRules: excludeRules,
	}
}

type CheckReadOnlyRootFilesystemRule struct {
	name string

	excludeRules []config.ContainerRuleExclude
}

func (r *CheckReadOnlyRootFilesystemRule) Name() string {
	return r.name
}

func (r *CheckReadOnlyRootFilesystemRule) Enabled(object storage.StoreObject, container *corev1.Container) bool {
	for _, rule := range r.excludeRules {
		if rule.Kind == object.Unstructured.GetKind() &&
			rule.Name == object.Unstructured.GetName() &&
			(rule.Container == "" || rule.Container == container.Name) {
			return false
		}
	}

	return true
}
