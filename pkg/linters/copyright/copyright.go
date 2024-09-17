package copyright

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/errors"
	"github.com/deckhouse/d8-lint/pkg/module"
)

// Copyright linter
type Copyright struct {
	name, desc string
	cfg        *config.CopyrightSettings
}

type Module interface {
	GetName() string
	GetPath() string
}

func New(cfg *config.CopyrightSettings) *Copyright {
	return &Copyright{
		name: "copyright",
		desc: "Copyright will check all files in the modules for contains copyright",
		cfg:  cfg,
	}
}

func (o *Copyright) Run(m *module.Module) (errors.LintRuleErrorsList, error) {
	files, err := o.getFiles(m.GetPath())
	if err != nil {
		return errors.LintRuleErrorsList{}, err
	}

	var result errors.LintRuleErrorsList
	for _, fileName := range files {
		name, _ := strings.CutPrefix(fileName, m.GetPath())
		name = m.GetName() + ":" + name
		if _, ok := o.cfg.CopyrightExcludes[name]; ok {
			continue
		}

		ok, er := checkFileCopyright(fileName)
		if !ok {
			path, _ := strings.CutPrefix(fileName, m.GetPath())
			result.Add(errors.NewLintRuleError(
				"copyright",
				path,
				er,
				"errors in `%s` module",
				m.GetName(),
			))
		}
	}

	return result, nil
}

func (*Copyright) getFiles(rootPath string) ([]string, error) {
	var result []string
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, _ error) error {
		if info.Mode()&os.ModeSymlink != 0 {
			return filepath.SkipDir
		}

		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}

			return nil
		}

		if fileToCheckRe.MatchString(path) && !fileToSkipRe.MatchString(path) {
			result = append(result, path)
		}

		return nil
	})

	return result, err
}

func (o *Copyright) Name() string {
	return o.name
}

func (o *Copyright) Desc() string {
	return o.desc
}
