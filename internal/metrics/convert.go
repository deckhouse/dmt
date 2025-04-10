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
	"fmt"
	"slices"
	"time"

	"github.com/deckhouse/dmt/internal/promremote"
)

// GetTimeSeries converts Prometheus metric families to a map of promremote.TimeSeries
// where the key is the metric name and the value is a slice of time series for that metric
func (p *PrometheusMetricsService) GetTimeSeries() []promremote.TimeSeries {
	var series []promremote.TimeSeries

	metricFamilies, err := p.Gatherer.Gather()
	if err != nil {
		return nil
	}
	// Store timestamp for consistent use across series
	timestamp := time.Now()

	for _, metricFamily := range metricFamilies {
		metricName := metricFamily.GetName()
		for _, metric := range metricFamily.Metric {
			// Create labels from the metric's label pairs
			labels := make([]promremote.Label, 0, len(metric.Label)+1) // +1 for the name label
			for _, labelPair := range metric.Label {
				labels = append(labels, promremote.Label{
					Name:  labelPair.GetName(),
					Value: labelPair.GetValue(),
				})
			}

			// Extract value based on metric type
			switch {
			case metric.GetCounter() != nil:
				// Add counter as a single time series
				counterLabels := slices.Clone(labels)
				counterLabels = append(counterLabels, promremote.Label{
					Name:  "__name__",
					Value: metricName,
				})

				series = append(series, promremote.TimeSeries{
					Labels: counterLabels,
					Datapoint: promremote.Datapoint{
						Timestamp: timestamp,
						Value:     metric.GetCounter().GetValue(),
					},
				})

			case metric.GetGauge() != nil:
				// Add gauge as a single time series
				gaugeLabels := slices.Clone(labels)
				gaugeLabels = append(gaugeLabels, promremote.Label{
					Name:  "__name__",
					Value: metricName,
				})

				series = append(series, promremote.TimeSeries{
					Labels: gaugeLabels,
					Datapoint: promremote.Datapoint{
						Timestamp: timestamp,
						Value:     metric.GetGauge().GetValue(),
					},
				})

			case metric.GetHistogram() != nil:
				histogram := metric.GetHistogram()

				// 1. Add sum time series
				sumLabels := slices.Clone(labels)
				sumLabels = append(sumLabels, promremote.Label{
					Name:  "__name__",
					Value: metricName + "_sum",
				})

				series = append(series, promremote.TimeSeries{
					Labels: sumLabels,
					Datapoint: promremote.Datapoint{
						Timestamp: timestamp,
						Value:     histogram.GetSampleSum(),
					},
				})

				// 2. Add count time series
				countLabels := slices.Clone(labels)
				countLabels = append(countLabels, promremote.Label{
					Name:  "__name__",
					Value: metricName + "_count",
				})

				series = append(series, promremote.TimeSeries{
					Labels: countLabels,
					Datapoint: promremote.Datapoint{
						Timestamp: timestamp,
						Value:     float64(histogram.GetSampleCount()),
					},
				})

				// 3. Add bucket time series
				for _, bucket := range histogram.GetBucket() {
					bucketLabels := slices.Clone(labels)
					bucketLabels = append(bucketLabels,
						promremote.Label{
							Name:  "le",
							Value: fmt.Sprintf("%g", bucket.GetUpperBound()),
						},
						promremote.Label{
							Name:  "__name__",
							Value: metricName + "_bucket",
						},
					)

					series = append(series, promremote.TimeSeries{
						Labels: bucketLabels,
						Datapoint: promremote.Datapoint{
							Timestamp: timestamp,
							Value:     float64(bucket.GetCumulativeCount()),
						},
					})
				}

			case metric.GetSummary() != nil:
				summary := metric.GetSummary()

				// 1. Add sum time series
				sumLabels := slices.Clone(labels)
				sumLabels = append(sumLabels, promremote.Label{
					Name:  "__name__",
					Value: metricName + "_sum",
				})

				series = append(series, promremote.TimeSeries{
					Labels: sumLabels,
					Datapoint: promremote.Datapoint{
						Timestamp: timestamp,
						Value:     summary.GetSampleSum(),
					},
				})

				// 2. Add count time series
				countLabels := slices.Clone(labels)
				countLabels = append(countLabels, promremote.Label{
					Name:  "__name__",
					Value: metricName + "_count",
				})

				series = append(series, promremote.TimeSeries{
					Labels: countLabels,
					Datapoint: promremote.Datapoint{
						Timestamp: timestamp,
						Value:     float64(summary.GetSampleCount()),
					},
				})

				// 3. Add quantile time series
				for _, quantile := range summary.GetQuantile() {
					quantileLabels := slices.Clone(labels)
					quantileLabels = append(quantileLabels,
						promremote.Label{
							Name:  "quantile",
							Value: fmt.Sprintf("%g", quantile.GetQuantile()),
						},
						promremote.Label{
							Name:  "__name__",
							Value: metricName,
						},
					)

					series = append(series, promremote.TimeSeries{
						Labels: quantileLabels,
						Datapoint: promremote.Datapoint{
							Timestamp: timestamp,
							Value:     quantile.GetValue(),
						},
					})
				}
			}
		}
	}

	return series
}
