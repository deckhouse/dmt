package conversions

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters"
)

// Conversions linter
type Conversions struct {
	name, desc string
	cfg        *ConversionsSettings
}

type ConversionsSettings struct {
	// skip all conversion checks for this modules
	SkipCheckModule map[string]struct{}
	// first conversion version to make conversion flow
	FirstVersion int
}

const ID = "conversions"

func New(cfg *config.ModuleConfig) linters.Linter {
	return &Conversions{
		name: ID,
		desc: "Lint conversions rules",
		cfg:  remapConversionsConfig(&cfg.LintersSettings.Conversions),
	}
}

func (c *Conversions) Run(m *module.Module) *errors.LintRuleErrorsList {

	result := errors.NewLinterRuleList(ID, m.GetName())

	if m == nil {
		return result
	}

	result.Merge(c.checkModuleYaml(m.GetName(), m.GetPath()))

	return result
}

func (c *Conversions) Name() string {
	return c.name
}

func (c *Conversions) Desc() string {
	return c.desc
}

func remapConversionsConfig(input *config.ConversionsSettings) *ConversionsSettings {
	newCfg := &ConversionsSettings{
		FirstVersion:    input.FirstVersion,
		SkipCheckModule: make(map[string]struct{}),
	}

	for _, module := range input.SkipCheckModule {
		newCfg.SkipCheckModule[module] = struct{}{}
	}

	return newCfg
}
