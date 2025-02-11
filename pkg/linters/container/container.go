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
		name:      ID,
		desc:      "Lint container objects",
		cfg:       &cfg.LintersSettings.Container,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Container.Impact),
	}
}

func (l *Container) Run(m *module.Module) *errors.LintRuleErrorsList {
	if m == nil {
		return nil
	}

	errorList := l.ErrorList.WithModule(m.GetName())
	for _, object := range m.GetStorage() {
		applyContainerRules(m.GetName(), object, errorList)
	}

	return nil
}

func (l *Container) Name() string {
	return l.name
}

func (l *Container) Desc() string {
	return l.desc
}
