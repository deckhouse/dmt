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
	errs "errors"
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	LegacyReleaseFileRuleName = "legacy-release-file"
	legacyReleaseFilename     = "release.yaml"
)

func NewLegacyReleaseFileRule() *LegacyReleaseFileRule {
	return &LegacyReleaseFileRule{
		RuleMeta: pkg.RuleMeta{
			Name: LegacyReleaseFileRuleName,
		},
	}
}

type LegacyReleaseFileRule struct {
	pkg.RuleMeta
}

func (r *LegacyReleaseFileRule) CheckLegacyReleaseFile(modulePath string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName()).WithFilePath(legacyReleaseFilename)

	_, err := os.Stat(filepath.Join(modulePath, legacyReleaseFilename))
	if errs.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		errorList.Errorf("Cannot stat file %q: %s", legacyReleaseFilename, err)
		return
	}

	errorList.Error("Remove release.yaml. Version need to be defined in 'version.json'.")
}
