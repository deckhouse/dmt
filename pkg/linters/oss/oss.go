package oss

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Copyright linter
type OSS struct {
	name, desc string
	cfg        *config.OSSSettings
}

func New(cfg *config.ModuleConfig) *OSS {
	return &OSS{
		name: "oss",
		desc: "Copyright will check oss license file",
		cfg:  &cfg.LintersSettings.OSS,
	}
}

func (o *OSS) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(o.Name(), m.GetName())

	if m.GetPath() == "" {
		return result
	}

	result.Merge(o.ossModuleRule(m.GetName(), m.GetPath()))

	return result
}

func (o *OSS) Name() string {
	return o.name
}

func (o *OSS) Desc() string {
	return o.desc
}
