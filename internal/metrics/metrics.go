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

	"github.com/prometheus/client_golang/prometheus"

	"github.com/deckhouse/dmt/internal/flags"
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
)

func GetClient() *PrometheusMetricsService {
	if metrics != nil {
		return metrics
	}

	metrics = NewPrometheusMetricsService(os.Getenv("DMT_METRICS_URL"), os.Getenv("DMT_METRICS_TOKEN"))

	return metrics
}

func GetInfo(dir string) PrometheusCollectorFunc {
	return func(_ context.Context) (string, prometheus.Metric) {
		repository := cmp.Or(os.Getenv("DMT_REPOSITORY"), getRepositoryAddress(dir))
		if repository == "" {
			return "", nil
		}
		repositoryElements := strings.Split(repository, "/")
		repositoryID := repository
		if len(repositoryElements) > 1 {
			repositoryID = repositoryElements[len(repositoryElements)-1]
		}
		id := cmp.Or(os.Getenv("DMT_METRICS_ID"), repositoryID)

		c := dmtInfo.With(prometheus.Labels{
			"id":         id,
			"version":    flags.Version,
			"repository": repository,
		})
		c.Add(1)

		return "dmt_info", c
	}
}

var singletonLinterWarnings = make(map[string]struct{})

func IncLinterWarning(linter, rule string) {
	if _, ok := singletonLinterWarnings[linter+rule]; !ok {
		GetClient().AddMetrics(
			func(_ context.Context) (string, prometheus.Metric) {
				return "dmt_linter_warnings_count",
					dmtLinterWarningsCount.With(prometheus.Labels{"version": flags.Version, "linter": linter, "rule": rule})
			})
		singletonLinterWarnings[linter+rule] = struct{}{}
	}

	dmtLinterWarningsCount.With(prometheus.Labels{
		"version": flags.Version,
		"linter":  linter,
		"rule":    rule,
	}).Add(1)
}
