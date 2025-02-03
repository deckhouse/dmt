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
	name, desc string
	cfg        *config.NoCyrillicSettings

	skipDocRe  *regexp.Regexp
	skipI18NRe *regexp.Regexp
	skipSelfRe *regexp.Regexp
}

func New(cfg *config.NoCyrillicSettings) *NoCyrillic {
	// default settings for no-cyrillic
	if len(cfg.FileExtensions) == 0 {
		cfg.FileExtensions = []string{
			"yaml", "yml", "json",
			"go",
		}
	}

	if cfg.SkipDocRe == "" {
		cfg.SkipDocRe = `doc-ru-.+\.y[a]?ml$|_RU\.md$|_ru\.html$|docs/site/_.+|docs/documentation/_.+|tools/spelling/.+`
	}

	if cfg.SkipSelfRe == "" {
		cfg.SkipSelfRe = `no_cyrillic(_test)?.go$`
	}

	if cfg.SkipI18NRe == "" {
		cfg.SkipI18NRe = `/i18n/`
	}

	return &NoCyrillic{
		name:       "no-cyrillic",
		desc:       "NoCyrillic will check all files in the modules for contains cyrillic symbols",
		cfg:        cfg,
		skipDocRe:  regexp.MustCompile(cfg.SkipDocRe),
		skipI18NRe: regexp.MustCompile(cfg.SkipI18NRe),
		skipSelfRe: regexp.MustCompile(cfg.SkipSelfRe),
	}
}

func (o *NoCyrillic) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList("no-cyrillic", m.GetName())

	if m.GetPath() == "" {
		return result
	}

	files, err := o.getFiles(m.GetPath())
	if err != nil {
		result.AddValue(
			[]string{err.Error()},
			"error in `%s` module",
			m.GetName())

		return result
	}

	for _, fileName := range files {
		skip := func(fileName string) bool {
			name, _ := strings.CutPrefix(fileName, m.GetPath())
			name = m.GetName() + ":" + name
			if slices.Contains(o.cfg.NoCyrillicFileExcludes, name) {
				return true
			}
			if o.skipDocRe.MatchString(fileName) {
				return true
			}

			if o.skipI18NRe.MatchString(fileName) {
				return true
			}

			if o.skipSelfRe.MatchString(fileName) {
				return true
			}

			return false
		}

		lines, err := getFileContent(fileName)
		if err != nil {
			result.
				WithWarning(skip(fileName)).
				AddValue(
					[]string{err.Error()},
					"error in `%s` module",
					m.GetName())
			return result
		}

		cyrMsg, hasCyr := checkCyrillicLettersInArray(lines)
		fName, _ := strings.CutPrefix(fileName, m.GetPath())
		if hasCyr {
			result.WithObjectID(fName).
				WithWarning(skip(fileName)).
				AddValue(
					addPrefix(strings.Split(cyrMsg, "\n"), "\t"),
					"errors in `%s` module",
					m.GetName())
		}
	}

	return result
}

func (o *NoCyrillic) getFiles(rootPath string) ([]string, error) {
	var result []string
	files, err := fsutils.GetFiles(rootPath, false)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if slices.ContainsFunc(o.cfg.FileExtensions, func(s string) bool {
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
