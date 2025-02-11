package nocyrillic

import (
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// NoCyrillic linter
type NoCyrillic struct {
	name string

	fileExtensions []string
	skipDocRe      *regexp.Regexp
	skipI18NRe     *regexp.Regexp
	skipSelfRe     *regexp.Regexp
}

var (
	fileExtensions = []string{"yaml", "yml", "json", "go"}
	skipDocRe      = `doc-ru-.+\.y[a]?ml$|_RU\.md$|_ru\.html$|docs/site/_.+|docs/documentation/_.+|tools/spelling/.+|openapi/conversions/.+`
	skipSelfRe     = `no_cyrillic(_test)?.go$`
	skipI18NRe     = `/i18n/`
)

func Run(m *module.Module) {
	// default settings for no-cyrillic
	cfg := config.Cfg.LintersSettings.NoCyrillic

	o := &NoCyrillic{
		name:           "no-cyrillic",
		fileExtensions: fileExtensions,
		skipDocRe:      regexp.MustCompile(skipDocRe),
		skipI18NRe:     regexp.MustCompile(skipI18NRe),
		skipSelfRe:     regexp.MustCompile(skipSelfRe),
	}

	logger.DebugF("Running linter `%s` on module `%s`", o.name, m.GetName())

	lintError := errors.NewError(o.name, m.GetName())

	if m.GetPath() == "" {
		return
	}

	files, err := o.getFiles(m.GetPath())
	if err != nil {
		lintError.WithValue([]string{err.Error()}).
			Add("error in `%s` module", m.GetName())

		return
	}

	for _, fileName := range files {
		name, _ := strings.CutPrefix(fileName, m.GetPath())
		name = m.GetName() + ":" + name
		if slices.Contains(cfg.NoCyrillicFileExcludes, name) {
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
			lintError.WithValue([]string{err.Error()}).
				Add("error in `%s` module", m.GetName())
			return
		}

		cyrMsg, hasCyr := checkCyrillicLettersInArray(lines)
		fName, _ := strings.CutPrefix(fileName, m.GetPath())
		if hasCyr {
			lintError.WithObjectID(fName).WithValue(addPrefix(strings.Split(cyrMsg, "\n"), "\t")).
				Add("errors in `%s` module", m.GetName())
		}
	}
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
