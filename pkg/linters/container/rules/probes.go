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
	"github.com/deckhouse/dmt/pkg/linters/utils"
)

const (
	LivenessRuleName  = "liveness-probe"
	ReadinessRuleName = "readiness-probe"
)

// ProbeRule defines interface for probe rules
type ProbeRule interface {
	GetName() string
	Enabled(object storage.StoreObject, container *v1.Container) bool
	GetProbe(container *v1.Container) *v1.Probe
}

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

// GetProbe returns the liveness probe for the container
func (*LivenessRule) GetProbe(container *v1.Container) *v1.Probe {
	return container.LivenessProbe
}

// GetProbe returns the readiness probe for the container
func (*ReadinessRuleNameRule) GetProbe(container *v1.Container) *v1.Probe {
	return container.ReadinessProbe
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
func (r *LivenessRule) CheckProbe(object storage.StoreObject, containers []v1.Container, errorList *errors.LintRuleErrorsList) {
	checkProbeGeneric(r, object, containers, errorList, "liveness-probe")
}

// check readinessProbe exist and correct
func (r *ReadinessRuleNameRule) CheckProbe(object storage.StoreObject, containers []v1.Container, errorList *errors.LintRuleErrorsList) {
	checkProbeGeneric(r, object, containers, errorList, "readiness-probe")
}

// checkProbeGeneric is a generic function to check probe existence and validity
func checkProbeGeneric(rule ProbeRule, object storage.StoreObject, containers []v1.Container, errorList *errors.LintRuleErrorsList, probeType string) {
	errorList = errorList.WithRule(rule.GetName()).WithFilePath(object.ShortPath())

	if !utils.IsPodController(object.Unstructured.GetKind()) {
		return
	}

	for idx := range containers {
		c := &containers[idx]

		if !rule.Enabled(object, c) {
			// TODO: add metrics
			continue
		}

		errorList = errorList.WithObjectID(object.Identity() + " ; container = " + c.Name).WithFilePath(object.ShortPath())

		probe := rule.GetProbe(c)
		if probe == nil {
			errorList.Errorf("Container does not contain %s", probeType)
			continue
		}

		if probeHandlerIsNotValid(probe.ProbeHandler) {
			errorList.Errorf("Container does not use correct %s", probeType)
		}
	}
}
