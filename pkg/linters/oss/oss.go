package oss

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ID = "oss"
)

// Copyright linter
type OSS struct {
	name, desc string
	cfg        *config.OSSSettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *OSS {
	return &OSS{
		name:      ID,
		desc:      "Copyright will check oss license file",
		cfg:       &cfg.LintersSettings.OSS,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.OSS.Impact),
	}
}

func (o *OSS) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(o.Name(), m.GetName()).WithMaxLevel(o.cfg.Impact)

	if m.GetPath() == "" {
		return result
	}

	result.Merge(o.ossModuleRule(m.GetName(), m.GetPath()))

	result.CorrespondToMaxLevel()

	o.ErrorList.Merge(result)

	return result
}

func (o *OSS) Name() string {
	return o.name
}

func (o *OSS) Desc() string {
	return o.desc
}
