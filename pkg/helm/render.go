package helm

import (
	"fmt"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
)

type Renderer struct {
	Name      string
	Namespace string
	LintMode  bool
}

func (r Renderer) RenderChartFromDir(dir, values string) (files map[string]string, err error) {
	c, err := loader.Load(dir)
	if err != nil {
		panic(fmt.Errorf("chart load from '%s': %w", dir, err))
	}
	return r.RenderChart(c, values)
}

func (r Renderer) RenderChart(c *chart.Chart, values string) (files map[string]string, err error) {
	vals, err := chartutil.ReadValues([]byte(values))
	if err != nil {
		return nil, fmt.Errorf("helm chart read raw values: %w", err)
	}

	releaseName := "release"
	if r.Name != "" {
		releaseName = r.Name
	}
	releaseNamespace := "default"
	if r.Namespace != "" {
		releaseNamespace = r.Namespace
	}
	releaseOptions := chartutil.ReleaseOptions{
		Name:      releaseName,
		Namespace: releaseNamespace,
		IsInstall: true,
		IsUpgrade: true,
	}

	caps := chartutil.DefaultCapabilities
	vers := []string(caps.APIVersions)

	var found bool
	for _, ver := range vers {
		found = ver == "autoscaling.k8s.io/v1/VerticalPodAutoscaler"
	}
	if !found {
		vers = append(vers, "autoscaling.k8s.io/v1/VerticalPodAutoscaler")
	}

	caps.APIVersions = vers

	valuesToRender, err := chartutil.ToRenderValues(c, vals, releaseOptions, nil)
	if err != nil {
		return nil, fmt.Errorf("helm chart prepare render values: %w", err)
	}

	return r.RenderChartFromRawValues(c, valuesToRender)
}

func (r Renderer) RenderChartFromRawValues(c *chart.Chart, values chartutil.Values) (files map[string]string, err error) {
	// render chart with prepared values
	var e engine.Engine
	e.LintMode = r.LintMode

	out, err := e.Render(c, values)
	if err != nil {
		return nil, fmt.Errorf("helm chart render: %w", err)
	}

	return out, nil
}
