package license

import (
	"slices"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ID = "license"
)

// Copyright linter
type Copyright struct {
	name, desc string
	cfg        *config.LicenseSettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Copyright {
	return &Copyright{
		name:      ID,
		desc:      "Copyright will check all files in the modules for contains copyright",
		cfg:       &cfg.LintersSettings.License,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.License.Impact),
	}
}

func (l *Copyright) Run(m *module.Module) *errors.LintRuleErrorsList {
	if m.GetPath() == "" {
		return nil
	}

	errorList := l.ErrorList.WithModule(m.GetName())

	files, err := getFiles(m.GetPath())
	if err != nil {
		errorList.Error("error getting files")

		return nil
	}

	for _, fileName := range files {
		name, _ := strings.CutPrefix(fileName, m.GetPath())
		name = m.GetName() + ":" + name
		if slices.Contains(l.cfg.CopyrightExcludes, name) {
			continue
		}

		ok, err := checkFileCopyright(fileName)
		if !ok {
			path, _ := strings.CutPrefix(fileName, m.GetPath())

			errorList.WithFilePath(path).Error(err.Error())
		}
	}

	return nil
}

func getFiles(rootPath string) ([]string, error) {
	files, err := fsutils.GetFiles(rootPath, true)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, path := range files {
		if fileToCheckRe.MatchString(path) && !fileToSkipRe.MatchString(path) {
			result = append(result, path)
		}
	}

	return result, nil
}

func (l *Copyright) Name() string {
	return l.name
}

func (l *Copyright) Desc() string {
	return l.desc
}
