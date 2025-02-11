package monitoring

import (
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Monitoring linter
type Monitoring struct {
	name string
	cfg  *config.MonitoringSettings
}

func Run(m *module.Module) {
	if m == nil {
		return
	}

	o := &Monitoring{
		name: "monitoring",
		cfg:  &config.Cfg.LintersSettings.Monitoring,
	}
	logger.DebugF("Running linter `%s` on module `%s`", o.name, m.GetName())
	lintError := errors.NewError(o.name, m.GetName())

	o.monitoringModuleRule(m.GetName(), m.GetPath(), m.GetNamespace(), lintError)

	// TODO: compile code instead of external binary - promtool
	for _, object := range m.GetStorage() {
		o.promtoolRuleCheck(m, object, lintError)
	}
}
