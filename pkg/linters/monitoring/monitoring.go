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

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Monitoring {
	return &Monitoring{
		name:      ID,
		desc:      "Lint monitoring rules",
		cfg:       &cfg.LintersSettings.Monitoring,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Monitoring.Impact),
	}
}

func (l *Monitoring) Run(m *module.Module) {
	if m == nil {
		return
	}

	l.checkMonitoringRules(m.GetName(), m.GetPath(), m.GetNamespace())
}

func (l *Monitoring) Name() string {
	return l.name
}

func (l *Monitoring) Desc() string {
	return l.desc
}
