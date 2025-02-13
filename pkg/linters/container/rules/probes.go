package rules

import (
	v1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
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

func probeHandlerIsNotValid(probe v1.ProbeHandler) bool {
	var count int8
	if probe.Exec != nil {
		count++
	}
	if probe.GRPC != nil {
		count++
	}
	if probe.HTTPGet != nil {
		count++
	}
	if probe.TCPSocket != nil {
		count++
	}
	if count != 1 {
		return true
	}

	return false
}

// check livenessProbe exist and correct
func (r *LivenessRule) CheckProbe(object storage.StoreObject, containers []v1.Container, errorList *errors.LintRuleErrorsList) { //nolint: dupl // we have doubled code in probes because it's separate rules and we need to edit them separate
	errorList = errorList.WithRule(r.GetName())

	for idx := range containers {
		c := &containers[idx]

		if !r.Enabled(object, c) {
			// TODO: add metrics
			return
		}

		errorList = errorList.WithObjectID(object.Identity() + " ; container = " + c.Name)

		livenessProbe := c.LivenessProbe
		if livenessProbe == nil {
			errorList.Error("Container does not contains liveness-probe")

			return
		}

		if probeHandlerIsNotValid(livenessProbe.ProbeHandler) {
			errorList.Error("Container does not use correct liveness-probe")
		}
	}
}

// check readinessProbe exist and correct
func (r *ReadinessRuleNameRule) CheckProbe(object storage.StoreObject, containers []v1.Container, errorList *errors.LintRuleErrorsList) { //nolint: dupl // we have doubled code in probes because it's separate rules and we need to edit them separate
	errorList = errorList.WithRule(r.GetName())

	for idx := range containers {
		c := &containers[idx]

		if !r.Enabled(object, c) {
			// TODO: add metrics
			return
		}

		errorList = errorList.WithObjectID(object.Identity() + " ; container = " + c.Name)

		readinessProbe := c.ReadinessProbe
		if readinessProbe == nil {
			errorList.Error("Container does not contains readiness-probe")

			return
		}

		if probeHandlerIsNotValid(readinessProbe.ProbeHandler) {
			errorList.Error("Container does not use correct readiness-probe")
		}
	}
}
