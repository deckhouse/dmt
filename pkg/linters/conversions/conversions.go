package conversions

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Conversions linter
type Conversions struct {
	name string
	cfg  *ConversionsSettings
}

type ConversionsSettings struct {
	// skip all conversion checks for this modules
	SkipCheckModule map[string]struct{}
	// first conversion version to make conversion flow
	FirstVersion int
}

func Run(m *module.Module) {
	if m == nil {
		return
	}
	inputCfg := &config.Cfg.LintersSettings.Conversions
	o := &Conversions{
		name: "conversions",
		cfg:  remapConversionsConfig(inputCfg),
	}

	lintError := errors.NewError(o.name, m.GetName())

	_, ok := o.cfg.SkipCheckModule[m.GetName()]
	if ok {
		return
	}

	o.checkModuleYaml(m.GetPath(), lintError)
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
