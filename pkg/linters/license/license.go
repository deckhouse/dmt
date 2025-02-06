package license

import (
	"slices"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Copyright linter
type Copyright struct {
	name, desc string
	cfg        *config.LicenseSettings
}

func New(cfg *config.ModuleConfig) *Copyright {
	return &Copyright{
		name: "license",
		desc: "Copyright will check all files in the modules for contains copyright",
		cfg:  &cfg.LintersSettings.License,
	}
}

func (o *Copyright) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(o.Name(), m.GetName())

	if m.GetPath() == "" {
		return result
	}
	files, err := getFiles(m.GetPath())
	if err != nil {
		return result.WithValue(err.Error()).Add("error getting files in `%s` module", m.GetName())
	}

	for _, fileName := range files {
		name, _ := strings.CutPrefix(fileName, m.GetPath())
		name = m.GetName() + ":" + name
		if slices.Contains(o.cfg.CopyrightExcludes, name) {
			continue
		}

		ok, err := checkFileCopyright(fileName)
		if !ok {
			path, _ := strings.CutPrefix(fileName, m.GetPath())
			result.WithObjectID(path).WithValue(err).
				Add("errors in `%s` module", m.GetName())
		}
	}

	return result
}

func getFiles(rootPath string) ([]string, error) {
	files, err := fsutils.GetFiles(rootPath, true)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, path := range files {
		if fileToCheckRe.MatchString(path) && !fileToSkipRe.MatchString(path) {
			result = append(result, path)
		}
	}

	return result, nil
}

func (o *Copyright) Name() string {
	return o.name
}

func (o *Copyright) Desc() string {
	return o.desc
}
