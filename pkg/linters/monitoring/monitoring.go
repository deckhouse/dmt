package monitoring

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Monitoring linter
type Monitoring struct {
	name string
	cfg  *config.MonitoringSettings
}

var Cfg *config.MonitoringSettings

func Run(m *module.Module) {
	if m == nil {
		return
	}

	o := &Monitoring{
		name: "monitoring",
		cfg:  &config.Cfg.LintersSettings.Monitoring,
	}
	Cfg = o.cfg
	lintError := errors.NewError(o.name, m.GetName())

	MonitoringModuleRule(m.GetName(), m.GetPath(), m.GetNamespace(), lintError)

	// TODO: compile code instead of external binary - promtool
	for _, object := range m.GetStorage() {
		PromtoolRuleCheck(m, object, lintError)
	}
}
