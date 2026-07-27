/*
Copyright 2026 Flant JSC

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

	"github.com/werf/nelm/pkg/helm/pkg/chart"
	"github.com/werf/nelm/pkg/helm/pkg/chart/loader"
	"github.com/werf/nelm/pkg/helm/pkg/chartutil"
	"github.com/werf/nelm/pkg/helm/pkg/engine"
	"github.com/werf/nelm/pkg/helm/pkg/werf/helmopts"
)

// EngineOption is a functional option for configuring NelmEngine.
type EngineOption func(*NelmEngine)

// NelmEngine is a reusable wrapper around werf/nelm for rendering Helm charts.
// Unlike the old Renderer, it does not own chart name / namespace / lint mode -
// those are passed per-render call. Engine-level concerns (log suppression,
// default options, DepDownloader) are injected via functional options.
type NelmEngine struct {
	chartLoadOpts helmopts.ChartLoadOptions
}

// NewEngine creates a NelmEngine with optional overrides.
func NewEngine(opts ...EngineOption) *NelmEngine {
	e := &NelmEngine{
		chartLoadOpts: helmopts.ChartLoadOptions{
			DefaultChartAPIVersion: "v2",
			DefaultChartVersion:    "0.2.0",
			DepDownloader:          &lintDepDownloader{},
		},
	}
	for _, o := range opts {
		o(e)
	}

	return e
}

// WithDepDownloader overrides the default lintDepDownloader.
func WithDepDownloader(d helmopts.DepDownloader) EngineOption {
	return func(e *NelmEngine) {
		e.chartLoadOpts.DepDownloader = d
	}
}

// WithChartLoadOption sets a ChartLoadOptions field directly.
func WithChartLoadOption(fn func(*helmopts.ChartLoadOptions)) EngineOption {
	return func(e *NelmEngine) {
		fn(&e.chartLoadOpts)
	}
}

// LoadChart loads the chart at chartDir using nelm's loader with the engine's
// default options. name is used as DefaultChartName when the chart lacks a
// Chart.yaml. Returns the loaded chart or an error.
func (e *NelmEngine) LoadChart(chartDir, name string) (*chart.Chart, error) {
	if name == "" {
		return nil, fmt.Errorf("helm chart must have a name")
	}

	opts := helmopts.HelmOptions{
		ChartLoadOpts: e.chartLoadOpts,
	}
	opts.ChartLoadOpts.DefaultChartName = name

	// Suppress nelm's indiscriminate symlink/Chart.lock logging during load.
	stdlogWriter := stdlog.Writer()

	stdlog.SetOutput(io.Discard)

	chrt, err := loader.LoadDir(chartDir, opts)

	stdlog.SetOutput(stdlogWriter)

	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}

	return chrt, nil
}

// RenderChart loads the chart at chartDir and renders it with the given values.
// name is used as DefaultChartName; lintMode controls strictness of rendering.
func (e *NelmEngine) RenderChart(chartDir, name string, values map[string]any, lintMode bool) (map[string]string, error) {
	chrt, err := e.LoadChart(chartDir, name)
	if err != nil {
		return nil, err
	}

	eng := engine.Engine{LintMode: lintMode}

	out, err := eng.Render(chrt, chartutil.Values(values), helmopts.HelmOptions{
		ChartLoadOpts: e.chartLoadOpts,
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}
