package modules

import (
	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/errors"
)

// Modules linter
type Modules struct {
	name, desc string
	cfg        *config.ModulesSettings
}

const (
	ID = "modules"
)

var Cfg *config.ModulesSettings

func New(cfg *config.ModulesSettings) *Modules {
	Cfg = cfg

	return &Modules{
		name: "modules",
		desc: "Lint modules objects",
		cfg:  cfg,
	}
}

func (o *Modules) Run(m *module.Module) (result errors.LintRuleErrorsList, err error) {
	if m == nil {
		return result, err
	}

	result.Merge(applyModuleRules(m))

	return result, nil
}

func (o *Modules) Name() string {
	return o.name
}

func (o *Modules) Desc() string {
	return o.desc
}
