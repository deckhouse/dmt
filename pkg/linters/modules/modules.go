package modules

import (
	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/errors"
	"github.com/deckhouse/d8-lint/pkg/linters/modules/rules"
)

// Modules linter
type Modules struct {
	name, desc string
	cfg        *config.ModulesSettings
}

func New(cfg *config.ModulesSettings) *Modules {
	rules.Cfg = cfg

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

	result.Merge(rules.ApplyModuleRules(m))

	return result, nil
}

func (o *Modules) Name() string {
	return o.name
}

func (o *Modules) Desc() string {
	return o.desc
}
