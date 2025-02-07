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
	name, desc     string
	cfg            *config.NoCyrillicSettings
	fileExtensions []string
	skipDocRe      *regexp.Regexp
	skipI18NRe     *regexp.Regexp
	skipSelfRe     *regexp.Regexp
	ErrorList      *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *NoCyrillic {
	// default settings for no-cyrillic
	fileExtensions := []string{"yaml", "yml", "json", "go"}
	skipDocRe := `doc-ru-.+\.y[a]?ml$|_RU\.md$|_ru\.html$|docs/site/_.+|docs/documentation/_.+|tools/spelling/.+|openapi/conversions/.+`
	skipSelfRe := `no_cyrillic(_test)?.go$`
	skipI18NRe := `/i18n/`

	return &NoCyrillic{
		name:           ID,
		desc:           "NoCyrillic will check all files in the modules for contains cyrillic symbols",
		fileExtensions: fileExtensions,
		skipDocRe:      regexp.MustCompile(skipDocRe),
		skipI18NRe:     regexp.MustCompile(skipSelfRe),
		skipSelfRe:     regexp.MustCompile(skipI18NRe),
		cfg:            &cfg.LintersSettings.NoCyrillic,
		ErrorList:      errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.NoCyrillic.Impact),
	}
}

func (l *NoCyrillic) Run(m *module.Module) *errors.LintRuleErrorsList {
	errorList := l.ErrorList.WithModule(m.GetName())

	if m.GetPath() == "" {
		return nil
	}

	files, err := l.getFiles(m.GetPath())
	if err != nil {
		errorList.Error(err.Error())

		return nil
	}

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

			return nil
		}

		cyrMsg, hasCyr := checkCyrillicLettersInArray(lines)
		fName, _ := strings.CutPrefix(fileName, m.GetPath())
		if hasCyr {
			errorList.WithObjectID(fName).WithValue(addPrefix(strings.Split(cyrMsg, "\n"), "\t")).
				Error("has cyrillic letters")
		}
	}

	return nil
}

func (l *NoCyrillic) getFiles(rootPath string) ([]string, error) {
	var result []string

	files, err := fsutils.GetFiles(rootPath, false)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if slices.ContainsFunc(l.fileExtensions, func(s string) bool {
			return strings.HasSuffix(file, s)
		}) {
			result = append(result, file)
		}
	}

	return result, nil
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
