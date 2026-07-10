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

package helm

import (
	"fmt"
	"io"
	stdlog "log"
	"path"

	"github.com/werf/nelm/pkg/helm/pkg/chart"
	"github.com/werf/nelm/pkg/helm/pkg/chart/loader"
	"github.com/werf/nelm/pkg/helm/pkg/chartutil"
	"github.com/werf/nelm/pkg/helm/pkg/engine"
	"github.com/werf/nelm/pkg/helm/pkg/werf/helmopts"
)

func init() {
	// nelm's chart loader prints "Cannot automatically download chart
	// dependencies without Chart.lock or requirements.lock." straight to stdout
	// when a module's Chart.yaml declares dependencies but ships no lock file.
	// dmt only lints rendered templates and never resolves remote dependencies,
	// so this warning is noise that pollutes scan output. Suppress it.
	loader.NoChartLockWarning = ""
}

type Renderer struct {
	Name      string
	Namespace string
	LintMode  bool

	// HelmLibOverrides replaces the data of helper templates living under a
	// chart's templates/ directory, keyed by the template's base file name
	// (e.g. "_module_image.tpl"). It lets dmt inject deterministic stubs for the
	// helm_lib helpers so image and module-name references resolve to stable
	// values during linting. Overrides are applied to the chart and all of its
	// dependencies.
	HelmLibOverrides map[string][]byte

	// OnSymlinkLoop, when set, is called once for each symbolic link loop that
	// was found in the chart directory and skipped while preparing a cleaned
	// copy for rendering. It lets the caller surface the loop as a lint finding
	// instead of silently dropping it. path is the offending link; resolved is
	// the ancestor directory it points back to.
	OnSymlinkLoop func(path, resolved string)
}

// RenderChartFromDir renders the chart located at chartDir with nelm's chart
// engine using the provided render values (the top-level context holding
// .Release, .Capabilities and .Values). It returns a map of chart-relative
// source file path to the rendered manifests.
func (r Renderer) RenderChartFromDir(chartDir string, values map[string]any) (map[string]string, error) {
	if r.Name == "" {
		return nil, fmt.Errorf("helm chart must have a name")
	}

	// nelm's chart loader reads every file in the directory into memory and
	// follows symlinks without cycle detection, so a symlink loop makes it
	// exhaust memory and abort the whole process with a fatal out-of-memory
	// error. Pre-flight the directory: on a symlink loop, render a cleaned copy
	// (loops skipped, symlinks dereferenced) so linting still proceeds; an
	// oversized tree is rejected outright since cleaning cannot make it safe.
	loadDir := chartDir

	hasLoop, err := scanChartDir(chartDir)
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}

	if hasLoop {
		cleaned, cleanup, loops, err := materializeCleanChartDir(chartDir)
		if err != nil {
			return nil, fmt.Errorf("load chart: prepare cleaned chart dir: %w", err)
		}

		defer cleanup()

		if r.OnSymlinkLoop != nil {
			for _, l := range loops {
				r.OnSymlinkLoop(l.Path, l.Resolved)
			}
		}

		loadDir = cleaned
	}

	opts := helmopts.HelmOptions{
		ChartLoadOpts: helmopts.ChartLoadOptions{
			// deckhouse modules may omit Chart.yaml; provide sane defaults so the
			// directory still loads as a chart.
			DefaultChartAPIVersion: "v2",
			DefaultChartName:       r.Name,
			DefaultChartVersion:    "0.2.0",
			// Nelm's chart loader calls DepDownloader.SetChartPath / Build when
			// the chart has a Chart.lock with external (non-file://) dependencies.
			// Leave it nil and the loader panics with a nil pointer dereference.
			DepDownloader: &lintDepDownloader{},
		},
	}

	// nelm's chart loader calls sympath.Walk which indiscriminately logs
	// "found symbolic link in path: %s resolves to %s" via the standard
	// log package. Deckhouse modules use symlinks for helm_lib and other
	// shared resources; those messages are expected noise. Mute std log
	// during the load call and restore it afterwards.
	stdlogWriter := stdlog.Writer()

	stdlog.SetOutput(io.Discard)

	chrt, err := loader.LoadDir(loadDir, opts)

	stdlog.SetOutput(stdlogWriter)

	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}

	r.applyTemplateOverrides(chrt)

	e := engine.Engine{LintMode: r.LintMode}

	out, err := e.Render(chrt, chartutil.Values(values), opts)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// applyTemplateOverrides replaces helper template data in the chart and all of
// its dependencies according to r.HelmLibOverrides.
func (r Renderer) applyTemplateOverrides(chrt *chart.Chart) {
	if len(r.HelmLibOverrides) == 0 {
		return
	}

	for _, tpl := range chrt.Templates {
		if tpl == nil || path.Dir(tpl.Name) != "templates" {
			continue
		}

		if data, ok := r.HelmLibOverrides[path.Base(tpl.Name)]; ok {
			tpl.Data = data
		}
	}

	for _, dep := range chrt.Dependencies() {
		r.applyTemplateOverrides(dep)
	}
}

// lintDepDownloader is a minimal implementation of helmopts.DepDownloader used
// during lint. Nelm's loader requires a non-nil DepDownloader when a chart has
// a Chart.lock with external dependencies, but dmt does not fetch charts from
// remote repositories. Returning nil lets lint continue without the external
// dependencies instead of panicking on a nil pointer dereference.
type lintDepDownloader struct{}

func (d *lintDepDownloader) SetChartPath(path string) {}

func (d *lintDepDownloader) Build(opts helmopts.HelmOptions) error {
	return nil
}

func (d *lintDepDownloader) Update(opts helmopts.HelmOptions) error {
	return nil
}

func (d *lintDepDownloader) UpdateRepositories() error {
	return nil
}
