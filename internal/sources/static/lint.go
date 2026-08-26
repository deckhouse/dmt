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

	"dario.cat/mergo"
	"github.com/go-openapi/spec"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/manager"
	"github.com/deckhouse/dmt/internal/matrix"
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

	// matrix renders each module under every value combination its openapi schema
	// describes (see internal/matrix) instead of only the default one; matrixLimit
	// caps the combinations per module.
	matrix      bool
	matrixLimit int
}

var _ manager.Source = (*Source)(nil)

// Option customizes a Source at construction time.
type Option func(*Source)

// WithMatrix enables matrix mode with the given per-module combination limit. A
// non-positive limit falls back to the matrix package default. It is the
// programmatic equivalent of the --matrix / --matrix-limit flags, and lets callers
// (e.g. tests running cases in parallel) opt in without touching process-global
// flags.
func WithMatrix(enabled bool, limit int) Option {
	return func(s *Source) {
		s.matrix = enabled

		if limit > 0 {
			s.matrixLimit = limit
		}
	}
}

// NewSource lints every module found under dir.
func NewSource(dir string, opts ...Option) *Source {
	s := &Source{
		dir: dir,

		// Default to the process-global flags so the CLI keeps working; the options
		// override them for programmatic callers.
		matrix:      flags.Matrix,
		matrixLimit: flags.MatrixLimit,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
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
//
// Modules are handed to yield as they are built rather than collected: under
// --matrix one module becomes many renders, and yield blocks while the linters are
// busy, so only a bounded number of them is ever resident.
func (s *Source) Targets(
	_ context.Context,
	cfg *config.RootConfig,
	errorList *errors.LintRuleErrorsList,
	yield func(manager.Target) bool,
) error {
	paths, err := moduleloader.GetModulePaths(s.dir)
	if err != nil {
		log.Error("Error getting module paths", log.Err(err))

		return nil
	}

	vals, err := decodeValuesFile(flags.ValuesFile)
	if err != nil {
		log.Error("Failed to decode values file", log.Err(err))
	}

	globalValues, err := values.GetGlobalValues(getRootDirectory(s.dir))
	if err != nil {
		log.Error("Failed to get global values", log.Err(err))

		return nil
	}

	// Validate every module before rendering any, so the count below is the number
	// of modules that will actually be linted and is reported up front rather than
	// after the last (slow) render.
	valid := make([]string, 0, len(paths))

	for i := range paths {
		log.Debug("Found module", slog.String("module", filepath.Base(paths[i])))

		if err := validateModule(paths[i], errorList); err != nil {
			// linting errors are already logged
			continue
		}

		valid = append(valid, paths[i])
	}

	log.Info("Found modules", slog.Int("count", len(valid)))

	for _, path := range valid {
		if !s.push(path, vals, globalValues, cfg, errorList, yield) {
			return nil
		}
	}

	return nil
}

// push renders every variant of one module and hands each to yield, returning false
// once yield has asked the run to stop.
func (s *Source) push(
	path string,
	vals chartutil.Values,
	globalValues *spec.Schema,
	cfg *config.RootConfig,
	errorList *errors.LintRuleErrorsList,
	yield func(manager.Target) bool,
) bool {
	moduleName := filepath.Base(path)
	running := true

	s.forEachVariant(path, moduleName, errorList, func(v variant) bool {
		mdl := s.render(path, moduleName, v, vals, globalValues, cfg, errorList)
		if mdl == nil {
			return true
		}

		running = yield(manager.Target{
			Module: mdl,
			Scope:  scopes.Static,
			// The directory, not the name: two directories may declare the same
			// module name, and the summary must still count them separately.
			ModuleID: path,
			Variant:  s.matrix,
		})

		return running
	})

	return running
}

// variant is one render of a module: the value overrides that select which template
// branches are produced. Nil overrides are the default render — the one whose
// failure is a genuine module defect rather than an invalid value combination.
type variant struct {
	label     string
	overrides chartutil.Values
}

// forEachVariant invokes fn for each render of a module, generated lazily. Without
// --matrix that is the single default render (the generated values plus
// --values-file). With it, it is every value combination the module's openapi schema
// describes, the default among them, streamed one at a time so a schema that expands
// to millions of combinations is never materialized as a slice. fn returning false
// stops the iteration.
func (s *Source) forEachVariant(
	path, moduleName string,
	errorList *errors.LintRuleErrorsList,
	fn func(variant) bool,
) {
	if !s.matrix {
		fn(variant{})

		return
	}

	seq, count, err := matrix.Generate(path, "values.yaml", s.matrixLimit)
	if err != nil {
		errorList.WithFilePath(path).WithModule(moduleName).
			WithValue(err.Error()).
			Errorf("cannot expand matrix variants for module `%s`", moduleName)

		// Fall back to the default render so the module is still linted.
		fn(variant{})

		return
	}

	log.Info("Matrix variants for module",
		slog.String("module", moduleName), slog.Int("count", count))

	for v := range seq {
		if !fn(variant{label: v.Label, overrides: v.Overrides}) {
			return
		}
	}
}

// render builds the module for one variant. It returns nil when the variant cannot
// be rendered: a matrix variant (non-nil overrides) that fails is almost always an
// invalid value combination the chart rejects via `fail` — two mutually-exclusive
// parameters, say — so it is skipped quietly. Only the default render's failure is
// reported as a genuine "module doesn't build" finding.
func (s *Source) render(
	path, moduleName string,
	v variant,
	vals chartutil.Values,
	globalValues *spec.Schema,
	cfg *config.RootConfig,
	errorList *errors.LintRuleErrorsList,
) *modules.Module {
	merged := mergeValues(vals, v.overrides)

	mdl, err := modules.NewModule(path, &merged, globalValues, cfg, errorList)
	if err == nil {
		return mdl
	}

	if v.overrides != nil {
		log.Debug("skipping matrix variant that failed to render",
			slog.String("module", moduleName),
			slog.String("variant", v.label),
			slog.String("error", err.Error()),
		)

		return nil
	}

	errorList.
		WithFilePath(path).WithModule(moduleName).
		WithValue(err.Error()).
		Errorf("cannot create module `%s`", moduleName)

	return nil
}

// mergeValues returns a fresh value tree with override applied on top of base. base
// is read-only: one tree is shared by every render of every module, so a variant that
// wrote into it would hand its overrides to every variant rendered after it.
//
// The copy has to be deep. mergo assigns a nested map by reference when the
// destination does not have the key yet, so merging base into an empty tree and the
// override on top of that would reach through the shared sub-maps and write the
// override into base itself.
func mergeValues(base, override chartutil.Values) chartutil.Values {
	if len(override) == 0 {
		return base
	}

	out := make(chartutil.Values, len(base))
	for k, v := range base {
		out[k] = deepCopyValue(v)
	}

	// override is built fresh for this variant and used once, so the sub-trees mergo
	// takes from it by reference are not shared with anything.
	_ = mergo.Merge(&out, override, mergo.WithOverride)

	return out
}

// deepCopyValue copies the maps and slices of a value tree, leaving scalars alone.
func deepCopyValue(v any) any {
	switch t := v.(type) {
	case chartutil.Values:
		out := make(chartutil.Values, len(t))
		for k, val := range t {
			out[k] = deepCopyValue(val)
		}

		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = deepCopyValue(val)
		}

		return out
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = deepCopyValue(t[i])
		}

		return out
	default:
		return v
	}
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
