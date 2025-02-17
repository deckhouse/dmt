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
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ImageDigestRuleName = "image-digest"
)

const defaultRegistry = "registry.example.com/deckhouse"

func NewImageDigestRule(excludeRules []pkg.ContainerRuleExclude) *ImageDigestRule {
	return &ImageDigestRule{
		RuleMeta: pkg.RuleMeta{
			Name: ImageDigestRuleName,
		},
		ContainerRule: pkg.ContainerRule{
			ExcludeRules: excludeRules,
		},
	}
}

type ImageDigestRule struct {
	pkg.RuleMeta
	pkg.ContainerRule
}

func (r *ImageDigestRule) ContainerImageDigestCheck(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	for i := range containers {
		c := &containers[i]

		if !r.Enabled(object, c) {
			// TODO: add metrics
			continue
		}

		match := strings.Split(c.Image, "@")
		if len(match) == 0 {
			errorList.WithObjectID(object.Identity() + "; container = " + c.Name).
				Error("Cannot parse repository from image")

			continue
		}

		repo, err := name.NewRepository(match[0])
		if err != nil {
			errorList.WithObjectID(object.Identity()+"; container = "+c.Name).
				Errorf("Cannot parse repository from image: %s", c.Image)

			continue
		}

		if repo.Name() != defaultRegistry {
			errorList.WithObjectID(object.Identity()+"; container = "+c.Name).
				Errorf("All images must be deployed from the same default registry: %s current: %s", defaultRegistry, repo.RepositoryStr())

			continue
		}
	}
}

func (r *ImageDigestRule) Enabled(object storage.StoreObject, container *corev1.Container) bool {
	for _, rule := range r.ExcludeRules {
		if !rule.Enabled(object, container) {
			return false
		}
	}

	return true
}
