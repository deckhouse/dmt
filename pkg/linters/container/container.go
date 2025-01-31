package container

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ID = "container"
)

var Cfg *config.ContainerSettings

// Container linter
type Container struct {
	name, desc string
	cfg        *config.ContainerSettings
}

func New(cfg *config.ContainerSettings) *Container {
	Cfg = cfg

	return &Container{
		name: "container",
		desc: "Lint container objects",
		cfg:  cfg,
	}
}

func (*Container) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, m.GetName())
	if m == nil {
		return result
	}

	for _, object := range m.GetStorage() {
		result.Merge(applyContainerRules(m, object))
	}

	return result
}

func (o *Container) Name() string {
	return o.name
}

func (o *Container) Desc() string {
	return o.desc
}
