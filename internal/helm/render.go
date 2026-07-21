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
	"sync"

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

// stdlog muting state. nelm writes two kinds of noise straight to the standard
// log package:
//   - the chart loader's sympath.Walk logs "found symbolic link in path:..."
//     (deckhouse modules symlink helm_lib and other shared resources);
//   - the render engine logs "[INFO] Fail: <msg>" for every `fail` call in a
//     template, in addition to returning it as an error.
//
// Both are expected — the second especially in --matrix mode, where invalid
// value combinations legitimately hit `fail`.
var (
	stdlogMu      sync.Mutex
	stdlogDepth   int
	stdlogRestore io.Writer
)

// muteStdlog redirects the standard log package to io.Discard and returns a
// function that restores the previous output. Because renders run concurrently
// (the manager builds one module per matrix variant in a worker pool) and
// stdlog's output is process-global, muting is reference counted: the first
// active render redirects output and the last one restores it. Only the
// save/restore is guarded by the mutex — the render itself runs without holding
// it, so parallelism is preserved.
func muteStdlog() func() {
	stdlogMu.Lock()
	if stdlogDepth == 0 {
		stdlogRestore = stdlog.Writer()
		stdlog.SetOutput(io.Discard)
	}
	stdlogDepth++
	stdlogMu.Unlock()

	return func() {
		stdlogMu.Lock()
		stdlogDepth--
		if stdlogDepth == 0 {
			stdlog.SetOutput(stdlogRestore)
			stdlogRestore = nil
		}
		stdlogMu.Unlock()
	}
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
}

// RenderChartFromDir renders the chart located at chartDir with nelm's chart
// engine using the provided render values (the top-level context holding
// .Release, .Capabilities and .Values). It returns a map of chart-relative
// source file path to the rendered manifests.
func (r Renderer) RenderChartFromDir(chartDir string, values map[string]any) (map[string]string, error) {
	if r.Name == "" {
		return nil, fmt.Errorf("helm chart must have a name")
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

	// Mute std log for the whole load+render; the real errors are still returned.
	defer muteStdlog()()

	chrt, err := loader.LoadDir(chartDir, opts)
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
