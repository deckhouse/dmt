package openapienum

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Enum linter
type Enum struct {
	name, desc string
	cfg        *config.ProbesSettings
}

func New(cfg *config.ProbesSettings) *Enum {
	return &Enum{
		name: "openapi-enum",
		desc: "Probes will check openapi enum values is correct",
		cfg:  cfg,
	}
}

func (*Enum) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList("probes", m.GetName())

	return result
}

func (o *Enum) Name() string {
	return o.name
}

func (o *Enum) Desc() string {
	return o.desc
}
