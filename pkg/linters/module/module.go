package module

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/module/rules"
)

// Module linter
type Module struct {
	name, desc string
	cfg        *config.ModuleSettings
	ErrorList  *errors.LintRuleErrorsList
}

const ID = "module"

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Module {
	return &Module{
		name:      ID,
		desc:      "Lint module rules",
		cfg:       &cfg.LintersSettings.Module,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Module.Impact),
	}
}

func (l *Module) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())

	rules.NewDefinitionFileRule(l.cfg.DefinitionFile.Disable).CheckDefinitionFile(m.GetPath(), errorList)
	rules.NewOSSRule(l.cfg.OSS.Disable).OssModuleRule(m.GetPath(), errorList)
	rules.NewConversionsRule(l.cfg.Conversions.Disable).CheckConversions(m.GetPath(), errorList)
}

func (l *Module) Name() string {
	return l.name
}

func (l *Module) Desc() string {
	return l.desc
}
