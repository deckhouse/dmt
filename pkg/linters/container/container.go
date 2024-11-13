package container

import (
	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/errors"
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

func (*Container) Run(m *module.Module) (result errors.LintRuleErrorsList, err error) {
	if m == nil {
		return result, err
	}

	for _, object := range m.GetStorage() {
		result.Merge(applyContainerRules(object))
	}

	return result, nil
}

func (o *Container) Name() string {
	return o.name
}

func (o *Container) Desc() string {
	return o.desc
}
