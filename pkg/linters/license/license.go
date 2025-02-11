package license

import (
	"slices"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Copyright linter
type Copyright struct {
	name string
	cfg  *config.LicenseSettings
}

func Run(m *module.Module) {
	o := &Copyright{
		name: "license",
		cfg:  &config.Cfg.LintersSettings.License,
	}

	logger.DebugF("Running linter `%s` on module `%s`", o.name, m.GetName())

	lintError := errors.NewError(o.name, m.GetName())

	if m.GetPath() == "" {
		return
	}

	files, err := getFiles(m.GetPath())
	if err != nil {
		lintError.WithValue(err.Error()).Add("error getting files in `%s` module", m.GetName())
		return
	}

	if !slices.Contains(o.cfg.SkipOssChecks, m.GetName()) {
		OssModuleRule(m.GetName(), m.GetPath())
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
			lintError.WithObjectID(path).WithValue(err).
				Add("errors in `%s` module", m.GetName())
		}
	}
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
