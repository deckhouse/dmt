package helm

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/helm/rules"
)

// Helm linter
type Helm struct {
	name, desc string
	cfg        *config.HelmSettings
}

func New(cfg *config.HelmSettings) *Helm {
	rules.Cfg = cfg

	return &Helm{
		name: "helm",
		desc: "Lint helm objects",
		cfg:  cfg,
	}
}

func (*Helm) Run(m *module.Module) (result errors.LintRuleErrorsList, err error) {
	if m == nil {
		return result, err
	}

	result.Merge(rules.ApplyHelmRules(m))

	return result, nil
}

func (o *Helm) Name() string {
	return o.name
}

func (o *Helm) Desc() string {
	return o.desc
}
