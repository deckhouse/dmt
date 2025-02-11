package images

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
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
	return &Images{
		name:      ID,
		desc:      "Lint docker images",
		cfg:       &cfg.LintersSettings.Images,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Images.Impact),
	}
}

func (l *Images) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())

	l.ApplyImagesRules(m, errorList)
}

func (l *Images) Name() string {
	return l.name
}

func (l *Images) Desc() string {
	return l.desc
}
