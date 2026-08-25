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
	"github.com/deckhouse/dmt/pkg/errors"
)

// ObjectContainers is a rendered object together with the containers extracted
// from it. The linter builds these once, so the extraction — and the diagnostic
// it can produce — happens exactly once per object rather than once per rule.
type ObjectContainers struct {
	Object storage.StoreObject

	// All comes from GetAllContainers and is never empty: objects without
	// containers are dropped, which is what stops container rules from running
	// on them.
	All []corev1.Container

	// NotInit comes from GetContainers and is empty when the object has no
	// non-init containers, which is what stops the probe rules from running.
	NotInit []corev1.Container
}

// CollectObjectContainers extracts containers from every object in storageMap,
// reproducing the gates the container linter has always applied: an object
// whose containers cannot be extracted, or which has none, takes no further
// part. The extraction failure is reported here — once per object, with no rule
// ID — exactly as the old dispatcher did.
func CollectObjectContainers(
	storageMap map[storage.ResourceIndex]storage.StoreObject,
	errorList *errors.LintRuleErrorsList,
) []ObjectContainers {
	result := make([]ObjectContainers, 0, len(storageMap))

	for _, object := range storageMap {
		objectErrorList := errorList.WithFilePath(object.GetPath())

		all, err := object.GetAllContainers()
		if err != nil {
			objectErrorList.WithObjectID(object.Identity()).
				Errorf("Cannot get containers from object: %s", err)

			continue
		}

		if len(all) == 0 {
			continue
		}

		item := ObjectContainers{Object: object, All: all}

		// Unreachable in practice — GetAllContainers calls GetContainers — but
		// kept so the diagnostic behaviour matches the old dispatcher exactly.
		notInit, err := object.GetContainers()
		if err != nil {
			objectErrorList.WithObjectID(object.Identity()).
				Errorf("Cannot get containers from object: %s", err)
		} else {
			item.NotInit = notInit
		}

		result = append(result, item)
	}

	return result
}
