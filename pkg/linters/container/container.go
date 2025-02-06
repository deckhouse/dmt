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
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Container {
	Cfg = &cfg.LintersSettings.Container

	return &Container{
		name:      "container",
		desc:      "Lint container objects",
		cfg:       &cfg.LintersSettings.Container,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Container.Impact),
	}
}

func (o *Container) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList("images", m.GetName()).WithMaxLevel(o.cfg.Impact)
	if m == nil {
		return result
	}

	for _, object := range m.GetStorage() {
		result.Merge(o.applyContainerRules(m, object))
	}

	result.CorrespondToMaxLevel()

	o.ErrorList.Merge(result)

	return o.ErrorList
}

func (o *Container) Name() string {
	return o.name
}

func (o *Container) Desc() string {
	return o.desc
}
