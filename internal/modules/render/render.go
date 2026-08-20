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
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/werf/nelm/pkg/action"
	"github.com/werf/nelm/pkg/common"
	"github.com/werf/nelm/pkg/helm/pkg/chart/loader"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/pkg/log"
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
	// OnDrop, if set, is called once for each template the tolerant render had to
	// neutralize to make the chart render: templatePath is the chart-relative path
	// (e.g. "templates/postgres.yaml") and renderErr is the abort that caused the
	// drop. Linting callers use it to surface a warning; others may leave it nil.
	OnDrop func(templatePath, renderErr string)
}

// Render renders the chart at opts.Path with nelm's public action.ChartRender,
// offline (no cluster). This is the same public entrypoint deckhouse-controller
// and d8-package-plugin use, so dmt renders modules the way they are actually
// installed. nelm parses the manifests, so callers get []Object and need no
// manifest splitting.
//
// Before rendering it injects an image-resolution override (see injectImageStub) so
// image names dmt cannot know offline — werf-computed names, werf-defined aliases —
// still resolve instead of aborting the render, and removes it again afterwards.
//
// The render is tolerant per template. A single manifest template that aborts the
// render — an intentional `fail`, or a `required` on a value dmt cannot supply
// offline — would otherwise abort the WHOLE chart and hide every other resource
// from the linters. Instead Render neutralizes just that one template and
// re-renders, so the remaining resources are still returned and linted; every
// neutralized template is restored before Render returns. An error that cannot be
// localized to a droppable manifest template (a failing partial, a values/schema
// error) is a genuine render failure and is surfaced to the caller.
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

	// Neutralized templates are restored once the render loop settles.
	neutralizer := newTemplateNeutralizer(opts.Path)
	defer neutralizer.restore()

	var res *action.ChartRenderResultV2

	for {
		var renderErr error

		res, renderErr = renderChart(ctx, namespace, releaseName, valuesFile, opts)
		if renderErr == nil {
			break
		}

		// Localize the failure to a droppable manifest template and blank just that
		// one, then re-render the rest. Give up (surface the error) when the failure
		// is not a droppable template or that template was already neutralized.
		rel := failingManifestTemplate(renderErr)
		if rel == "" || !neutralizer.neutralize(rel) {
			return nil, renderErr
		}

		if opts.OnDrop != nil {
			opts.OnDrop(rel, renderErr.Error())
		}

		log.Debug("dmt: template failed to render; dropping it and continuing with the rest of the chart",
			slog.String("chart", releaseName), slog.String("template", rel), log.Err(renderErr))
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

// failingTemplateRe extracts the chart-relative template path from a nelm render
// error. nelm formats a template execution failure as
//
//	... execution error at (<chart-name>/templates/foo.yaml:LINE:COL): <message>
//
// so the capture group holds "<chart-name>/templates/foo.yaml".
var failingTemplateRe = regexp.MustCompile(`\(([^():]+):\d+:\d+\)`)

// failingManifestTemplate returns the chart-dir-relative path (forward-slashed,
// e.g. "templates/postgres.yaml") of the manifest template a render error is
// localized to, or "" when the error cannot be attributed to a droppable manifest
// template. Only non-partial YAML templates under templates/ are droppable:
// blanking a partial (_*.tpl, helm_lib) or reacting to a values/schema error would
// not make the rest of the chart renderable, so those are surfaced to the caller.
func failingManifestTemplate(err error) string {
	if err == nil {
		return ""
	}

	m := failingTemplateRe.FindStringSubmatch(err.Error())
	if m == nil {
		return ""
	}

	// The path is chart-name-prefixed (e.g. "commander/templates/foo.yaml"); the
	// on-disk layout has no such prefix, so drop the leading segment.
	_, rel, found := strings.Cut(m[1], "/")
	if !found || !strings.HasPrefix(rel, "templates/") {
		return ""
	}

	if strings.HasPrefix(path.Base(rel), "_") {
		return ""
	}

	switch strings.ToLower(path.Ext(rel)) {
	case ".yaml", ".yml":
		return rel
	default:
		return ""
	}
}

// templateNeutralizer blanks manifest templates that abort the render so the rest
// of the chart can render, then restores each one (content and mode) afterwards.
type templateNeutralizer struct {
	chartDir string
	backups  map[string]neutralizedFile
}

type neutralizedFile struct {
	content []byte
	mode    os.FileMode
}

func newTemplateNeutralizer(chartDir string) *templateNeutralizer {
	return &templateNeutralizer{chartDir: chartDir, backups: map[string]neutralizedFile{}}
}

// neutralize replaces the template at rel (chart-relative, forward-slashed) with a
// comment-only file so it renders no resources. It returns false when the template
// was already neutralized (so the caller stops instead of looping) or the file
// cannot be read/written.
func (n *templateNeutralizer) neutralize(rel string) bool {
	if _, done := n.backups[rel]; done {
		return false
	}

	full := filepath.Join(n.chartDir, filepath.FromSlash(rel))

	info, err := os.Stat(full)
	if err != nil {
		return false
	}

	orig, err := os.ReadFile(full)
	if err != nil {
		return false
	}

	if err := os.WriteFile(full, []byte("# dmt: template neutralized after render failure\n"), info.Mode().Perm()); err != nil {
		return false
	}

	n.backups[rel] = neutralizedFile{content: orig, mode: info.Mode().Perm()}

	return true
}

// restore rewrites every neutralized template with its original content and mode.
func (n *templateNeutralizer) restore() {
	for rel, f := range n.backups {
		full := filepath.Join(n.chartDir, filepath.FromSlash(rel))
		_ = os.WriteFile(full, f.content, f.mode)
	}
}
