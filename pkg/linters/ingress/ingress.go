package ingress

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Ingress linter
type Ingress struct {
	name, desc string
	cfg        *config.IngressSettings
	ErrorList  *errors.LintRuleErrorsList
}

const ID = "ingress"

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Ingress {
	return &Ingress{
		name:      ID,
		desc:      "Lint ingresses rules",
		cfg:       &cfg.LintersSettings.Ingress,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Ingress.Impact),
	}
}

func (l *Ingress) Run(m *module.Module) {
	if m == nil {
		return
	}

	for _, object := range m.GetStorage() {
		l.ingressCopyCustomCertificateRule(m, object)
	}
}

func (l *Ingress) Name() string {
	return l.name
}

func (l *Ingress) Desc() string {
	return l.desc
}
