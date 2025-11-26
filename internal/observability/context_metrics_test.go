package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestContextMetricsRecordsTokensAndCompression(t *testing.T) {
        reg := prometheus.NewRegistry()
        metrics := NewContextMetricsWithRegisterer(reg)

	metrics.RecordTokensBySection("system", 42)
	metrics.RecordTokensBySection("dynamic", 15)
	metrics.RecordCompression("dynamic")
	metrics.RecordCompression("dynamic")

	if got := testutil.ToFloat64(metrics.tokensBySection.WithLabelValues("system")); got != 42 {
		t.Fatalf("expected system tokens gauge to be 42, got %v", got)
	}
	if got := testutil.ToFloat64(metrics.tokensBySection.WithLabelValues("dynamic")); got != 15 {
		t.Fatalf("expected dynamic tokens gauge to be 15, got %v", got)
	}
        if got := testutil.ToFloat64(metrics.compressions.WithLabelValues("dynamic")); got != 2 {
                t.Fatalf("expected dynamic compression counter to be 2, got %v", got)
        }
}

func TestContextMetricsRecordsMetaAndCacheMisses(t *testing.T) {
        reg := prometheus.NewRegistry()
        metrics := NewContextMetricsWithRegisterer(reg)

        metrics.RecordMetaUsage(true)
        metrics.RecordMetaUsage(false)
        metrics.RecordCacheMiss("envelope_hash")
        metrics.RecordCacheMiss("envelope_hash")

        if got := testutil.ToFloat64(metrics.metaHits); got != 1 {
                t.Fatalf("expected meta hit counter to be 1, got %v", got)
        }
        if got := testutil.ToFloat64(metrics.metaMisses); got != 1 {
                t.Fatalf("expected meta miss counter to be 1, got %v", got)
        }
        if got := testutil.ToFloat64(metrics.cacheMisses.WithLabelValues("envelope_hash")); got != 2 {
                t.Fatalf("expected envelope cache miss counter to be 2, got %v", got)
        }
}
