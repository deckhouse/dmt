package container

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

var Cfg *config.ContainerSettings

// Container linter
type Container struct {
	name, desc string
	cfg        *config.ContainerSettings
	result     *errors.LintRuleErrorsList
}

func New(cfg *config.ContainerSettings) *Container {
	Cfg = cfg

	return &Container{
		name: "container",
		desc: "Lint container objects",
		cfg:  cfg,
	}
}

func (o *Container) Run(m *module.Module) *errors.LintRuleErrorsList {
	o.result = errors.NewLinterRuleList(o.Name(), m.GetName())
	if m == nil {
		return nil
	}

	for _, object := range m.GetStorage() {
		o.result.Merge(o.applyContainerRules(object))
	}

	return o.result
}

func (o *Container) Name() string {
	return o.name
}

func (o *Container) Desc() string {
	return o.desc
}
