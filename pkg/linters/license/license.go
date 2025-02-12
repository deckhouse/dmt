package license

import (
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

	NewFilesRule(l.cfg.ExcludeRules.Files.Get()).
		checkFiles(m, errorList)

}

func (r *FilesRule) checkFiles(module *module.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	files := fsutils.GetFiles(module.GetPath(), false, filterFiles)
	for _, fileName := range files {
		name, _ := strings.CutPrefix(fileName, module.GetPath())
		name = module.GetName() + ":" + name

		if !r.Enabled(name) {
			// TODO: add metrics
			continue
		}

		ok, err := checkFileCopyright(fileName)
		if !ok {
			path, _ := strings.CutPrefix(fileName, module.GetPath())

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
