package vpa

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ID = "vpa-resources"
)

// VPAResources linter
type VPAResources struct {
	name, desc string
	cfg        *config.VPAResourcesSettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *VPAResources {
	skipVPAChecks = cfg.LintersSettings.VPAResources.SkipVPAChecks

	return &VPAResources{
		name:      ID,
		desc:      "Lint vpa-resources",
		cfg:       &cfg.LintersSettings.VPAResources,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.VPAResources.Impact),
	}
}

func (l *VPAResources) Run(m *module.Module) *errors.LintRuleErrorsList {
	if m == nil {
		return nil
	}

	l.controllerMustHaveVPA(m)

	return nil
}

func (l *VPAResources) Name() string {
	return l.name
}

func (l *VPAResources) Desc() string {
	return l.desc
}
