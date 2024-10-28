package copyright

import (
	"slices"
	"strings"

	"github.com/deckhouse/d8-lint/internal/fsutils"
	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/errors"
)

// Copyright linter
type Copyright struct {
	name, desc string
	cfg        *config.CopyrightSettings
}

func New(cfg *config.CopyrightSettings) *Copyright {
	return &Copyright{
		name: "copyright",
		desc: "Copyright will check all files in the modules for contains copyright",
		cfg:  cfg,
	}
}

func (o *Copyright) Run(m *module.Module) (errors.LintRuleErrorsList, error) {
	files, err := getFiles(m.GetPath())
	if err != nil {
		return errors.LintRuleErrorsList{}, err
	}

	var result errors.LintRuleErrorsList
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
