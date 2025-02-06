package openapienum

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Enum linter
type Enum struct {
	name, desc  string
	cfg         *config.OpenAPIEnumSettings
	keyExcludes map[string]struct{}
}

func New(cfg *config.OpenAPIEnumSettings) *Enum {
	keyExcludes := make(map[string]struct{})

	for _, exc := range cfg.EnumFileExcludes["*"] {
		keyExcludes[exc+".enum"] = struct{}{}
	}

	return &Enum{
		name:        "openapi-enum",
		desc:        "Probes will check openapi enum values is correct",
		cfg:         cfg,
		keyExcludes: keyExcludes,
	}
}

func (*Enum) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList("openapi-enum", m.GetName())

	return result
}

func (o *Enum) Name() string {
	return o.name
}

func (o *Enum) Desc() string {
	return o.desc
}
