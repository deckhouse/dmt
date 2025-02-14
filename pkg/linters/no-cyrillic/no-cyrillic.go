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

package nocyrillic

import (
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ID = "no-cyrillic"
)

// NoCyrillic linter
type NoCyrillic struct {
	name, desc string
	cfg        *config.NoCyrillicSettings
	skipDocRe  *regexp.Regexp
	skipI18NRe *regexp.Regexp
	skipSelfRe *regexp.Regexp
	ErrorList  *errors.LintRuleErrorsList
}

var (
	fileExtensions = []string{"yaml", "yml", "json", "go"}
	skipDocRe      = `doc-ru-.+\.y[a]?ml$|_RU\.md$|_ru\.html$|docs/site/_.+|docs/documentation/_.+|tools/spelling/.+|openapi/conversions/.+`
	skipSelfRe     = `no_cyrillic(_test)?.go$`
	skipI18NRe     = `/i18n/`
)

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *NoCyrillic {
	return &NoCyrillic{
		name:       ID,
		desc:       "NoCyrillic will check all files in the modules for contains cyrillic symbols",
		skipDocRe:  regexp.MustCompile(skipDocRe),
		skipI18NRe: regexp.MustCompile(skipSelfRe),
		skipSelfRe: regexp.MustCompile(skipI18NRe),
		cfg:        &cfg.LintersSettings.NoCyrillic,
		ErrorList:  errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.NoCyrillic.Impact),
	}
}

func (l *NoCyrillic) Run(m *module.Module) {
	errorList := l.ErrorList.WithModule(m.GetName())

	if m.GetPath() == "" {
		return
	}

	files := fsutils.GetFiles(m.GetPath(), false, filterFiles)
	for _, fileName := range files {
		name, _ := strings.CutPrefix(fileName, m.GetPath())
		name = m.GetName() + ":" + name

		if slices.Contains(l.cfg.NoCyrillicFileExcludes, name) {
			continue
		}

		if l.skipDocRe.MatchString(fileName) {
			continue
		}

		if l.skipI18NRe.MatchString(fileName) {
			continue
		}

		if l.skipSelfRe.MatchString(fileName) {
			continue
		}

		lines, err := getFileContent(fileName)
		if err != nil {
			errorList.Error(err.Error())

			return
		}

		cyrMsg, hasCyr := checkCyrillicLettersInArray(lines)
		fName, _ := strings.CutPrefix(fileName, m.GetPath())
		if hasCyr {
			errorList.WithObjectID(fName).WithValue(cyrMsg).
				Error("has cyrillic letters")
		}
	}
}

func filterFiles(path string) bool {
	return slices.ContainsFunc(fileExtensions, func(s string) bool {
		return strings.HasSuffix(path, s)
	})
}

func getFileContent(filename string) ([]string, error) {
	fileBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	sliceData := strings.Split(string(fileBytes), "\n")

	return sliceData, nil
}

func (l *NoCyrillic) Name() string {
	return l.name
}

func (l *NoCyrillic) Desc() string {
	return l.desc
}
