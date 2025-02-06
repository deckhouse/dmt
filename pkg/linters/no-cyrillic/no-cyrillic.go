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
		name:           "no-cyrillic",
		desc:           "NoCyrillic will check all files in the modules for contains cyrillic symbols",
		fileExtensions: fileExtensions,
		skipDocRe:      regexp.MustCompile(skipDocRe),
		skipI18NRe:     regexp.MustCompile(skipSelfRe),
		skipSelfRe:     regexp.MustCompile(skipI18NRe),
		cfg:            &cfg.LintersSettings.NoCyrillic,
		ErrorList:      errorList,
	}
}

func (o *NoCyrillic) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList("no-cyrillic", m.GetName()).WithMaxLevel(o.cfg.Impact)

	if m.GetPath() == "" {
		return result
	}

	files, err := o.getFiles(m.GetPath())
	if err != nil {
		result.WithValue([]string{err.Error()}).
			Add("error in `%s` module", m.GetName())

		return result
	}

	for _, fileName := range files {
		name, _ := strings.CutPrefix(fileName, m.GetPath())
		name = m.GetName() + ":" + name
		if slices.Contains(o.cfg.NoCyrillicFileExcludes, name) {
			continue
		}
		if o.skipDocRe.MatchString(fileName) {
			continue
		}

		if o.skipI18NRe.MatchString(fileName) {
			continue
		}

		if o.skipSelfRe.MatchString(fileName) {
			continue
		}

		lines, err := getFileContent(fileName)
		if err != nil {
			result.WithValue([]string{err.Error()}).
				Add("error in `%s` module", m.GetName())
			return result
		}

		cyrMsg, hasCyr := checkCyrillicLettersInArray(lines)
		fName, _ := strings.CutPrefix(fileName, m.GetPath())
		if hasCyr {
			result.WithObjectID(fName).WithValue(addPrefix(strings.Split(cyrMsg, "\n"), "\t")).
				Add("errors in `%s` module", m.GetName())
		}
	}

	result.CorrespondToMaxLevel()

	o.ErrorList.Merge(result)

	return result
}

func (o *NoCyrillic) getFiles(rootPath string) ([]string, error) {
	var result []string
	files, err := fsutils.GetFiles(rootPath, false)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if slices.ContainsFunc(o.fileExtensions, func(s string) bool {
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

func (o *NoCyrillic) Name() string {
	return o.name
}

func (o *NoCyrillic) Desc() string {
	return o.desc
}
