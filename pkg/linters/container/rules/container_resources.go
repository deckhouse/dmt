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
	errorList = errorList.WithRule(r.GetName()).WithFilePath(object.ShortPath())

	for i := range containers {
		c := &containers[i]

		errorList = errorList.WithEnabled(r.Enabled(object, c))

		if c.Resources.Requests.StorageEphemeral() == nil || c.Resources.Requests.StorageEphemeral().Value() == 0 {
			errorList.WithObjectID(object.Identity() + "; container = " + c.Name).
				Error("Ephemeral storage for container is not defined in Resources.Requests")
		}
	}
}
