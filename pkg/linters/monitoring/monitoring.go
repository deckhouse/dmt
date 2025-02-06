package monitoring

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Monitoring linter
type Monitoring struct {
	name, desc string
	cfg        *config.MonitoringSettings
	ErrorList  *errors.LintRuleErrorsList
}

const ID = "monitoring"

var Cfg *config.MonitoringSettings

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Monitoring {
	Cfg = &cfg.LintersSettings.Monitoring

	return &Monitoring{
		name:      "monitoring",
		desc:      "Lint monitoring rules",
		cfg:       &cfg.LintersSettings.Monitoring,
		ErrorList: errorList,
	}
}

func (o *Monitoring) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, m.GetName()).WithMaxLevel(o.cfg.Impact)
	if m == nil {
		return result
	}

	result.Merge(MonitoringModuleRule(m.GetName(), m.GetPath(), m.GetNamespace()))

	// TODO: compile code instead of external binary - promtool
	for _, object := range m.GetStorage() {
		result.Merge(PromtoolRuleCheck(m, object))
	}

	result.CorrespondToMaxLevel()

	o.ErrorList.Merge(result)

	return result
}

func (o *Monitoring) Name() string {
	return o.name
}

func (o *Monitoring) Desc() string {
	return o.desc
}
