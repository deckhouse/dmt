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
	"bytes"
	errs "errors"
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	EnabledScriptRuleName = "enabled-script"
	enabledScriptFilename = "enabled"
)

func NewEnabledScriptRule() *EnabledScriptRule {
	return &EnabledScriptRule{
		RuleMeta: pkg.RuleMeta{
			Name: EnabledScriptRuleName,
		},
	}
}

type EnabledScriptRule struct {
	pkg.RuleMeta
}

// CheckEnabledScript warns when a module ships a non-empty `enabled` script.
// The enabled-script mechanism is deprecated and should be removed.
func (r *EnabledScriptRule) CheckEnabledScript(modulePath string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName()).WithFilePath(enabledScriptFilename)

	data, err := os.ReadFile(filepath.Join(modulePath, enabledScriptFilename))
	if errs.Is(err, os.ErrNotExist) {
		return
	}

	if err != nil {
		errorList.Errorf("Cannot read file %q: %s", enabledScriptFilename, err)
		return
	}

	// An empty (or whitespace-only) script does not activate the mechanism, so it
	// is not worth flagging.
	if len(bytes.TrimSpace(data)) == 0 {
		return
	}

	errorList.Warn("The enabled-script mechanism is deprecated and must be removed.")
}
