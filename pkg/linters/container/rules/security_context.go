package rules

import (
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

const (
	SecurityContextRuleName = "security-context"
)

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

func (r *SecurityContextRule) ContainerSecurityContext(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	for i := range containers {
		c := &containers[i]

		if !r.Enabled(object, c) {
			// TODO: add metrics
			continue
		}

		if c.SecurityContext == nil {
			errorList.WithObjectID(object.Identity() + "; container = " + c.Name).
				Error("Container SecurityContext is not defined")

			return
		}
	}
}
