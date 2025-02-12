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

func (l *OSS) Run(m *module.Module) {
	if m == nil || m.GetPath() == "" {
		return
	}

	if l.cfg.Disable {
		return
	}

	l.ossModuleRule(m.GetName(), m.GetPath())
}

func (l *OSS) Name() string {
	return l.name
}

func (l *OSS) Desc() string {
	return l.desc
}
