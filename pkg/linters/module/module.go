package module

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
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

func (l *Module) Run(m *module.Module) *errors.LintRuleErrorsList {
	if m == nil {
		return nil
	}

	l.checkModuleYaml(m.GetName(), m.GetPath())

	return nil
}

func (l *Module) Name() string {
	return l.name
}

func (l *Module) Desc() string {
	return l.desc
}
