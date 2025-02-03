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

var Cfg *config.LicenseSettings

func New(cfg *config.LicenseSettings) *Copyright {
	Cfg = cfg
	return &Copyright{
		name: "license",
		desc: "Copyright will check all files in the modules for contains copyright",
		cfg:  cfg,
	}
}

func (o *Copyright) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(o.Name(), m.GetName())

	if m.GetPath() == "" {
		return result
	}
	files, err := getFiles(m.GetPath())
	if err != nil {
		return result.AddValue(err.Error(), "error getting files in `%s` module", m.GetName())
	}

	result.Merge(OssModuleRule(m.GetName(), m.GetPath()))

	for _, fileName := range files {
		name, _ := strings.CutPrefix(fileName, m.GetPath())
		name = m.GetName() + ":" + name

		ok, err := checkFileCopyright(fileName)
		if !ok {
			path, _ := strings.CutPrefix(fileName, m.GetPath())
			result.WithObjectID(path).
				WithWarning(slices.Contains(o.cfg.CopyrightExcludes, name)).
				AddValue(
					err,
					"errors in `%s` module",
					m.GetName(),
				)
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
