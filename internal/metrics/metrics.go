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
	"reflect"
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

func SetLinterWarningsMetrics(cfg *global.Global) {
	v := reflect.ValueOf(&cfg.Linters).Elem()
	for i := range v.NumField() {
		field := v.Field(i)
		fType := v.Type().Field(i)

		if field.CanInterface() {
			linterConfig, ok := field.Interface().(global.LinterConfig)
			if ok && linterConfig.IsWarn() {
				metrics.CounterAdd("dmt_linter_info", 1, prometheus.Labels{
					"id":     metrics.id,
					"linter": strings.ToLower(fType.Name),
					"level":  linterConfig.Impact,
				})
			}
		}
	}
}

func IncDmtLinterErrorsCount(linter, rule, level string) {
	metrics.CounterAdd("dmt_linter_check_count", 1, prometheus.Labels{
		"linter": linter,
		"rule":   rule,
		"id":     metrics.id,
		"level":  level})
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
