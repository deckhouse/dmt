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
	"sort"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type metricStorage struct {
	Counters         map[string]*prometheus.CounterVec
	Gauges           map[string]*prometheus.GaugeVec
	Histograms       map[string]*prometheus.HistogramVec
	HistogramBuckets map[string][]float64

	countersLock   sync.RWMutex
	gaugesLock     sync.RWMutex
	histogramsLock sync.RWMutex

	Registry   *prometheus.Registry
	Gatherer   prometheus.Gatherer
	Registerer prometheus.Registerer
}

func newMetricStorage() *metricStorage {
	storage := &metricStorage{
		Gauges:           make(map[string]*prometheus.GaugeVec),
		Counters:         make(map[string]*prometheus.CounterVec),
		Histograms:       make(map[string]*prometheus.HistogramVec),
		HistogramBuckets: make(map[string][]float64),
	}

	storage.Registry = prometheus.NewRegistry()
	storage.Gatherer = storage.Registry
	storage.Registerer = storage.Registry

	return storage
}

// Gauges

func (m *metricStorage) GaugeSet(metric string, value float64, labels map[string]string) {
	if m == nil {
		return
	}
	m.gauge(metric, labels).With(labels).Set(value)
}

func (m *metricStorage) GaugeAdd(metric string, value float64, labels map[string]string) {
	if m == nil {
		return
	}
	m.gauge(metric, labels).With(labels).Add(value)
}

func (m *metricStorage) gauge(metric string, labels map[string]string) *prometheus.GaugeVec {
	m.gaugesLock.RLock()
	vec, ok := m.Gauges[metric]
	m.gaugesLock.RUnlock()
	if ok {
		return vec
	}

	return m.registerGauge(metric, labels)
}

func (m *metricStorage) registerGauge(metric string, labels map[string]string) *prometheus.GaugeVec {
	m.gaugesLock.Lock()
	defer m.gaugesLock.Unlock()
	// double check
	vec, ok := m.Gauges[metric]
	if ok {
		return vec
	}

	vec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metric,
			Help: metric,
		},
		labelNames(labels),
	)
	m.Registerer.MustRegister(vec)
	m.Gauges[metric] = vec
	return vec
}

// Counters

func (m *metricStorage) CounterAdd(metric string, value float64, labels map[string]string) {
	if m == nil {
		return
	}
	m.counter(metric, labels).With(labels).Add(value)
}

func (m *metricStorage) counter(metric string, labels map[string]string) *prometheus.CounterVec {
	m.countersLock.RLock()
	vec, ok := m.Counters[metric]
	m.countersLock.RUnlock()
	if ok {
		return vec
	}

	return m.registerCounter(metric, labels)
}

func (m *metricStorage) registerCounter(metric string, labels map[string]string) *prometheus.CounterVec {
	m.countersLock.Lock()
	defer m.countersLock.Unlock()
	// double check
	vec, ok := m.Counters[metric]
	if ok {
		return vec
	}

	vec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: metric,
			Help: metric,
		},
		labelNames(labels),
	)
	m.Registerer.MustRegister(vec)
	m.Counters[metric] = vec
	return vec
}

// Histograms

func (m *metricStorage) HistogramObserve(metric string, value float64, labels map[string]string, buckets []float64) {
	if m == nil {
		return
	}
	m.histogram(metric, labels, buckets).With(labels).Observe(value)
}

func (m *metricStorage) histogram(metric string, labels map[string]string, buckets []float64) *prometheus.HistogramVec {
	m.histogramsLock.RLock()
	vec, ok := m.Histograms[metric]
	m.histogramsLock.RUnlock()
	if ok {
		return vec
	}
	return m.registerHistogram(metric, labels, buckets)
}

func (m *metricStorage) registerHistogram(metric string, labels map[string]string, buckets []float64) *prometheus.HistogramVec {
	m.histogramsLock.Lock()
	defer m.histogramsLock.Unlock()
	// double check
	vec, ok := m.Histograms[metric]
	if ok {
		return vec
	}

	b, has := m.HistogramBuckets[metric]
	// This shouldn't happen except when entering this concurrently
	// If there are buckets for this histogram about to be registered, keep them
	// Otherwise, use the new buckets.
	// No need to check for nil or empty slice, as the p8s lib will use DefBuckets
	// (https://pkg.go.dev/github.com/prometheus/client_golang/prometheus#HistogramOpts)
	if has {
		buckets = b
	}

	vec = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    metric,
		Help:    metric,
		Buckets: buckets,
	}, labelNames(labels))

	m.Registerer.MustRegister(vec)
	m.Histograms[metric] = vec
	return vec
}

// labelNames returns sorted label keys
func labelNames(labels map[string]string) []string {
	names := make([]string, 0)
	for labelName := range labels {
		names = append(names, labelName)
	}
	sort.Strings(names)
	return names
}
