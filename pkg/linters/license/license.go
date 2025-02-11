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

func (l *Copyright) Run(m *module.Module) {
	if m.GetPath() == "" {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())

	files := fsutils.GetFiles(m.GetPath(), false, filterFiles)
	for _, fileName := range files {
		name, _ := strings.CutPrefix(fileName, m.GetPath())
		if name == "/charts/helm_lib" {
			continue
		}
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
}

func filterFiles(path string) bool {
	if fileToCheckRe.MatchString(path) && !fileToSkipRe.MatchString(path) {
		return true
	}
	return false
}

func (l *Copyright) Name() string {
	return l.name
}

func (l *Copyright) Desc() string {
	return l.desc
}
