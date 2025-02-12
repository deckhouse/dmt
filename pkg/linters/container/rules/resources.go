package rules

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ResourcesRuleName = "resources"
)

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

func (r *ResourcesRule) ContainerStorageEphemeral(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.Name)

	for i := range containers {
		c := &containers[i]

		if !r.Enabled(object, c) {
			// TODO: add metrics
			continue
		}

		if c.Resources.Requests.StorageEphemeral() == nil || c.Resources.Requests.StorageEphemeral().Value() == 0 {
			errorList.WithObjectID(object.Identity() + "; container = " + c.Name).
				Error("Ephemeral storage for container is not defined in Resources.Requests")
		}
	}
}
