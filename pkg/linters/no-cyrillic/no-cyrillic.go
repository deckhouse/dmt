package nocyrillic

import (
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/errors"
	"github.com/deckhouse/d8-lint/pkg/fsutils"
	"github.com/deckhouse/d8-lint/pkg/module"
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
	return &NoCyrillic{
		name:       "no-cyrillic",
		desc:       "NoCyrillic will check all files in the modules for contains cyrillic symbols",
		cfg:        cfg,
		skipDocRe:  regexp.MustCompile(cfg.SkipDocRe),
		skipI18NRe: regexp.MustCompile(cfg.SkipI18NRe),
		skipSelfRe: regexp.MustCompile(cfg.SkipSelfRe),
	}
}

func (o *NoCyrillic) Run(m *module.Module) (errors.LintRuleErrorsList, error) {
	files, err := o.getFiles(m.GetPath())
	if err != nil {
		return errors.LintRuleErrorsList{}, err
	}

	var result errors.LintRuleErrorsList
	for _, fileName := range files {
		name, _ := strings.CutPrefix(fileName, m.GetPath())
		name = m.GetName() + ":" + name
		if _, ok := o.cfg.NoCyrillicFileExcludes[name]; ok {
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

		lines, er := getFileContent(fileName)
		if er != nil {
			return errors.LintRuleErrorsList{}, er
		}

		cyrMsg, hasCyr := checkCyrillicLettersInArray(lines)
		if hasCyr {
			result.Add(errors.NewLintRuleError(
				"no-cyrillic",
				fileName,
				addPrefix(strings.Split(cyrMsg, "\n"), "\t"),
				"errors in `%s` module",
				m.GetName(),
			))
		}
	}

	return result, nil
}

func (o *NoCyrillic) getFiles(rootPath string) ([]string, error) {
	var result []string
	files, err := fsutils.GetFiles(rootPath, false)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !slices.ContainsFunc(o.cfg.FileExtensions, func(s string) bool {
			return strings.HasSuffix(file, s)
		}) {
			continue
		}
		result = append(result, file)
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
