package ingress

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters"
)

// Ingress linter
type Ingress struct {
	name, desc string
	cfg        *config.IngressSettings
}

const ID = "ingress"

var Cfg *config.IngressSettings

func New(cfg *config.ModuleConfig) linters.Linter {
	Cfg = &cfg.LintersSettings.Ingress

	return &Ingress{
		name: "ingress",
		desc: "Lint ingresses rules",
		cfg:  &cfg.LintersSettings.Ingress,
	}
}

func (*Ingress) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, m.GetName())
	if m == nil {
		return nil
	}

	for _, object := range m.GetStorage() {
		result.Merge(ingressCopyCustomCertificateRule(m, object))
	}

	return result
}

func (o *Ingress) Name() string {
	return o.name
}

func (o *Ingress) Desc() string {
	return o.desc
}
