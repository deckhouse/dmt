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
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	FilesRuleName = "files"
)

var (
	skipDocRe  = `doc-ru-.+\.y[a]?ml$|_RU\.md$|_ru\.html$|docs/site/_.+|docs/documentation/_.+|tools/spelling/.+|openapi/conversions/.+`
	skipSelfRe = `no_cyrillic(_test)?.go$`
	skipI18NRe = `/i18n/`
)

func NewFilesRule(excludeFileRules []pkg.StringRuleExclude,
	excludeDirectoryRules []pkg.PrefixRuleExclude) *FilesRule {
	return &FilesRule{
		RuleMeta: pkg.RuleMeta{
			Name: FilesRuleName,
		},
		PathRule: pkg.PathRule{
			ExcludeStringRules: excludeFileRules,
			ExcludePrefixRules: excludeDirectoryRules,
		},
		skipDocRe:  regexp.MustCompile(skipDocRe),
		skipI18NRe: regexp.MustCompile(skipSelfRe),
		skipSelfRe: regexp.MustCompile(skipI18NRe),
	}
}

type FilesRule struct {
	pkg.RuleMeta
	pkg.PathRule

	skipDocRe  *regexp.Regexp
	skipI18NRe *regexp.Regexp
	skipSelfRe *regexp.Regexp
}

func (r *FilesRule) CheckFile(m *module.Module, fileName string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	fName := fsutils.Rel(m.GetPath(), fileName)
	if !r.Enabled(fName) {
		// TODO: add metrics
		return
	}

	if r.skipDocRe.MatchString(fileName) {
		return
	}

	if r.skipI18NRe.MatchString(fileName) {
		return
	}

	if r.skipSelfRe.MatchString(fileName) {
		return
	}

	lines, err := getFileContent(fileName)
	if err != nil {
		errorList.Error(err.Error())

		return
	}

	cyrMsg, hasCyr := checkCyrillicLettersInArray(lines)
	if hasCyr {
		errorList.WithFilePath(fName).WithValue(cyrMsg).
			Error("has cyrillic letters")
	}
}

func getFileContent(filename string) ([]string, error) {
	fileBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	sliceData := strings.Split(string(fileBytes), "\n")

	return sliceData, nil
}
