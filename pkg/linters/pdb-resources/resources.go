package pdb

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ID = "pdb-resources"
)

// PDB linter
type PDB struct {
	name, desc string
	cfg        *config.PDBResourcesSettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *PDB {
	skipPDBChecks = cfg.LintersSettings.PDBResources.SkipPDBChecks

	return &PDB{
		name:      "pdb-resources",
		desc:      "Lint pdb-resources",
		cfg:       &cfg.LintersSettings.PDBResources,
		ErrorList: errorList,
	}
}

func (l *PDB) Run(m *module.Module) *errors.LintRuleErrorsList {
	if m == nil {
		return nil
	}

	l.controllerMustHavePDB(m)
	l.daemonSetMustNotHavePDB(m)

	return nil
}

func (l *PDB) Name() string {
	return l.name
}

func (l *PDB) Desc() string {
	return l.desc
}
