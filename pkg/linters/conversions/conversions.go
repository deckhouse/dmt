package conversions

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Conversions linter
type Conversions struct {
	name, desc string
	cfg        *ConversionsSettings

	result *errors.LintRuleErrorsList
}

type ConversionsSettings struct {
	// skip all conversion checks for this modules
	SkipCheckModule map[string]struct{}
	// first conversion version to make conversion flow
	FirstVersion int
}

func New(inputCfg *config.ConversionsSettings) *Conversions {
	return &Conversions{
		name: "conversions",
		desc: "Lint conversions rules",
		cfg:  remapConversionsConfig(inputCfg),
	}
}

func (o *Conversions) Run(m *module.Module) *errors.LintRuleErrorsList {
	o.result = errors.NewLinterRuleList(o.Name(), m.GetName())

	if m == nil {
		return nil
	}

	o.result.Merge(o.checkModuleYaml(m.GetName(), m.GetPath()))

	return o.result
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
