/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rules

import (
	v1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	LivenessRuleName  = "liveness-probe"
	ReadinessRuleName = "readiness-probe"
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
	errorList = errorList.WithRule(r.GetName()).WithFilePath(object.ShortPath())

	for idx := range containers {
		c := &containers[idx]

		if !r.Enabled(object, c) {
			// TODO: add metrics
			continue
		}

		errorList = errorList.WithObjectID(object.Identity() + " ; container = " + c.Name).WithFilePath(object.ShortPath())

		livenessProbe := c.LivenessProbe
		if livenessProbe == nil {
			errorList.Error("Container does not contain liveness-probe")

			return
		}

		if probeHandlerIsNotValid(livenessProbe.ProbeHandler) {
			errorList.Error("Container does not use correct liveness-probe")
		}
	}
}

// check readinessProbe exist and correct
func (r *ReadinessRuleNameRule) CheckProbe(object storage.StoreObject, containers []v1.Container, errorList *errors.LintRuleErrorsList) { //nolint: dupl // we have doubled code in probes because it's separate rules and we need to edit them separate
	errorList = errorList.WithRule(r.GetName()).WithFilePath(object.ShortPath())

	for idx := range containers {
		c := &containers[idx]

		if !r.Enabled(object, c) {
			// TODO: add metrics
			continue
		}

		errorList = errorList.WithObjectID(object.Identity() + " ; container = " + c.Name).WithFilePath(object.ShortPath())

		readinessProbe := c.ReadinessProbe
		if readinessProbe == nil {
			errorList.Error("Container does not contain readiness-probe")

			return
		}

		if probeHandlerIsNotValid(readinessProbe.ProbeHandler) {
			errorList.Error("Container does not use correct readiness-probe")
		}
	}
}
