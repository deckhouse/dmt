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

var Cfg *config.IngressSettings

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Ingress {
	Cfg = &cfg.LintersSettings.Ingress

	return &Ingress{
		name:      "ingress",
		desc:      "Lint ingresses rules",
		cfg:       &cfg.LintersSettings.Ingress,
		ErrorList: errorList,
	}
}

func (i *Ingress) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, m.GetName()).WithMaxLevel(i.cfg.Impact)
	if m == nil {
		return nil
	}

	for _, object := range m.GetStorage() {
		result.Merge(ingressCopyCustomCertificateRule(m, object))
	}

	result.CorrespondToMaxLevel()

	i.ErrorList.Merge(result)

	return result
}

func (i *Ingress) Name() string {
	return i.name
}

func (i *Ingress) Desc() string {
	return i.desc
}
