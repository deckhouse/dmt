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
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/pkg/config/global"
)

var (
	metrics   *PrometheusMetricsService
	startTime = time.Now()
)

func GetClient(dir string) *PrometheusMetricsService {
	if metrics != nil {
		return metrics
	}

	metrics = newPrometheusMetricsService(os.Getenv("DMT_METRICS_URL"), os.Getenv("DMT_METRICS_TOKEN"), dir)

	return metrics
}

func getDmtInfo(dir string) (string, string) {
	repository := cmp.Or(os.Getenv("DMT_REPOSITORY"), getRepositoryAddress(dir))
	if repository == "" {
		return "", ""
	}

	repositoryElements := strings.Split(repository, "/")
	repositoryID := repository
	if len(repositoryElements) > 1 {
		repositoryID = repositoryElements[len(repositoryElements)-1]
	}
	id := cmp.Or(os.Getenv("DMT_METRICS_ID"), repositoryID)

	return id, repository
}

func SetDmtInfo() {
	metrics.CounterAdd("dmt_info", 1, prometheus.Labels{
		"id":         metrics.id,
		"version":    flags.Version,
		"repository": metrics.repository,
	})
}

func SetLinterWarningsMetrics(cfg global.Global) {
	if cfg.Linters.Templates.IsWarn() {
		metrics.CounterAdd("dmt_linter_info", 1, prometheus.Labels{
			"id":     metrics.id,
			"linter": "templates",
			"level":  "warning",
		})
	}
	if cfg.Linters.Images.IsWarn() {
		metrics.CounterAdd("dmt_linter_info", 1, prometheus.Labels{
			"id":     metrics.id,
			"linter": "images",
			"level":  "warning",
		})
	}
	if cfg.Linters.Container.IsWarn() {
		metrics.CounterAdd("dmt_linter_info", 1, prometheus.Labels{
			"id":     metrics.id,
			"linter": "container",
			"level":  "warning",
		})
	}
	if cfg.Linters.Rbac.IsWarn() {
		metrics.CounterAdd("dmt_linter_info", 1, prometheus.Labels{
			"id":     metrics.id,
			"linter": "rbac",
			"level":  "warning",
		})
	}
	if cfg.Linters.Hooks.IsWarn() {
		metrics.CounterAdd("dmt_linter_info", 1, prometheus.Labels{
			"id":     metrics.id,
			"linter": "hooks",
			"level":  "warning",
		})
	}
	if cfg.Linters.Module.IsWarn() {
		metrics.CounterAdd("dmt_linter_info", 1, prometheus.Labels{
			"id":     metrics.id,
			"linter": "module",
			"level":  "warning",
		})
	}
	if cfg.Linters.OpenAPI.IsWarn() {
		metrics.CounterAdd("dmt_linter_info", 1, prometheus.Labels{
			"id":     metrics.id,
			"linter": "openapi",
			"level":  "warning",
		})
	}
	if cfg.Linters.NoCyrillic.IsWarn() {
		metrics.CounterAdd("dmt_linter_info", 1, prometheus.Labels{
			"id":     metrics.id,
			"linter": "no-cyrillic",
			"level":  "warning",
		})
	}
	if cfg.Linters.License.IsWarn() {
		metrics.CounterAdd("dmt_linter_info", 1, prometheus.Labels{
			"id":     metrics.id,
			"linter": "license",
			"level":  "warning",
		})
	}
}

func IncDmtLinterWarningsCount(linter, rule string) {
	metrics.CounterAdd("dmt_linter_check_count", 1, prometheus.Labels{
		"linter": linter,
		"rule":   rule,
		"id":     metrics.id,
		"level":  "warning"})
}

func SetDmtRuntimeDuration() {
	metrics.HistogramObserve(
		"dmt_runtime_duration",
		time.Since(startTime).Seconds(),
		prometheus.Labels{
			"id":         metrics.id,
			"repository": metrics.repository,
		},
		prometheus.DefBuckets)
}

func SetDmtRuntimeDurationSeconds() {
	metrics.GaugeSet(
		"dmt_runtime_duration_seconds",
		time.Since(startTime).Seconds(),
		prometheus.Labels{
			"id":         metrics.id,
			"repository": metrics.repository,
		})
}
