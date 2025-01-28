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
}

const ID = "module"

var Cfg *config.ModuleSettings

func New(cfg *config.ModuleSettings) *Module {
	Cfg = cfg

	return &Module{
		name: "module",
		desc: "Lint module rules",
		cfg:  cfg,
	}
}

func (*Module) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, m.GetName())
	if m == nil {
		return result
	}

	result.Merge(checkModuleYaml(m.GetName(), m.GetPath()))

	return result
}

func (o *Module) Name() string {
	return o.name
}

func (o *Module) Desc() string {
	return o.desc
}
