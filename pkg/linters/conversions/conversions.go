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

type ConversionsSettings struct {
	// skip all conversion checks for this modules
	SkipCheckModule map[string]struct{}
	// first conversion version to make conversion flow
	FirstVersion int
}

const ID = "conversions"

var cfg *ConversionsSettings

func New(inputCfg *config.ConversionsSettings) *Conversions {
	cfg = remapConversionsConfig(inputCfg)

	return &Conversions{
		name: ID,
		desc: "Lint conversions rules",
		cfg:  inputCfg,
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

func remapConversionsConfig(input *config.ConversionsSettings) *ConversionsSettings {
	cfg := &ConversionsSettings{
		FirstVersion: input.FirstVersion,
	}

	for _, module := range input.SkipCheckModule {
		cfg.SkipCheckModule[module] = struct{}{}
	}

	return cfg
}
