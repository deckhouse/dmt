package crd

import (
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters"
)

const (
	ID      = "crd-resources"
	CrdsDir = "crds"
)

// Object linter
type Object struct {
	name, desc string
	cfg        *config.CRDResourcesSettings
}

func New(cfg *config.ModuleConfig) linters.Linter {
	return &Object{
		name: "crd-resources",
		desc: "Lint crd-resources",
		cfg:  &cfg.LintersSettings.CRDResources,
	}
}

func (o *Object) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(o.Name(), m.GetName())
	if m == nil {
		return result
	}

	if isExistsOnFilesystem(m.GetPath(), CrdsDir) {
		result.Merge(crdsModuleRule(m.GetName(), filepath.Join(m.GetPath(), CrdsDir)))
	}

	return result
}

func (o *Object) Name() string {
	return o.name
}

func (o *Object) Desc() string {
	return o.desc
}

func isExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}
