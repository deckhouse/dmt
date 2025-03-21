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

package promremote

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func ConvertMetric(metric prometheus.Metric, name string) TimeSeries {
	var dtoMetric dto.Metric
	err := metric.Write(&dtoMetric)
	if err != nil {
		return TimeSeries{}
	}

	labels := []Label{
		{Name: "__name__", Value: name},
	}

	for _, label := range dtoMetric.GetLabel() {
		labels = append(labels, Label{
			Name:  label.GetName(),
			Value: label.GetValue(),
		})
	}

	var value float64
	var timestamp time.Time

	if dtoMetric.Gauge != nil {
		value = dtoMetric.Gauge.GetValue()
	} else if dtoMetric.Counter != nil {
		value = dtoMetric.Counter.GetValue()
	} else if dtoMetric.Untyped != nil {
		value = dtoMetric.Untyped.GetValue()
	}

	if dtoMetric.TimestampMs != nil {
		timestamp = time.Unix(0, *dtoMetric.TimestampMs*int64(time.Millisecond))
	} else {
		timestamp = time.Now()
	}

	return TimeSeries{
		Labels: labels,
		Datapoint: Datapoint{
			Timestamp: timestamp,
			Value:     value,
		},
	}
}
