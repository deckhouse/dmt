package images

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/images/rules"
)

// Images linter
type Images struct {
	name, desc string
	cfg        *config.ImageSettings
}

func New(cfg *config.ModuleConfig) *Images {
	rules.Cfg = &cfg.LintersSettings.Images

	return &Images{
		name: "images",
		desc: "Lint docker images",
		cfg:  &cfg.LintersSettings.Images,
	}
}

func (*Images) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList("images", m.GetName())
	if m == nil {
		return result
	}

	result.Merge(rules.ApplyImagesRules(m))

	return result
}

func (o *Images) Name() string {
	return o.name
}

func (o *Images) Desc() string {
	return o.desc
}
