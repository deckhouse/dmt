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
	NoNewPrivilegesRuleName = "no-new-privileges"
)

func NewNoNewPrivilegesRule(excludeRules []pkg.ContainerRuleExclude) *NoNewPrivilegesRule {
	return &NoNewPrivilegesRule{
		RuleMeta: pkg.RuleMeta{
			Name: NoNewPrivilegesRuleName,
		},
		ContainerRule: pkg.ContainerRule{
			ExcludeRules: excludeRules,
		},
	}
}

type NoNewPrivilegesRule struct {
	pkg.RuleMeta
	pkg.ContainerRule
}

// ContainerNoNewPrivileges checks that containers have allowPrivilegeEscalation set to false
// This prevents privilege escalation attacks by ensuring containers cannot gain additional privileges
// Reference: CIS Kubernetes Benchmark 5.2.5
func (r *NoNewPrivilegesRule) ContainerNoNewPrivileges(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
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

		if c.SecurityContext == nil {
			errorList.WithObjectID(object.Identity() + " ; container = " + c.Name).
				Error("Container's SecurityContext is missing - cannot verify allowPrivilegeEscalation setting")
			continue
		}

		if c.SecurityContext.AllowPrivilegeEscalation == nil {
			errorList.WithObjectID(object.Identity() + " ; container = " + c.Name).
				Error("Container's SecurityContext missing parameter AllowPrivilegeEscalation - should be set to false to prevent privilege escalation")
			continue
		}

		if *c.SecurityContext.AllowPrivilegeEscalation {
			errorList.WithObjectID(object.Identity() + " ; container = " + c.Name).
				Error("Container's SecurityContext has `AllowPrivilegeEscalation: true`, but it must be `false` to prevent privilege escalation attacks")
		}
	}
}
