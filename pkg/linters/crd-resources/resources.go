package crd

import (
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ID      = "crd-resources"
	CrdsDir = "crds"
)

// Object linter
type Object struct {
	name, desc string
	cfg        *config.CRDResourcesSettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Object {
	return &Object{
		name:      "crd-resources",
		desc:      "Lint crd-resources",
		cfg:       &cfg.LintersSettings.CRDResources,
		ErrorList: errorList,
	}
}

func (o *Object) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(o.Name(), m.GetName()).WithMaxLevel(o.cfg.Impact)
	if m == nil {
		return result
	}

	if isExistsOnFilesystem(m.GetPath(), CrdsDir) {
		result.Merge(crdsModuleRule(m.GetName(), filepath.Join(m.GetPath(), CrdsDir)))
	}

	result.CorrespondToMaxLevel()

	o.ErrorList.Merge(result)

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
