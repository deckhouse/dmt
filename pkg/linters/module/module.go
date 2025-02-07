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

var Cfg *config.ModuleSettings

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Module {
	Cfg = &cfg.LintersSettings.Module

	return &Module{
		name:      ID,
		desc:      "Lint module rules",
		cfg:       &cfg.LintersSettings.Module,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Module.Impact),
	}
}

func (o *Module) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, m.GetName()).WithMaxLevel(o.cfg.Impact)
	if m == nil {
		return result
	}

	result.Merge(checkModuleYaml(m.GetName(), m.GetPath()))

	result.CorrespondToMaxLevel()

	o.ErrorList.Merge(result)

	return result
}

func (o *Module) Name() string {
	return o.name
}

func (o *Module) Desc() string {
	return o.desc
}
