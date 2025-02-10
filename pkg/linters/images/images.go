package images

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/images/rules"
)

const (
	ID = "images"
)

// Images linter
type Images struct {
	name, desc string
	cfg        *config.ImageSettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Images {
	rules.Cfg = &cfg.LintersSettings.Images

	return &Images{
		name:      ID,
		desc:      "Lint docker images",
		cfg:       &cfg.LintersSettings.Images,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Images.Impact),
	}
}

func (o *Images) Run(m *module.Module) *errors.LintRuleErrorsList {
	errorList := o.ErrorList.WithModule(m.GetName())
	if m == nil {
		return nil
	}

	errorList.CorrespondToMaxLevel()

	rules.ApplyImagesRules(m, errorList)

	return errorList
}

func (o *Images) Name() string {
	return o.name
}

func (o *Images) Desc() string {
	return o.desc
}
