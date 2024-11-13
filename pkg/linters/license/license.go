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

func (o *Copyright) Run(m *module.Module) (errors.LintRuleErrorsList, error) {
	if m.GetPath() == "" {
		return errors.LintRuleErrorsList{}, nil
	}
	files, err := getFiles(m.GetPath())
	if err != nil {
		return errors.LintRuleErrorsList{}, err
	}

	var result errors.LintRuleErrorsList

	result.Merge(OssModuleRule(m.GetName(), m.GetPath()))

	for _, fileName := range files {
		name, _ := strings.CutPrefix(fileName, m.GetPath())
		name = m.GetName() + ":" + name
		if slices.Contains(o.cfg.CopyrightExcludes, name) {
			continue
		}

		ok, er := checkFileCopyright(fileName)
		if !ok {
			path, _ := strings.CutPrefix(fileName, m.GetPath())
			result.Add(errors.NewLintRuleError(
				"copyright",
				path,
				m.GetName(),
				er,
				"errors in `%s` module",
				m.GetName(),
			))
		}
	}

	return result, nil
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
