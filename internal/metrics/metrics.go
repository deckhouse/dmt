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

package metrics

import (
	"cmp"
	"context"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/promremote"
	"github.com/deckhouse/dmt/pkg/config/global"
)

var (
	metrics *PrometheusMetricsService
)

var (
	dmtInfo = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dmt_info",
		Help: "DMT info",
	}, []string{"version", "id", "repository"})

	dmtLinterWarningsCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dmt_linter_warnings_count",
		Help: "DMT linter warnings count",
	}, []string{"version", "linter", "rule"})

	dmtLinterWarnings = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dmt_linter_warnings",
		Help: "DMT linter warnings",
	}, []string{"version", "linter"})
)

func GetClient() *PrometheusMetricsService {
	if metrics != nil {
		return metrics
	}

	metrics = NewPrometheusMetricsService(os.Getenv("DMT_METRICS_URL"), os.Getenv("DMT_METRICS_TOKEN"))

	return metrics
}

func (p *PrometheusMetricsService) SetInfoMetric(dir string) {
	repository := cmp.Or(os.Getenv("DMT_REPOSITORY"), getRepositoryAddress(dir))
	if repository == "" {
		return
	}

	repositoryElements := strings.Split(repository, "/")
	repositoryID := repository
	if len(repositoryElements) > 1 {
		repositoryID = repositoryElements[len(repositoryElements)-1]
	}
	id := cmp.Or(os.Getenv("DMT_METRICS_ID"), repositoryID)

	p.Add("dmt_info", map[string]string{"version": flags.Version, "id": id, "repository": repository}, 1)
}

func IncLinterWarning(linter, rule string) {
	dmtLinterWarningsCount.With(prometheus.Labels{
		"version": flags.Version,
		"linter":  linter,
		"rule":    rule,
	}).Add(1)
}

func GetLinterWarningsMetrics(cfg global.Global) []PrometheusCollectorFunc {
	result := make([]PrometheusCollectorFunc, 0)
	if cfg.Linters.Templates.IsWarn() {
		c := dmtLinterWarnings.With(prometheus.Labels{"version": flags.Version, "linter": "templates"})
		c.Add(1)
		result = append(result, func(_ context.Context) (string, prometheus.Metric) {
			return "dmt_linter_warnings", c
		})
	}
	if cfg.Linters.Images.IsWarn() {
		c := dmtLinterWarnings.With(prometheus.Labels{"version": flags.Version, "linter": "images"})
		c.Add(1)
		result = append(result, func(_ context.Context) (string, prometheus.Metric) {
			return "dmt_linter_warnings", c
		})
	}
	if cfg.Linters.Container.IsWarn() {
		c := dmtLinterWarnings.With(prometheus.Labels{"version": flags.Version, "linter": "container"})
		c.Add(1)
		result = append(result, func(_ context.Context) (string, prometheus.Metric) {
			return "dmt_linter_warnings", c
		})
	}
	if cfg.Linters.Rbac.IsWarn() {
		c := dmtLinterWarnings.With(prometheus.Labels{"version": flags.Version, "linter": "rbac"})
		c.Add(1)
		result = append(result, func(_ context.Context) (string, prometheus.Metric) {
			return "dmt_linter_warnings", c
		})
	}
	if cfg.Linters.Hooks.IsWarn() {
		c := dmtLinterWarnings.With(prometheus.Labels{"version": flags.Version, "linter": "hooks"})
		c.Add(1)
		result = append(result, func(_ context.Context) (string, prometheus.Metric) {
			return "dmt_linter_warnings", c
		})
	}
	if cfg.Linters.Module.IsWarn() {
		c := dmtLinterWarnings.With(prometheus.Labels{"version": flags.Version, "linter": "module"})
		c.Add(1)
		result = append(result, func(_ context.Context) (string, prometheus.Metric) {
			return "dmt_linter_warnings", c
		})
	}
	if cfg.Linters.OpenAPI.IsWarn() {
		c := dmtLinterWarnings.With(prometheus.Labels{"version": flags.Version, "linter": "openapi"})
		c.Add(1)
		result = append(result, func(_ context.Context) (string, prometheus.Metric) {
			return "dmt_linter_warnings", c
		})
	}
	if cfg.Linters.NoCyrillic.IsWarn() {
		c := dmtLinterWarnings.With(prometheus.Labels{"version": flags.Version, "linter": "no-cyrillic"})
		c.Add(1)
		result = append(result, func(_ context.Context) (string, prometheus.Metric) {
			return "dmt_linter_warnings", c
		})
	}
	if cfg.Linters.License.IsWarn() {
		c := dmtLinterWarnings.With(prometheus.Labels{"version": flags.Version, "linter": "license"})
		c.Add(1)
		result = append(result, func(_ context.Context) (string, prometheus.Metric) {
			return "dmt_linter_warnings", c
		})
	}

	return result
}
