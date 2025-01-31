package container

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Container linter
type Container struct {
	name, desc string
	cfg        *config.ContainerSettings
}

func New(cfg *config.ContainerSettings) *Container {
	return &Container{
		name: "container",
		desc: "Lint container objects",
		cfg:  cfg,
	}
}

func (o *Container) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(o.Name(), m.GetName())
	if m == nil {
		return nil
	}

	for _, object := range m.GetStorage() {
		applyContainerRules(object, result, o.cfg)
	}

	return result
}

func (o *Container) Name() string {
	return o.name
}

func (o *Container) Desc() string {
	return o.desc
}
