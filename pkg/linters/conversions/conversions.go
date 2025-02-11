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

	ErrorList *errors.LintRuleErrorsList
}

type ConversionsSettings struct {
	// skip all conversion checks for this modules
	SkipCheckModule map[string]struct{}
	// first conversion version to make conversion flow
	FirstVersion int
}

const ID = "conversions"

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Conversions {
	return &Conversions{
		name:      ID,
		desc:      "Lint conversions rules",
		cfg:       remapConversionsConfig(&cfg.LintersSettings.Conversions),
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Conversions.Impact),
	}
}

func (l *Conversions) Run(m *module.Module) {
	if m == nil {
		return
	}

	l.checkModuleYaml(m.GetName(), m.GetPath())
}

func (l *Conversions) Name() string {
	return l.name
}

func (l *Conversions) Desc() string {
	return l.desc
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
