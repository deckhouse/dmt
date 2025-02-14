package rules

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	CheckReadOnlyRootFilesystemRuleName = "read-only-root-filesystem"
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

func (r *CheckReadOnlyRootFilesystemRule) ObjectReadOnlyRootFilesystem(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName()).WithFilePath(object.ShortPath())

	switch object.Unstructured.GetKind() {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
	default:
		return
	}

	for i := range containers {
		c := &containers[i]

		if !r.Enabled(object, c) {
			// TODO: add metrics
			continue
		}

		if c.VolumeMounts == nil {
			continue
		}

		if c.SecurityContext == nil {
			errorList.WithObjectID(object.Identity()).
				Error("Container's SecurityContext is missing")

			continue
		}

		if c.SecurityContext.ReadOnlyRootFilesystem == nil {
			errorList.WithObjectID(object.Identity() + " ; container = " + c.Name).
				Error("Container's SecurityContext missing parameter ReadOnlyRootFilesystem")

			continue
		}

		if !*c.SecurityContext.ReadOnlyRootFilesystem {
			errorList.WithObjectID(object.Identity() + " ; container = " + c.Name).
				Error("Container's SecurityContext has `ReadOnlyRootFilesystem: false`, but it must be `true`")
		}
	}
}
