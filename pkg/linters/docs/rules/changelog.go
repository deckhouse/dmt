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
	ChangelogRuleName   = "changelog"
	changelogFilename   = "CHANGELOG.md"
	changelogRuFilename = "CHANGELOG_RU.md"
)

func NewChangelogRule() *ChangelogRule {
	return &ChangelogRule{
		RuleMeta: pkg.RuleMeta{
			Name: ChangelogRuleName,
		},
	}
}

type ChangelogRule struct {
	pkg.RuleMeta
}

func (r *ChangelogRule) CheckChangelog(m pkg.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	changelogPath := filepath.Join(m.GetPath(), changelogFilename)
	changelogExists, changelogEmpty := checkFile(changelogPath)

	changelogRuPath := filepath.Join(m.GetPath(), changelogRuFilename)
	changelogRuExists, changelogRuEmpty := checkFile(changelogRuPath)

	if !changelogExists {
		errorList.WithFilePath(changelogFilename).Error("CHANGELOG.md file is missing")
	} else if changelogEmpty {
		errorList.WithFilePath(changelogFilename).Error("CHANGELOG.md file is empty")
	}

	if !changelogRuExists {
		errorList.WithFilePath(changelogRuFilename).Error("CHANGELOG_RU.md file is missing")
	} else if changelogRuEmpty {
		errorList.WithFilePath(changelogRuFilename).Error("CHANGELOG_RU.md file is empty")
	}
}

func checkFile(filePath string) (exists bool, empty bool) {
	stat, err := os.Stat(filePath)
	if errs.Is(err, os.ErrNotExist) {
		return false, true
	}
	if err != nil {
		// If we can't stat the file, consider it as non-existing
		return false, true
	}
	return true, stat.Size() == 0
}
