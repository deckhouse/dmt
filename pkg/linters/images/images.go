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

func New(cfg *config.ImageSettings) *Images {
	rules.Cfg = cfg

	return &Images{
		name: "images",
		desc: "Lint docker images",
		cfg:  cfg,
	}
}

func (*Images) Run(m *module.Module) (result *errors.LintRuleErrorsList, err error) {
	result = &errors.LintRuleErrorsList{}
	if m == nil {
		return result, err
	}

	result.Merge(rules.ApplyImagesRules(m))

	return result, nil
}

func (o *Images) Name() string {
	return o.name
}

func (o *Images) Desc() string {
	return o.desc
}
