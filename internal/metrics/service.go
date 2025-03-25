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
	"context"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/promremote"
)

type PrometheusCollectorFunc func(ctx context.Context) (string, prometheus.Metric)

// Service is a metrics service
type Service interface {
	Send(ctx context.Context)
}

type PrometheusMetricsService struct {
	url   string
	token string

	client       promremote.Client
	metricsFuncs []PrometheusCollectorFunc
}

func NewPrometheusMetricsService(url, token string) *PrometheusMetricsService {
	if url == "" || token == "" {
		return nil
	}

	client, _ := promremote.NewClient(promremote.NewConfig(promremote.WriteURLOption(url)))

	return &PrometheusMetricsService{
		url:    url,
		token:  token,
		client: client,
	}
}

func (p *PrometheusMetricsService) AddMetrics(fns ...PrometheusCollectorFunc) {
	if p == nil {
		return
	}
	p.metricsFuncs = append(p.metricsFuncs, fns...)
}

func (p *PrometheusMetricsService) Send(ctx context.Context) {
	if p == nil {
		return
	}
	var timeSeries []promremote.TimeSeries
	for _, fn := range p.metricsFuncs {
		name, metric := fn(ctx)
		timeSeries = append(timeSeries, promremote.ConvertMetric(metric, name))
	}
	_, err := p.client.WriteTimeSeries(
		ctx,
		timeSeries,
		promremote.WriteOptions{
			Headers: map[string]string{
				"Authorization": "Bearer " + p.token,
			},
		},
	)
	if err != nil {
		logger.ErrorF("error in sending metrics: %v", err)
	}

}
