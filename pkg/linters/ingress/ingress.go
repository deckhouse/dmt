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
}

func New(cfg *config.IngressSettings) *Ingress {
	return &Ingress{
		name: "ingress",
		desc: "Lint ingresses rules",
		cfg:  cfg,
	}
}

func (o *Ingress) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(o.Name(), m.GetName())
	if m == nil {
		return nil
	}

	for _, object := range m.GetStorage() {
		o.ingressCopyCustomCertificateRule(m, object, result)
	}

	return result
}

func (o *Ingress) Name() string {
	return o.name
}

func (o *Ingress) Desc() string {
	return o.desc
}
