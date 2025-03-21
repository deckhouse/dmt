package promremote

import (
	"github.com/prometheus/client_golang/prometheus"
)

func convertMetric(metric prometheus.Metric) TimeSeries {
	var ts TimeSeries
	metric.Write(&ts)
	return ts
}
