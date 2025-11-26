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
	tokensBySection prometheus.GaugeVec
	compressions    prometheus.CounterVec
	metaHits        prometheus.Counter
	metaMisses      prometheus.Counter
	cacheMisses     prometheus.CounterVec
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
		tokensBySection: *factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "alex",
			Subsystem: "context",
			Name:      "tokens_by_section",
			Help:      "Approximate tokens per context section for the most recent window build",
		}, []string{"section"}),
		compressions: *factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: "alex",
			Subsystem: "context",
			Name:      "compression_total",
			Help:      "Total number of compression passes performed by section",
		}, []string{"section"}),
		metaHits: factory.NewCounter(prometheus.CounterOpts{
			Namespace: "alex",
			Subsystem: "context",
			Name:      "meta_hit_total",
			Help:      "Number of times stewarded meta entries were loaded for a persona",
		}),
		metaMisses: factory.NewCounter(prometheus.CounterOpts{
			Namespace: "alex",
			Subsystem: "context",
			Name:      "meta_miss_total",
			Help:      "Number of times stewarded meta entries were missing for a persona",
		}),
		cacheMisses: *factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: "alex",
			Subsystem: "context",
			Name:      "cache_miss_total",
			Help:      "Cache misses detected when envelope hashes drift or static data reloads",
		}, []string{"kind"}),
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

// RecordTokensBySection sets the latest token measurement for a section.
func (m *ContextMetrics) RecordTokensBySection(section string, tokens int) {
	if m == nil {
		return
	}
	gauge := m.tokensBySection.WithLabelValues(section)
	gauge.Set(float64(tokens))
}

// RecordCompression increments the compression counter for a section.
func (m *ContextMetrics) RecordCompression(section string) {
	if m == nil {
		return
	}
	counter := m.compressions.WithLabelValues(section)
	counter.Inc()
}

// RecordMetaUsage tracks whether a stewarded meta profile was present.
func (m *ContextMetrics) RecordMetaUsage(hit bool) {
	if m == nil {
		return
	}
	if hit {
		if m.metaHits != nil {
			m.metaHits.Inc()
		}
		return
	}
	if m.metaMisses != nil {
		m.metaMisses.Inc()
	}
}

// RecordCacheMiss increments a labeled cache miss counter.
func (m *ContextMetrics) RecordCacheMiss(kind string) {
	if m == nil {
		return
	}
	counter := m.cacheMisses.WithLabelValues(kind)
	counter.Inc()
}
