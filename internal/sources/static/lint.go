/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package static reads the modules to lint from a working tree: the full source
// as committed, rendered the way Deckhouse would render it.
package static

import (
	"context"
	"log/slog"
	"path/filepath"

	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/manager"
	"github.com/deckhouse/dmt/internal/moduleloader"
	"github.com/deckhouse/dmt/internal/modules"
	"github.com/deckhouse/dmt/internal/modules/values"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/scopes"
)

// Source reads modules from a directory on disk.
type Source struct {
	dir string
}

var _ manager.Source = (*Source)(nil)

// NewSource lints every module found under dir.
func NewSource(dir string) *Source {
	return &Source{dir: dir}
}

// ConfigDir is the linted directory itself: .dmtlint.yaml is looked up from the tree
// it configures.
func (s *Source) ConfigDir() string {
	return s.dir
}

func (s *Source) Scopes() []scopes.Scope {
	return []scopes.Scope{scopes.Static}
}

// Close has nothing to release: the modules are the caller's own files.
func (s *Source) Close() {}

// Targets walks the tree for modules and builds each one. A module that cannot be
// read is reported as a finding and skipped, not returned as an error: one broken
// module must not cost the caller the findings of every other.
func (s *Source) Targets(
	_ context.Context,
	cfg *config.RootConfig,
	errorList *errors.LintRuleErrorsList,
) ([]manager.Target, error) {
	paths, err := moduleloader.GetModulePaths(s.dir)
	if err != nil {
		log.Error("Error getting module paths", log.Err(err))

		return nil, nil
	}

	vals, err := decodeValuesFile(flags.ValuesFile)
	if err != nil {
		log.Error("Failed to decode values file", log.Err(err))
	}

	globalValues, err := values.GetGlobalValues(getRootDirectory(s.dir))
	if err != nil {
		log.Error("Failed to get global values", log.Err(err))

		return nil, nil
	}

	targets := make([]manager.Target, 0, len(paths))

	for i := range paths {
		moduleName := filepath.Base(paths[i])
		log.Debug("Found module", slog.String("module", moduleName))

		if err := validateModule(paths[i], errorList); err != nil {
			// linting errors are already logged
			continue
		}

		mdl, err := modules.NewModule(paths[i], &vals, globalValues, cfg, errorList)
		if err != nil {
			errorList.
				WithFilePath(paths[i]).WithModule(moduleName).
				WithValue(err.Error()).
				Errorf("cannot create module `%s`", moduleName)

			continue
		}

		targets = append(targets, manager.Target{
			Module: mdl,
			Scope:  scopes.Static,
			// The directory, not the name: two directories may declare the same
			// module name, and the summary must still count them separately.
			ModuleID: paths[i],
		})
	}

	return targets, nil
}

func decodeValuesFile(path string) (chartutil.Values, error) {
	if path == "" {
		return nil, nil
	}

	valuesFile, err := fsutils.ExpandDir(path)
	if err != nil {
		return nil, err
	}

	return chartutil.ReadValuesFile(valuesFile)
}

func getRootDirectory(dir string) string {
	for {
		if fsutils.IsDir(filepath.Join(dir, "global-hooks", "openapi")) &&
			fsutils.IsDir(filepath.Join(dir, "modules")) &&
			fsutils.IsFile(filepath.Join(dir, "global-hooks", "openapi", "config-values.yaml")) &&
			fsutils.IsFile(filepath.Join(dir, "global-hooks", "openapi", "values.yaml")) {
			return dir
		}

		parent := filepath.Dir(dir)
		if dir == parent || parent == "" {
			break
		}

		dir = parent
	}

	return ""
}
