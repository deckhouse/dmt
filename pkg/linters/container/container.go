package container

import (
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

var Cfg *config.ContainerSettings

// Container linter
type Container struct {
	name string
}

func Run(m *module.Module) {
	if m == nil {
		return
	}

	Cfg = &config.Cfg.LintersSettings.Container

	c := &Container{
		name: "container",
	}

	logger.DebugF("Running linter `%s` on module `%s`", c.name, m.GetName())

	lintError := errors.NewError(c.name, m.GetName())

	for _, object := range m.GetStorage() {
		applyContainerRules(object, lintError)
	}
}
