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
	"os"
	"regexp"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	LicenseRuleName = "license"
)

var fileToCheckRe = regexp.MustCompile(
	`\.go$|/[^.]+$|\.sh$|\.lua$|\.py$`,
)

var fileToSkipRe = regexp.MustCompile(
	`geohash.lua$|\.github/.*|Dockerfile$|Makefile$|/docs/documentation/|/docs/site/|bashrc$|inputrc$` +
		`|modules_menu_skip$|LICENSE$|tools/spelling/.+|/lib/python/|charts/helm_lib|PROJECT|pb.go$` +
		`|zz_generated.go$|zz_generated.*\.go$|zz_generated.*\.yaml$`,
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

	// Use new parser if available
	parser := NewLicenseParser()

	files := fsutils.GetFiles(mod.GetPath(), false, filterFiles)
	for _, fileName := range files {
		name := fsutils.Rel(mod.GetPath(), fileName)

		if !r.Enabled(name) {
			// TODO: add metrics
			continue
		}

		licenseInfo, parseErr := parser.ParseFile(fileName)
		if parseErr != nil {
			errorList.WithFilePath(name).Error(parseErr.Error())
			continue
		}

		// Handle parsed license info
		if !licenseInfo.Valid {
			errorList.WithFilePath(name).Error(licenseInfo.Error)
		}
	}
}

func filterFiles(rootPath, path string) bool {
	f, err := os.Stat(path)
	if err != nil {
		logger.DebugF("Error getting file info: %v", err)
		return false
	}
	if f.IsDir() {
		return false
	}
	path = fsutils.Rel(rootPath, path)
	if fileToCheckRe.MatchString(path) && !fileToSkipRe.MatchString(path) {
		return true
	}

	return false
}
