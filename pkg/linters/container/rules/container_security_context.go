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
	ContainerSecurityContextRuleName = "security-context"
)

func NewContainerSecurityContextRule(excludeRules []pkg.ContainerRuleExclude,
	objects []ObjectContainers, errorList *errors.LintRuleErrorsList) *ContainerSecurityContextRule {
	return &ContainerSecurityContextRule{
		RuleMeta: pkg.RuleMeta{
			Name: ContainerSecurityContextRuleName,
		},
		ContainerRule: pkg.ContainerRule{
			ExcludeRules: excludeRules,
		},
		objects:   objects,
		errorList: errorList.WithRule(ContainerSecurityContextRuleName),
	}
}

type ContainerSecurityContextRule struct {
	pkg.RuleMeta
	pkg.ContainerRule

	objects   []ObjectContainers
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*ContainerSecurityContextRule)(nil)

func (r *ContainerSecurityContextRule) Check(_ context.Context) {
	for _, oc := range r.objects {
		r.checkObject(oc.Object, oc.All)
	}
}

// checkObject must stay a separate method rather than being inlined into the
// loop above: its early returns end the check for one object, not for the rule.
func (r *ContainerSecurityContextRule) checkObject(object storage.StoreObject, containers []corev1.Container) {
	errorList := r.errorList.WithFilePath(object.GetPath())

	for i := range containers {
		c := &containers[i]

		if !r.Enabled(object, c) {
			// TODO: add metrics
			continue
		}

		if c.SecurityContext == nil {
			errorList.WithObjectID(object.Identity() + "; container = " + c.Name).
				Error("Container ContainerSecurityContext is not defined")

			continue
		}
	}
}
