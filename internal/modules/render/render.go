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

package render

import (
	"context"
	"errors"
	"fmt"
	"io"
	stdlog "log"
	"os"
	"strings"

	"github.com/werf/nelm/pkg/action"
	"github.com/werf/nelm/pkg/common"
	"github.com/werf/nelm/pkg/helm/pkg/chart/loader"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func init() {
	// nelm's chart loader prints "Cannot automatically download chart
	// dependencies without Chart.lock or requirements.lock." to stdout when a
	// chart declares dependencies but ships no lock file. dmt only renders
	// templates and never resolves remote dependencies, so this is noise.
	loader.NoChartLockWarning = ""

	// nelm's chart loader also logs "found symbolic link in path: ..." via the
	// standard log package for every symlink a module ships (helm_lib and friends).
	// dmt logs through slog and never the stdlib logger, so silence it once here.
	// Doing it globally (rather than saving/restoring around each render) avoids a
	// race on the process-global logger under parallel per-module linting.
	stdlog.SetOutput(io.Discard)
}

const (
	// defaultChartVersion / defaultChartAPIVersion supply chart identity
	// (alongside the caller-provided name) when a deckhouse module omits Chart.yaml.
	defaultChartVersion    = "0.2.0"
	defaultChartAPIVersion = "v2"
)

// Object is a single manifest produced by a strict render: the parsed Kubernetes
// object plus the chart-relative path of the template it came from.
type Object struct {
	FilePath string
	*unstructured.Unstructured
}

// Options configures Render.
type Options struct {
	// Path is the chart directory to render.
	Path string
	// Values is the .Values tree fed to the chart (not the full render context —
	// action.ChartRender builds .Release/.Capabilities itself).
	Values map[string]any
	// ExtraAPIVersions extends .Capabilities.APIVersions (e.g. VPA, cert-manager,
	// gateway kinds) so templates gating on them render offline.
	ExtraAPIVersions []string
}

// Render renders the chart at opts.Path with nelm's public action.ChartRender,
// offline (no cluster) and strictly: the first template error aborts the render.
// This is the same public entrypoint deckhouse-controller and d8-package-plugin use,
// so dmt renders modules the way they are actually installed. nelm parses the
// manifests, so callers get []Object and need no manifest splitting.
//
// Before rendering it injects an image-resolution override (see injectImageStub) so
// image names dmt cannot know offline — werf-computed names, werf-defined aliases —
// still resolve instead of aborting the render, and removes it again afterwards.
func Render(ctx context.Context, namespace, releaseName string, opts Options) ([]Object, error) {
	if releaseName == "" {
		return nil, fmt.Errorf("helm chart must have a name")
	}

	cleanupStub, err := injectImageStub(opts.Path)
	if err != nil {
		return nil, err
	}
	defer cleanupStub()

	valuesFile, cleanup, err := writeTempValues(opts.Values)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	res, err := renderChart(ctx, namespace, releaseName, valuesFile, opts)
	if err != nil {
		return nil, err
	}

	objects := make([]Object, 0, len(res.Resources))
	for _, r := range res.Resources {
		// FilePath is chart-name-prefixed (e.g. "module/templates/foo.yaml");
		// drop the leading chart-name segment to match the on-disk layout.
		_, relPath, _ := strings.Cut(r.FilePath, "/")

		objects = append(objects, Object{
			FilePath:     relPath,
			Unstructured: r.Unstruct,
		})
	}

	return objects, nil
}

// renderChart wraps action.ChartRender. Loader log noise is silenced globally in
// init(), so no per-call logger juggling is needed here.
func renderChart(
	ctx context.Context,
	namespace, releaseName, valuesFile string,
	opts Options,
) (*action.ChartRenderResultV2, error) {
	res, err := action.ChartRender(ctx, action.ChartRenderOptions{
		Chart:                  opts.Path,
		DefaultChartName:       releaseName,
		DefaultChartVersion:    defaultChartVersion,
		DefaultChartAPIVersion: defaultChartAPIVersion,
		ReleaseName:            releaseName,
		ReleaseNamespace:       namespace,
		ExtraAPIVersions:       opts.ExtraAPIVersions,
		// OutputFilePath neutralises nelm's own stdout printer (OutputNoPrint is a
		// no-op in ChartRender); we read res.Resources directly.
		OutputFilePath: os.DevNull,
		ValuesOptions: common.ValuesOptions{
			ValuesFiles: []string{valuesFile},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("render chart: %w", err)
	}

	return res, nil
}

// writeTempValues marshals values to a temporary YAML file and returns its path
// plus a cleanup func the caller must defer once the render is done. A nil map is
// written as an empty document so action.ChartRender always receives a well-formed
// values file. On error the returned cleanup is nil — there is nothing to remove.
func writeTempValues(values map[string]any) (string, func(), error) {
	if values == nil {
		values = map[string]any{}
	}

	data, err := yaml.Marshal(values)
	if err != nil {
		return "", nil, fmt.Errorf("marshal values: %w", err)
	}

	f, err := os.CreateTemp("", "dmt-values-*.yaml")
	if err != nil {
		return "", nil, fmt.Errorf("create temp values file: %w", err)
	}

	cleanup := func() { _ = os.Remove(f.Name()) }

	_, writeErr := f.Write(data)
	closeErr := f.Close()

	if err := errors.Join(writeErr, closeErr); err != nil {
		cleanup()

		return "", nil, fmt.Errorf("write temp values file: %w", err)
	}

	return f.Name(), cleanup, nil
}
