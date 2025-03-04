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
	"io"
	"os"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	LicenseRuleName = "license"
)

func NewLicenseRule(excludeFilesRules []pkg.StringRuleExclude,
	excludeDirectoryRules []pkg.PrefixRuleExclude) *LicenseRule {
	return &LicenseRule{
		RuleMeta: pkg.RuleMeta{
			Name: LicenseRuleName,
		},
		PathRule: pkg.PathRule{
			ExcludeStringRules: excludeFilesRules,
			ExcludePrefixRules: excludeDirectoryRules,
		},
	}
}

type LicenseRule struct {
	pkg.RuleMeta
	pkg.PathRule
}

func (r *LicenseRule) CheckFiles(mod *module.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	files := fsutils.GetFiles(mod.GetPath(), false, filterFiles)
	for _, fileName := range files {
		name := fsutils.Rel(mod.GetPath(), fileName)

		errorList = errorList.WithEnabled(r.Enabled(name))

		ok, err := checkFileCopyright(fileName)
		if !ok {
			if errs.Is(err, io.EOF) {
				// skip totally empty file
				continue
			}
			errorList.WithFilePath(name).Error(err.Error())
		}
	}
}

func filterFiles(path string) bool {
	f, err := os.Stat(path)
	if err != nil {
		logger.DebugF("Error getting file info: %v", err)
		return false
	}
	if f.IsDir() {
		return false
	}
	if fileToCheckRe.MatchString(path) && !fileToSkipRe.MatchString(path) {
		return true
	}

	return false
}
