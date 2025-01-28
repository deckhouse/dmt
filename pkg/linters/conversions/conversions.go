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
	lintErrors *errors.LintRuleErrorsList
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
		name:       ID,
		desc:       "Lint conversions rules",
		cfg:        cfg,
		lintErrors: errors.NewLinterRuleList(),
	}
}

func (c *Conversions) Run(m *module.Module) (errors.LintRuleErrorsList, error) {
	result := errors.LintRuleErrorsList{}

	if m == nil {
		return result, nil
	}

	c.checkModuleYaml(m.GetName(), m.GetPath())
	c.lintErrors.DumpFromStorage()

	// TODO: make pointer and handle nil in merge
	result.Merge(*c.lintErrors)

	return result, nil
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
		SkipCheckModule: make(map[string]struct{}, len(input.SkipCheckModule)),
	}

	for _, module := range input.SkipCheckModule {
		newCfg.SkipCheckModule[module] = struct{}{}
	}

	return newCfg
}
