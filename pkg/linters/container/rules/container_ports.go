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
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	PortsRuleName = "ports"
)

func NewPortsRule(excludeRules []pkg.ContainerRuleExclude) *PortsRule {
	return &PortsRule{
		RuleMeta: pkg.RuleMeta{
			Name: PortsRuleName,
		},
		ContainerRule: pkg.ContainerRule{
			ExcludeRules: excludeRules,
		},
	}
}

type PortsRule struct {
	pkg.RuleMeta
	pkg.ContainerRule
}

func (r *PortsRule) ContainerPorts(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	const t = 1024
	for i := range containers {
		c := &containers[i]

		if !r.Enabled(object, c) {
			// TODO: add metrics
			continue
		}

		for _, port := range c.Ports {
			if port.ContainerPort <= t {
				errorList.WithObjectID(object.Identity() + "; container = " + c.Name).WithValue(port.ContainerPort).
					Error("Container uses port <= 1024")

				continue
			}
		}
	}
}

func (r *PortsRule) Enabled(object storage.StoreObject, container *corev1.Container) bool {
	for _, rule := range r.ExcludeRules {
		if !rule.Enabled(object, container) {
			return false
		}
	}

	return true
}
