package images

import (
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/images/rules"
)

// Images linter
type Images struct {
	name string
}

func Run(m *module.Module) {
	if m == nil {
		return
	}

	rules.Cfg = &config.Cfg.LintersSettings.Images

	o := &Images{
		name: "images",
	}
	logger.DebugF("Running linter `%s` on module `%s`", o.name, m.GetName())

	lintError := errors.NewError(o.name, m.GetName())

	rules.ApplyImagesRules(m, lintError)
}
