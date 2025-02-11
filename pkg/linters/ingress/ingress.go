package ingress

import (
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

type Ingress struct {
	name string
	cfg  *config.IngressSettings
}

func Run(m *module.Module) {
	if m == nil {
		return
	}

	o := &Ingress{
		name: "ingress",
		cfg:  &config.Cfg.LintersSettings.Ingress,
	}
	logger.DebugF("Running linter `%s` on module `%s`", o.name, m.GetName())
	lintError := errors.NewError(o.name, m.GetName())

	for _, object := range m.GetStorage() {
		o.ingressCopyCustomCertificateRule(m, object, lintError)
	}
}
