package monitoring

import (
	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/errors"
)

// Monitoring linter
type Monitoring struct {
	name, desc string
	cfg        *config.MonitoringSettings
}

var Cfg *config.MonitoringSettings

func New(cfg *config.MonitoringSettings) *Monitoring {
	Cfg = cfg

	return &Monitoring{
		name: "monitoring",
		desc: "Lint monitoring rules",
		cfg:  cfg,
	}
}

func (*Monitoring) Run(m *module.Module) (result errors.LintRuleErrorsList, err error) {
	if m == nil {
		return result, err
	}

	result.Add(MonitoringModuleRule(m.GetName(), m.GetPath(), m.GetNamespace()))

	// TODO: compile code instead of external binary - promtool
	for _, object := range m.GetStorage() {
		result.Add(PromtoolRuleCheck(m, object))
	}

	return result, nil
}

func (o *Monitoring) Name() string {
	return o.name
}

func (o *Monitoring) Desc() string {
	return o.desc
}
