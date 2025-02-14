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
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

const (
	NameDuplicatesRuleName = "name-duplicates"
)

func NewNameDuplicatesRule() *NameDuplicatesRule {
	return &NameDuplicatesRule{
		RuleMeta: pkg.RuleMeta{
			Name: NameDuplicatesRuleName,
		},
	}
}

type NameDuplicatesRule struct {
	pkg.RuleMeta
}

func (r *NameDuplicatesRule) ContainerNameDuplicates(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	if hasDuplicates(containers, func(c corev1.Container) string { return c.Name }) {
		errorList.WithObjectID(object.Identity()).
			Error("Duplicate container name")
	}
}

func hasDuplicates[T any](items []T, keyFunc func(T) string) bool {
	seen := make(map[string]struct{})
	for _, item := range items {
		key := keyFunc(item)
		if _, ok := seen[key]; ok {
			return true
		}
		seen[key] = struct{}{}
	}
	return false
}
