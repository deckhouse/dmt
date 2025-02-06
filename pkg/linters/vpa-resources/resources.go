package vpa

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters"
)

const (
	ID = "vpa-resources"
)

// Object linter
type Object struct {
	name, desc string
	cfg        *config.VPAResourcesSettings
}

func New(cfg *config.ModuleConfig) linters.Linter {
	skipVPAChecks = cfg.LintersSettings.VPAResources.SkipVPAChecks

	return &Object{
		name: "vpa-resources",
		desc: "Lint vpa-resources",
		cfg:  &cfg.LintersSettings.VPAResources,
	}
}

func (o *Object) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(o.Name(), m.GetName())
	if m == nil {
		return result
	}

	result.Merge(controllerMustHaveVPA(m))

	return result
}

func (o *Object) Name() string {
	return o.name
}

func (o *Object) Desc() string {
	return o.desc
}
