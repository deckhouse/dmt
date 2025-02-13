package templates

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/templates/rules"
)

const (
	ID = "templates"
)

// Templates linter
type Templates struct {
	name, desc string
	cfg        *config.TemplatesSettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Templates {
	return &Templates{
		name:      ID,
		desc:      "Lint vpa-resources",
		cfg:       &cfg.LintersSettings.Templates,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Templates.Impact),
	}
}

func (l *Templates) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())

	tr := rules.NewVPATolerationsRule(l.cfg.ExcludeRules.Tolerations.Get())

	rules.NewVPAAbsentRule(l.cfg.ExcludeRules.Absent.Get()).ControllerMustHaveVPA(m, tr, errorList)
}

func (l *Templates) Name() string {
	return l.name
}

func (l *Templates) Desc() string {
	return l.desc
}
