package monitoring

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters"
)

// Monitoring linter
type Monitoring struct {
	name, desc string
	cfg        *config.MonitoringSettings
}

const ID = "monitoring"

var Cfg *config.MonitoringSettings

func New(cfg *config.ModuleConfig) linters.Linter {
	Cfg = &cfg.LintersSettings.Monitoring

	return &Monitoring{
		name: "monitoring",
		desc: "Lint monitoring rules",
		cfg:  &cfg.LintersSettings.Monitoring,
	}
}

func (*Monitoring) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, m.GetName())
	if m == nil {
		return result
	}

	result.Merge(MonitoringModuleRule(m.GetName(), m.GetPath(), m.GetNamespace()))

	// TODO: compile code instead of external binary - promtool
	for _, object := range m.GetStorage() {
		result.Merge(PromtoolRuleCheck(m, object))
	}

	return result
}

func (o *Monitoring) Name() string {
	return o.name
}

func (o *Monitoring) Desc() string {
	return o.desc
}
