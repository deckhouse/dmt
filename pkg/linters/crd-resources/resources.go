package crd

import (
	"path/filepath"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ID      = "crd-resources"
	CrdsDir = "crds"
)

// CRDResources linter
type CRDResources struct {
	name, desc string
	cfg        *config.CRDResourcesSettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *CRDResources {
	return &CRDResources{
		name:      ID,
		desc:      "Lint crd-resources",
		cfg:       &cfg.LintersSettings.CRDResources,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.CRDResources.Impact),
	}
}

func (l *CRDResources) Run(m *module.Module) {
	if m == nil {
		return
	}

	l.crdsModuleRule(m.GetName(), filepath.Join(m.GetPath(), CrdsDir))
}

func (l *CRDResources) Name() string {
	return l.name
}

func (l *CRDResources) Desc() string {
	return l.desc
}
