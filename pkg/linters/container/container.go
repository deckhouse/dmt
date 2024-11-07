package container

import (
	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/errors"
)

// Container linter
type Container struct {
	name, desc string
	cfg        *config.ContainerSettings
}

func New(cfg *config.ContainerSettings) *Container {
	return &Container{
		name: "copyright",
		desc: "Copyright will check all files in the modules for contains copyright",
		cfg:  cfg,
	}
}

func (o *Container) Run(m *module.Module) (result errors.LintRuleErrorsList, err error) {

	return result, nil
}

func (o *Container) Name() string {
	return o.name
}

func (o *Container) Desc() string {
	return o.desc
}
