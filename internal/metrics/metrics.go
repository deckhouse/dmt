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

func GetClient() *PrometheusMetricsService {
	if metrics != nil {
		return metrics
	}

	metrics = newPrometheusMetricsService(os.Getenv("DMT_METRICS_URL"), os.Getenv("DMT_METRICS_TOKEN"))

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

func SetDmtInfo(dir string) {
	id, repository := getDmtInfo(dir)
	metrics.CounterAdd("dmt_info", 1, prometheus.Labels{
		"id":         id,
		"version":    flags.Version,
		"repository": repository,
	})
}

func SetLinterWarningsMetrics(cfg global.Global) {
	if cfg.Linters.Templates.IsWarn() {
		metrics.CounterAdd("dmt_linter_warnings", 1, prometheus.Labels{
			"version": flags.Version,
			"linter":  "templates",
		})
	}
	if cfg.Linters.Images.IsWarn() {
		metrics.CounterAdd("dmt_linter_warnings", 1, prometheus.Labels{
			"version": flags.Version,
			"linter":  "images",
		})
	}
	if cfg.Linters.Container.IsWarn() {
		metrics.CounterAdd("dmt_linter_warnings", 1, prometheus.Labels{
			"version": flags.Version,
			"linter":  "container",
		})
	}
	if cfg.Linters.Rbac.IsWarn() {
		metrics.CounterAdd("dmt_linter_warnings", 1, prometheus.Labels{
			"version": flags.Version,
			"linter":  "rbac",
		})
	}
	if cfg.Linters.Hooks.IsWarn() {
		metrics.CounterAdd("dmt_linter_warnings", 1, prometheus.Labels{
			"version": flags.Version,
			"linter":  "hooks",
		})
	}
	if cfg.Linters.Module.IsWarn() {
		metrics.CounterAdd("dmt_linter_warnings", 1, prometheus.Labels{
			"version": flags.Version,
			"linter":  "module",
		})
	}
	if cfg.Linters.OpenAPI.IsWarn() {
		metrics.CounterAdd("dmt_linter_warnings", 1, prometheus.Labels{
			"version": flags.Version,
			"linter":  "openapi",
		})
	}
	if cfg.Linters.NoCyrillic.IsWarn() {
		metrics.CounterAdd("dmt_linter_warnings", 1, prometheus.Labels{
			"version": flags.Version,
			"linter":  "no-cyrillic",
		})
	}
	if cfg.Linters.License.IsWarn() {
		metrics.CounterAdd("dmt_linter_warnings", 1, prometheus.Labels{
			"version": flags.Version,
			"linter":  "license",
		})
	}
}

func IncDmtLinterWarningsCount(linter, rule string) {
	metrics.CounterAdd("dmt_linter_warnings_count", 1, prometheus.Labels{
		"version": flags.Version,
		"linter":  linter,
		"rule":    rule,
	})
}

func SetDmtRuntimeDuration(dir string) {
	id, repository := getDmtInfo(dir)
	metrics.HistogramObserve(
		"dmt_runtime_duration",
		time.Since(startTime).Seconds(),
		prometheus.Labels{
			"version":    flags.Version,
			"id":         id,
			"repository": repository,
		},
		prometheus.DefBuckets)
}
