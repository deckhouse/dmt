/*
Copyright 2021 Flant JSC

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

package matrix

import (
	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/internal/storage"
	"github.com/deckhouse/d8-lint/pkg/errors"
	"github.com/deckhouse/d8-lint/pkg/linters/matrix/rules"
	"github.com/deckhouse/d8-lint/pkg/linters/matrix/rules/resources"
)

func ApplyLintRules(md *module.Module, objectStore *storage.UnstructuredObjectStore) *errors.LintRuleErrorsList {
	linter := rules.ObjectLinter{
		ObjectStore: objectStore,
		Module:      md,
		ErrorsList:  &errors.LintRuleErrorsList{},
	}

	for _, object := range objectStore.Storage {
		linter.ApplyObjectRules(object)
		linter.ApplyContainerRules(object)
	}

	resources.ControllerMustHaveVPA(&linter)
	resources.ControllerMustHavePDB(&linter)
	resources.DaemonSetMustNotHavePDB(&linter)
	resources.NamespaceMustContainKubeRBACProxyCA(&linter)

	return linter.ErrorsList
}
