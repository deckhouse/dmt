package hooks

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/hooks/rules"
)

// Hooks linter
type Hooks struct {
	name, desc string
	cfg        *config.HooksSettings
	ErrorList  *errors.LintRuleErrorsList
}

const ID = "hooks"

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Hooks {
	return &Hooks{
		name:      ID,
		desc:      "Lint hooks",
		cfg:       &cfg.LintersSettings.Hooks,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Hooks.Impact),
	}
}

func (h *Hooks) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := h.ErrorList.WithModule(m.GetName())
	r := rules.NewHookRule(h.cfg)
	for _, object := range m.GetStorage() {
		r.CheckIngressCopyCustomCertificateRule(m, object, errorList)
	}
}

func (h *Hooks) Name() string {
	return h.name
}

func (h *Hooks) Desc() string {
	return h.desc
}
