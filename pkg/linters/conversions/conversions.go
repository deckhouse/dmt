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

func (*Conversions) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, m.GetName())

	if m == nil {
		return result
	}

	result.Merge(checkModuleYaml(m.GetName(), m.GetPath()))

	return result
}

func (o *Conversions) Name() string {
	return o.name
}

func (o *Conversions) Desc() string {
	return o.desc
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
