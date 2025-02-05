package pdb

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ID = "pdb-resources"
)

// Object linter
type Object struct {
	name, desc string
	cfg        *config.PDBResourcesSettings
}

func New(cfg *config.PDBResourcesSettings) *Object {
	skipPDBChecks = cfg.SkipPDBChecks

	return &Object{
		name: "pdb-resources",
		desc: "Lint pdb-resources",
		cfg:  cfg,
	}
}

func (o *Object) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(o.Name(), m.GetName())
	if m == nil {
		return result
	}

	result.Merge(controllerMustHavePDB(m))
	result.Merge(daemonSetMustNotHavePDB(m))

	return result
}

func (o *Object) Name() string {
	return o.name
}

func (o *Object) Desc() string {
	return o.desc
}
