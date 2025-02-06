package vpa

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ID = "vpa-resources"
)

// Object linter
type Object struct {
	name, desc string
	cfg        *config.VPAResourcesSettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Object {
	skipVPAChecks = cfg.LintersSettings.VPAResources.SkipVPAChecks

	return &Object{
		name:      "vpa-resources",
		desc:      "Lint vpa-resources",
		cfg:       &cfg.LintersSettings.VPAResources,
		ErrorList: errorList,
	}
}

func (o *Object) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(o.Name(), m.GetName()).WithMaxLevel(o.cfg.Impact)
	if m == nil {
		return result
	}

	result.Merge(controllerMustHaveVPA(m))

	result.CorrespondToMaxLevel()

	o.ErrorList.Merge(result)

	return result
}

func (o *Object) Name() string {
	return o.name
}

func (o *Object) Desc() string {
	return o.desc
}
