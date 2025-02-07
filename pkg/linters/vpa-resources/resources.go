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

func (o *VPAResources) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(o.Name(), m.GetName()).WithMaxLevel(o.cfg.Impact)
	if m == nil {
		return result
	}

	result.Merge(controllerMustHaveVPA(m))

	result.CorrespondToMaxLevel()

	o.ErrorList.Merge(result)

	return result
}

func (o *VPAResources) Name() string {
	return o.name
}

func (o *VPAResources) Desc() string {
	return o.desc
}
