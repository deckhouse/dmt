package conversions

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Conversions linter
type Conversions struct {
	name, desc string
	cfg        *config.ConversionsSettings
}

const ID = "conversions"

var Cfg *config.ConversionsSettings

func New(cfg *config.ConversionsSettings) *Conversions {
	Cfg = cfg

	return &Conversions{
		name: ID,
		desc: "Lint conversions rules",
		cfg:  cfg,
	}
}

func (*Conversions) Run(m *module.Module) (errors.LintRuleErrorsList, error) {
	result := errors.LintRuleErrorsList{}

	if m == nil {
		return result, nil
	}

	result.Merge(checkModuleYaml(m.GetName(), m.GetPath()))

	return result, nil
}

func (o *Conversions) Name() string {
	return o.name
}

func (o *Conversions) Desc() string {
	return o.desc
}
