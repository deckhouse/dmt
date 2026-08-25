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
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	HostNetworkPortsRuleName = "host-network-ports"
)

func NewHostNetworkPortsRule(excludeRules []pkg.ContainerRuleExclude,
	objects []ObjectContainers, errorList *errors.LintRuleErrorsList) *HostNetworkPortsRule {
	return &HostNetworkPortsRule{
		RuleMeta: pkg.RuleMeta{
			Name: HostNetworkPortsRuleName,
		},
		ContainerRule: pkg.ContainerRule{
			ExcludeRules: excludeRules,
		},
		objects:   objects,
		errorList: errorList.WithRule(HostNetworkPortsRuleName),
	}
}

type HostNetworkPortsRule struct {
	pkg.RuleMeta
	pkg.ContainerRule

	objects   []ObjectContainers
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*HostNetworkPortsRule)(nil)

func (r *HostNetworkPortsRule) Check(_ context.Context) {
	for _, oc := range r.objects {
		r.checkObject(oc.Object, oc.All)
	}
}

// checkObject must stay a separate method rather than being inlined into the
// loop above: its early returns end the check for one object, not for the rule.
func (r *HostNetworkPortsRule) checkObject(object storage.StoreObject, containers []corev1.Container) {
	errorList := r.errorList.WithFilePath(object.GetPath())

	switch object.Unstructured.GetKind() {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
	default:
		return
	}

	hostNetworkUsed, err := object.IsHostNetwork()
	if err != nil {
		errorList.WithObjectID(object.Identity()).
			Errorf("IsHostNetwork failed: %v", err)

		return
	}

	for i := range containers {
		c := &containers[i]

		if !r.Enabled(object, c) {
			// TODO: add metrics
			continue
		}

		for _, port := range c.Ports {
			if hostNetworkUsed && (port.ContainerPort < 4200 || port.ContainerPort >= 4300) {
				errorList.WithObjectID(object.Identity() + " ; container = " + c.Name).WithValue(port.ContainerPort).
					Error("Pod running in hostNetwork and it's container port doesn't fit the range [4200,4299]")
			}

			if port.HostPort != 0 && (port.HostPort < 4200 || port.HostPort >= 4300) {
				errorList.WithObjectID(object.Identity() + " ; container = " + c.Name).WithValue(port.HostPort).
					Error("Container uses hostPort that doesn't fit the range [4200,4299]")
			}
		}
	}
}
