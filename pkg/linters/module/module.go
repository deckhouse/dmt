package module

import (
	"slices"

	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Module linter
type Module struct {
	name string
	cfg  *config.ModuleSettings
}

func Run(m *module.Module) {
	if m == nil {
		return
	}

	o := &Module{
		name: "module",
		cfg:  &config.Cfg.LintersSettings.Module,
	}
	logger.DebugF("Running linter `%s` on module `%s`", o.name, m.GetName())
	lintError := errors.NewError("module", m.GetName())

	if slices.Contains(o.cfg.SkipCheckModuleYaml, m.GetName()) {
		return
	}

	o.checkModuleYaml(m.GetPath(), lintError)
}
