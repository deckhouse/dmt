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
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	FilesRuleName = "files"

	// maxCheckableFileSize bounds how large a file the Cyrillic check will read
	// into memory. Files above it are generated data blobs (bundled Grafana
	// dashboards, rendered openapi, CRD bundles), not hand-written sources;
	// reading a multi-gigabyte file just to scan for Cyrillic letters would
	// exhaust memory, so such files are skipped.
	maxCheckableFileSize = 10 << 20 // 10 MiB

	// maxCyrillicReportLines bounds how many offending lines a single finding
	// echoes back, and maxCyrillicLineWidth bounds the width of each. Together
	// they keep a file full of Cyrillic from dumping megabytes into the log.
	maxCyrillicReportLines = 100
	maxCyrillicLineWidth   = 200
)

var (
	skipDocRe  = `doc-ru-.+\.y[a]?ml$|\.ru\.y[a]?ml$|\.ru\.json$|\.ru\.md$|\.ru\.html$|_RU\.md$|_ru\.html$|docs/site/_.+|docs/documentation/_.+|tools/spelling/.+|openapi/conversions/.+|module.yaml|ru\..+`
	skipSelfRe = `no_cyrillic(_test)?.go$`
	skipI18NRe = `/i18n/`
)

func NewFilesRule(excludeFileRules []pkg.StringRuleExclude,
	excludeDirectoryRules []pkg.DirectoryRuleExclude) *FilesRule {
	return &FilesRule{
		RuleMeta: pkg.RuleMeta{
			Name: FilesRuleName,
		},
		PathRule: pkg.PathRule{
			ExcludeStringRules:    excludeFileRules,
			ExcludeDirectoryRules: excludeDirectoryRules,
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

func (r *FilesRule) CheckFile(m pkg.Module, fileName string, errorList *errors.LintRuleErrorsList) {
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

	info, err := os.Stat(fileName)
	if err != nil {
		errorList.Error(err.Error())

		return
	}

	// Files too large to be hand-written sources are not scanned: reading a
	// multi-gigabyte generated file into memory (and echoing its Cyrillic lines
	// into a finding) would blow up memory and flood the log. Report it as a
	// warning instead of failing. This is gated by r.Enabled(fName) above, so a
	// user can silence it by excluding the file or its directory in the
	// no-cyrillic exclude rules.
	if info.Size() > maxCheckableFileSize {
		errorList.WithFilePath(fName).
			Warnf("file is too large (%d bytes) to check for Cyrillic letters and was skipped; exclude the file or its directory in the no-cyrillic rules to silence this warning", info.Size())

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
