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

package rules

import (
	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/internal/storage"
	"github.com/deckhouse/d8-lint/pkg/errors"
	"github.com/deckhouse/d8-lint/pkg/linters/matrix/rules/roles"
)

type ObjectLinter struct {
	ObjectStore *storage.UnstructuredObjectStore
	ErrorsList  *errors.LintRuleErrorsList
	Module      *module.Module
}

func (l *ObjectLinter) ApplyObjectRules(object storage.StoreObject) {
	l.ErrorsList.Add(roles.ObjectUserAuthzClusterRolePath(l.Module, object))
	l.ErrorsList.Add(roles.ObjectRBACPlacement(l.Module, object))
	l.ErrorsList.Add(roles.ObjectBindingSubjectServiceAccountCheck(l.Module, object, l.ObjectStore))
	l.ErrorsList.Add(roles.ObjectRolesWildcard(object))
}
