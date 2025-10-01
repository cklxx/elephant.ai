package observability

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	promclient "github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// MetricsCollector manages all metrics for ALEX
type MetricsCollector struct {
	meter metric.Meter

	// LLM metrics
	llmRequests     metric.Int64Counter
	llmTokensInput  metric.Int64Counter
	llmTokensOutput metric.Int64Counter
	llmLatency      metric.Float64Histogram
	llmCost         metric.Float64Counter

	// Tool metrics
	toolExecutions metric.Int64Counter
	toolDuration   metric.Float64Histogram

	// Session metrics
	sessionsActive metric.Int64UpDownCounter

	// Server for Prometheus scraping
	prometheusServer *http.Server
}

// MetricsConfig configures the metrics collector
type MetricsConfig struct {
	Enabled        bool `yaml:"enabled"`
	PrometheusPort int  `yaml:"prometheus_port"`
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(config MetricsConfig) (*MetricsCollector, error) {
	if !config.Enabled {
		return &MetricsCollector{}, nil
	}

	// Create Prometheus exporter
	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	// Create meter provider
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)
	otel.SetMeterProvider(provider)

	// Get meter
	meter := provider.Meter("alex")

	// Create metrics
	llmRequests, err := meter.Int64Counter(
		"alex.llm.requests.total",
		metric.WithDescription("Total number of LLM requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create llm_requests counter: %w", err)
	}

	llmTokensInput, err := meter.Int64Counter(
		"alex.llm.tokens.input",
		metric.WithDescription("Total input tokens sent to LLM"),
		metric.WithUnit("{token}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create llm_tokens_input counter: %w", err)
	}

	llmTokensOutput, err := meter.Int64Counter(
		"alex.llm.tokens.output",
		metric.WithDescription("Total output tokens from LLM"),
		metric.WithUnit("{token}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create llm_tokens_output counter: %w", err)
	}

	llmLatency, err := meter.Float64Histogram(
		"alex.llm.latency",
		metric.WithDescription("LLM request latency in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create llm_latency histogram: %w", err)
	}

	llmCost, err := meter.Float64Counter(
		"alex.cost.total",
		metric.WithDescription("Total cost of LLM requests"),
		metric.WithUnit("USD"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create llm_cost counter: %w", err)
	}

	toolExecutions, err := meter.Int64Counter(
		"alex.tool.executions.total",
		metric.WithDescription("Total number of tool executions"),
		metric.WithUnit("{execution}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool_executions counter: %w", err)
	}

	toolDuration, err := meter.Float64Histogram(
		"alex.tool.duration",
		metric.WithDescription("Tool execution duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool_duration histogram: %w", err)
	}

	sessionsActive, err := meter.Int64UpDownCounter(
		"alex.sessions.active",
		metric.WithDescription("Number of active sessions"),
		metric.WithUnit("{session}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sessions_active gauge: %w", err)
	}

	collector := &MetricsCollector{
		meter:           meter,
		llmRequests:     llmRequests,
		llmTokensInput:  llmTokensInput,
		llmTokensOutput: llmTokensOutput,
		llmLatency:      llmLatency,
		llmCost:         llmCost,
		toolExecutions:  toolExecutions,
		toolDuration:    toolDuration,
		sessionsActive:  sessionsActive,
	}

	// Start Prometheus HTTP server
	if config.PrometheusPort > 0 {
		if err := collector.StartPrometheusServer(config.PrometheusPort); err != nil {
			return nil, fmt.Errorf("failed to start prometheus server: %w", err)
		}
	}

	return collector, nil
}

// StartPrometheusServer starts the Prometheus metrics server
func (m *MetricsCollector) StartPrometheusServer(port int) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promclient.Handler())

	m.prometheusServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		log.Printf("Prometheus metrics server listening on :%d", port)
		if err := m.prometheusServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Prometheus server error: %v", err)
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the metrics collector
func (m *MetricsCollector) Shutdown(ctx context.Context) error {
	if m.prometheusServer != nil {
		return m.prometheusServer.Shutdown(ctx)
	}
	return nil
}

// RecordLLMRequest records an LLM request
func (m *MetricsCollector) RecordLLMRequest(ctx context.Context, model string, status string, latency time.Duration, inputTokens, outputTokens int, cost float64) {
	if m.llmRequests == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("model", model),
		attribute.String("status", status),
	}

	m.llmRequests.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.llmTokensInput.Add(ctx, int64(inputTokens), metric.WithAttributes(attribute.String("model", model)))
	m.llmTokensOutput.Add(ctx, int64(outputTokens), metric.WithAttributes(attribute.String("model", model)))
	m.llmLatency.Record(ctx, latency.Seconds(), metric.WithAttributes(attrs...))
	if cost > 0 {
		m.llmCost.Add(ctx, cost, metric.WithAttributes(attribute.String("model", model)))
	}
}

// RecordToolExecution records a tool execution
func (m *MetricsCollector) RecordToolExecution(ctx context.Context, toolName string, status string, duration time.Duration) {
	if m.toolExecutions == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("tool_name", toolName),
		attribute.String("status", status),
	}

	m.toolExecutions.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.toolDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attribute.String("tool_name", toolName)))
}

// IncrementActiveSessions increments the active sessions counter
func (m *MetricsCollector) IncrementActiveSessions(ctx context.Context) {
	if m.sessionsActive == nil {
		return
	}
	m.sessionsActive.Add(ctx, 1)
}

// DecrementActiveSessions decrements the active sessions counter
func (m *MetricsCollector) DecrementActiveSessions(ctx context.Context) {
	if m.sessionsActive == nil {
		return
	}
	m.sessionsActive.Add(ctx, -1)
}

// EstimateCost estimates the cost of an LLM request (simplified)
// In production, use actual pricing from the provider
func EstimateCost(model string, inputTokens, outputTokens int) float64 {
	// Simplified pricing (per 1M tokens)
	// These are example prices and should be updated based on actual provider pricing
	prices := map[string]struct {
		input  float64
		output float64
	}{
		"gpt-4": {
			input:  30.0, // $30 per 1M input tokens
			output: 60.0, // $60 per 1M output tokens
		},
		"gpt-3.5-turbo": {
			input:  0.5, // $0.50 per 1M input tokens
			output: 1.5, // $1.50 per 1M output tokens
		},
		"claude-3-opus": {
			input:  15.0, // $15 per 1M input tokens
			output: 75.0, // $75 per 1M output tokens
		},
		"claude-3-sonnet": {
			input:  3.0,  // $3 per 1M input tokens
			output: 15.0, // $15 per 1M output tokens
		},
	}

	// Default pricing if model not found
	pricing, ok := prices[model]
	if !ok {
		pricing = struct {
			input  float64
			output float64
		}{input: 1.0, output: 2.0}
	}

	inputCost := (float64(inputTokens) / 1_000_000) * pricing.input
	outputCost := (float64(outputTokens) / 1_000_000) * pricing.output

	return inputCost + outputCost
}
