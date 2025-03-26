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
	"sync"
	"time"

	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/promremote"
)

type PrometheusMetricsService struct {
	url   string
	token string

	client promremote.Client

	mu         sync.Mutex
	timeSeries []promremote.TimeSeries
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

func (p *PrometheusMetricsService) Send(ctx context.Context) {
	if p == nil {
		return
	}
	_, err := p.client.WriteTimeSeries(
		ctx,
		p.timeSeries,
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

func (p *PrometheusMetricsService) AddTimeSeries(ts ...promremote.TimeSeries) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.timeSeries = append(p.timeSeries, ts...)
}

func (p *PrometheusMetricsService) Add(name string, labels map[string]string, value float64) {
	lbs := []promremote.Label{
		{Name: "__name__", Value: name},
	}

	for labelName, labelValue := range labels {
		lbs = append(lbs, promremote.Label{
			Name:  labelName,
			Value: labelValue,
		})
	}

	labels["__name__"] = name
	p.AddTimeSeries(promremote.TimeSeries{
		Labels: lbs,
		Datapoint: promremote.Datapoint{
			Timestamp: time.Now(),
			Value:     value,
		},
	})
}
