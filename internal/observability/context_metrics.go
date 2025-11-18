package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ContextMetrics tracks health of the layered context pipeline.
type ContextMetrics struct {
	staticCacheMiss prometheus.Counter
	snapshotErrors  prometheus.Counter
}

var (
	defaultContextMetrics     *ContextMetrics
	defaultContextMetricsOnce sync.Once
)

// NewContextMetrics builds a ContextMetrics recorder using the default registry.
func NewContextMetrics() *ContextMetrics {
	defaultContextMetricsOnce.Do(func() {
		defaultContextMetrics = newContextMetrics(prometheus.DefaultRegisterer)
	})
	return defaultContextMetrics
}

// NewContextMetricsWithRegisterer allows tests to provide a dedicated registry.
func NewContextMetricsWithRegisterer(reg prometheus.Registerer) *ContextMetrics {
	return newContextMetrics(reg)
}

func newContextMetrics(reg prometheus.Registerer) *ContextMetrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	factory := promauto.With(reg)
	return &ContextMetrics{
		staticCacheMiss: factory.NewCounter(prometheus.CounterOpts{
			Namespace: "alex",
			Subsystem: "context",
			Name:      "static_cache_miss_total",
			Help:      "Number of times the static context registry had to reload from disk",
		}),
		snapshotErrors: factory.NewCounter(prometheus.CounterOpts{
			Namespace: "alex",
			Subsystem: "context",
			Name:      "snapshot_error_total",
			Help:      "Number of failures when persisting session snapshots",
		}),
	}
}

// RecordStaticCacheMiss increments the static cache miss counter.
func (m *ContextMetrics) RecordStaticCacheMiss() {
	if m == nil || m.staticCacheMiss == nil {
		return
	}
	m.staticCacheMiss.Inc()
}

// RecordSnapshotError increments the snapshot error counter.
func (m *ContextMetrics) RecordSnapshotError() {
	if m == nil || m.snapshotErrors == nil {
		return
	}
	m.snapshotErrors.Inc()
}
