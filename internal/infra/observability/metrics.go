package observability

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"alex/internal/shared/async"
	"alex/internal/shared/logging"

	promclient "github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// MetricsCollector manages all metrics for ALEX.
type MetricsCollector struct {
	meter metric.Meter

	llmRequests     metric.Int64Counter
	llmTokensInput  metric.Int64Counter
	llmTokensOutput metric.Int64Counter
	llmLatency      metric.Float64Histogram
	llmCost         metric.Float64Counter

	toolExecutions metric.Int64Counter
	toolDuration   metric.Float64Histogram

	sessionsActive metric.Int64UpDownCounter

	httpRequests     metric.Int64Counter
	httpLatency      metric.Float64Histogram
	httpResponseSize metric.Int64Histogram

	sseConnections        metric.Int64UpDownCounter
	sseConnectionDuration metric.Float64Histogram
	sseMessages           metric.Int64Counter
	sseMessageBytes       metric.Int64Histogram

	taskExecutions metric.Int64Counter
	taskDuration   metric.Float64Histogram

	webVital metric.Float64Histogram

	nsmWTCR      metric.Float64Histogram
	nsmTimeSaved metric.Float64Histogram
	nsmAccuracy  metric.Float64Histogram

	leaderBlockerScans      metric.Int64Counter
	leaderBlockerDetected   metric.Int64Counter
	leaderBlockerNotified   metric.Int64Counter
	leaderPulseGenerations  metric.Int64Counter
	leaderPulseDuration     metric.Float64Histogram
	leaderPulseTaskCount    metric.Int64Histogram
	leaderAttentionDecision metric.Int64Counter
	leaderFocusSuppress     metric.Int64Counter
	leaderAlertOutcomes     metric.Int64Counter
	leaderAlertSendLatency  metric.Float64Histogram

	prometheusServer *http.Server
	testHooks        MetricsTestHooks
}

// MetricsTestHooks exposes callbacks that integration tests can use to assert
// instrumentation without spinning up a full OTel stack.
type MetricsTestHooks struct {
	LLMRequest        func(model, status string, latency time.Duration, inputTokens, outputTokens int, cost float64)
	HTTPServerRequest func(method, route string, status int, duration time.Duration, responseBytes int64)
	SSEMessage        func(eventType, status string, sizeBytes int64)
	TaskExecution     func(status string, duration time.Duration)
	BlockerScan       func(detected, notified int)
	PulseGeneration   func(taskCount int, duration time.Duration)
	AttentionDecision func(urgencyLevel string, suppressed bool)
	FocusTimeSuppress func(userID string)
	AlertOutcome      func(feature, channel, outcome string, latencyMs float64)
}

// SetTestHooks registers callbacks that are invoked whenever the matching
// metric is recorded. This is primarily used in unit tests so we can assert
// instrumentation without exporting real metrics.
func (m *MetricsCollector) SetTestHooks(hooks MetricsTestHooks) {
	if m == nil {
		return
	}
	m.testHooks = hooks
}

// MetricsConfig configures the metrics collector.
type MetricsConfig struct {
	Enabled        bool `yaml:"enabled"`
	PrometheusPort int  `yaml:"prometheus_port"`
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector(config MetricsConfig) (*MetricsCollector, error) {
	if !config.Enabled {
		return &MetricsCollector{}, nil
	}

	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	otel.SetMeterProvider(provider)

	collector := &MetricsCollector{meter: provider.Meter("alex")}
	for _, init := range []func() error{
		collector.initLLMMetrics,
		collector.initHTTPMetrics,
		collector.initSystemMetrics,
		collector.initLeaderMetrics,
	} {
		if err := init(); err != nil {
			return nil, err
		}
	}

	if config.PrometheusPort > 0 {
		if err := collector.StartPrometheusServer(config.PrometheusPort); err != nil {
			return nil, fmt.Errorf("failed to start prometheus server: %w", err)
		}
	}

	return collector, nil
}

// StartPrometheusServer starts the Prometheus metrics server.
func (m *MetricsCollector) StartPrometheusServer(port int) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promclient.Handler())

	m.prometheusServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	logger := logging.NewComponentLogger("PrometheusMetrics")
	async.Go(logger, "observability.prometheus", func() {
		log.Printf("Prometheus metrics server listening on :%d", port)
		if err := m.prometheusServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Prometheus server error: %v", err)
		}
	})

	return nil
}

// Shutdown gracefully shuts down the metrics collector.
func (m *MetricsCollector) Shutdown(ctx context.Context) error {
	if m.prometheusServer != nil {
		return m.prometheusServer.Shutdown(ctx)
	}
	return nil
}
