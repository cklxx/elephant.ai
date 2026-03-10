package observability

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"alex/internal/shared/async"
	"alex/internal/shared/logging"
	"alex/internal/shared/modelregistry"
	"alex/internal/shared/notification"

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

	// HTTP server metrics
	httpRequests     metric.Int64Counter
	httpLatency      metric.Float64Histogram
	httpResponseSize metric.Int64Histogram

	// SSE metrics
	sseConnections        metric.Int64UpDownCounter
	sseConnectionDuration metric.Float64Histogram
	sseMessages           metric.Int64Counter
	sseMessageBytes       metric.Int64Histogram

	// Task metrics
	taskExecutions metric.Int64Counter
	taskDuration   metric.Float64Histogram

	// Frontend performance metrics
	webVital metric.Float64Histogram

	// NSM (North Star Metrics)
	nsmWTCR      metric.Float64Histogram // Willing-To-Come-back Rate (0-1)
	nsmTimeSaved metric.Float64Histogram // Estimated seconds saved per interaction
	nsmAccuracy  metric.Float64Histogram // Correctness score (0-1) per action

	// Leader agent metrics
	leaderBlockerScans      metric.Int64Counter
	leaderBlockerDetected   metric.Int64Counter
	leaderBlockerNotified   metric.Int64Counter
	leaderPulseGenerations  metric.Int64Counter
	leaderPulseDuration     metric.Float64Histogram
	leaderPulseTaskCount    metric.Int64Histogram
	leaderAttentionDecision metric.Int64Counter
	leaderFocusSuppress     metric.Int64Counter

	// Leader alert outcome metrics
	leaderAlertOutcomes      metric.Int64Counter
	leaderAlertSendLatency   metric.Float64Histogram

	// Server for Prometheus scraping
	prometheusServer *http.Server

	// Optional callbacks used by tests to assert instrumentation behavior
	testHooks MetricsTestHooks
}

// MetricsTestHooks exposes callbacks that integration tests can use to assert
// instrumentation without spinning up a full OTel stack.
type MetricsTestHooks struct {
	HTTPServerRequest    func(method, route string, status int, duration time.Duration, responseBytes int64)
	SSEMessage           func(eventType, status string, sizeBytes int64)
	TaskExecution        func(status string, duration time.Duration)
	BlockerScan          func(detected, notified int)
	PulseGeneration      func(taskCount int, duration time.Duration)
	AttentionDecision    func(urgencyLevel string, suppressed bool)
	FocusTimeSuppress    func(userID string)
	AlertOutcome         func(feature, channel, outcome string, latencyMs float64)
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

	httpRequests, err := meter.Int64Counter(
		"alex.http.requests.total",
		metric.WithDescription("Total HTTP requests handled by the server"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http_requests counter: %w", err)
	}

	httpLatency, err := meter.Float64Histogram(
		"alex.http.latency",
		metric.WithDescription("HTTP request latency in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http_latency histogram: %w", err)
	}

	httpResponseSize, err := meter.Int64Histogram(
		"alex.http.response.size",
		metric.WithDescription("HTTP response payload sizes in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http_response_size histogram: %w", err)
	}

	sseConnections, err := meter.Int64UpDownCounter(
		"alex.sse.connections.active",
		metric.WithDescription("Active SSE connections"),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sse_connections gauge: %w", err)
	}

	sseConnectionDuration, err := meter.Float64Histogram(
		"alex.sse.connection.duration",
		metric.WithDescription("SSE connection lifetimes in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sse_connection_duration histogram: %w", err)
	}

	sseMessages, err := meter.Int64Counter(
		"alex.sse.messages.total",
		metric.WithDescription("Total SSE events delivered"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sse_messages counter: %w", err)
	}

	sseMessageBytes, err := meter.Int64Histogram(
		"alex.sse.message.size",
		metric.WithDescription("SSE payload sizes in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sse_message_size histogram: %w", err)
	}

	taskExecutions, err := meter.Int64Counter(
		"alex.tasks.executions.total",
		metric.WithDescription("Total background task executions"),
		metric.WithUnit("{execution}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create task_executions counter: %w", err)
	}

	taskDuration, err := meter.Float64Histogram(
		"alex.tasks.execution.duration",
		metric.WithDescription("Task execution duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create task_duration histogram: %w", err)
	}

	webVital, err := meter.Float64Histogram(
		"alex.frontend.web_vital",
		metric.WithDescription("Reported frontend web vital values"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create web_vital histogram: %w", err)
	}

	// NSM (North Star Metrics)
	nsmWTCR, err := meter.Float64Histogram(
		"alex.nsm.wtcr",
		metric.WithDescription("Willing-To-Come-back Rate per session (0-1)"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create nsm_wtcr histogram: %w", err)
	}

	nsmTimeSaved, err := meter.Float64Histogram(
		"alex.nsm.time_saved",
		metric.WithDescription("Estimated time saved per interaction in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create nsm_time_saved histogram: %w", err)
	}

	nsmAccuracy, err := meter.Float64Histogram(
		"alex.nsm.accuracy",
		metric.WithDescription("Correctness score per action (0-1)"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create nsm_accuracy histogram: %w", err)
	}

	// Leader agent metrics
	leaderBlockerScans, err := meter.Int64Counter(
		"alex.leader.blocker.scans.total",
		metric.WithDescription("Total blocker radar scans"),
		metric.WithUnit("{scan}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create leader_blocker_scans counter: %w", err)
	}

	leaderBlockerDetected, err := meter.Int64Counter(
		"alex.leader.blocker.detected.total",
		metric.WithDescription("Total blocked tasks detected"),
		metric.WithUnit("{task}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create leader_blocker_detected counter: %w", err)
	}

	leaderBlockerNotified, err := meter.Int64Counter(
		"alex.leader.blocker.notified.total",
		metric.WithDescription("Total blocker notifications sent"),
		metric.WithUnit("{notification}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create leader_blocker_notified counter: %w", err)
	}

	leaderPulseGenerations, err := meter.Int64Counter(
		"alex.leader.pulse.generations.total",
		metric.WithDescription("Total weekly pulse generations"),
		metric.WithUnit("{generation}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create leader_pulse_generations counter: %w", err)
	}

	leaderPulseDuration, err := meter.Float64Histogram(
		"alex.leader.pulse.duration",
		metric.WithDescription("Pulse generation duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create leader_pulse_duration histogram: %w", err)
	}

	leaderPulseTaskCount, err := meter.Int64Histogram(
		"alex.leader.pulse.task_count",
		metric.WithDescription("Number of tasks included in pulse digest"),
		metric.WithUnit("{task}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create leader_pulse_task_count histogram: %w", err)
	}

	leaderAttentionDecision, err := meter.Int64Counter(
		"alex.leader.attention.decisions.total",
		metric.WithDescription("Total attention gate dispatch decisions"),
		metric.WithUnit("{decision}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create leader_attention_decision counter: %w", err)
	}

	leaderFocusSuppress, err := meter.Int64Counter(
		"alex.leader.focus.suppressions.total",
		metric.WithDescription("Total messages suppressed during focus time"),
		metric.WithUnit("{suppression}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create leader_focus_suppress counter: %w", err)
	}

	leaderAlertOutcomes, err := meter.Int64Counter(
		"alex.leader.alert.outcomes.total",
		metric.WithDescription("Leader notification lifecycle outcomes (sent, delivered, failed, opened, dismissed, acted_on)"),
		metric.WithUnit("{outcome}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create leader_alert_outcomes counter: %w", err)
	}

	leaderAlertSendLatency, err := meter.Float64Histogram(
		"alex.leader.alert.send.latency",
		metric.WithDescription("Leader alert send latency in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create leader_alert_send_latency histogram: %w", err)
	}

	collector := &MetricsCollector{
		meter:                 meter,
		llmRequests:           llmRequests,
		llmTokensInput:        llmTokensInput,
		llmTokensOutput:       llmTokensOutput,
		llmLatency:            llmLatency,
		llmCost:               llmCost,
		toolExecutions:        toolExecutions,
		toolDuration:          toolDuration,
		sessionsActive:        sessionsActive,
		httpRequests:          httpRequests,
		httpLatency:           httpLatency,
		httpResponseSize:      httpResponseSize,
		sseConnections:        sseConnections,
		sseConnectionDuration: sseConnectionDuration,
		sseMessages:           sseMessages,
		sseMessageBytes:       sseMessageBytes,
		taskExecutions:        taskExecutions,
		taskDuration:          taskDuration,
		webVital:              webVital,
		nsmWTCR:                nsmWTCR,
		nsmTimeSaved:           nsmTimeSaved,
		nsmAccuracy:            nsmAccuracy,
		leaderBlockerScans:     leaderBlockerScans,
		leaderBlockerDetected:  leaderBlockerDetected,
		leaderBlockerNotified:  leaderBlockerNotified,
		leaderPulseGenerations: leaderPulseGenerations,
		leaderPulseDuration:    leaderPulseDuration,
		leaderPulseTaskCount:   leaderPulseTaskCount,
		leaderAttentionDecision:  leaderAttentionDecision,
		leaderFocusSuppress:     leaderFocusSuppress,
		leaderAlertOutcomes:     leaderAlertOutcomes,
		leaderAlertSendLatency:  leaderAlertSendLatency,
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

	logger := logging.NewComponentLogger("PrometheusMetrics")
	async.Go(logger, "observability.prometheus", func() {
		log.Printf("Prometheus metrics server listening on :%d", port)
		if err := m.prometheusServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Prometheus server error: %v", err)
		}
	})

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

// RecordHTTPServerRequest records metrics for an HTTP request lifecycle

func (m *MetricsCollector) RecordHTTPServerRequest(ctx context.Context, method, route string, status int, duration time.Duration, responseBytes int64) {
	if m == nil {
		return
	}
	if hook := m.testHooks.HTTPServerRequest; hook != nil {
		hook(method, route, status, duration, responseBytes)
	}
	if m.httpRequests == nil || m.httpLatency == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("http.method", method),
		attribute.String("http.route", route),
		attribute.Int("http.status_code", status),
	}
	m.httpRequests.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.httpLatency.Record(ctx, duration.Seconds(), metric.WithAttributes(
		attribute.String("http.method", method),
		attribute.String("http.route", route),
	))
	if m.httpResponseSize != nil && responseBytes >= 0 {
		m.httpResponseSize.Record(ctx, responseBytes, metric.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.route", route),
		))
	}
}

// IncrementSSEConnections increments the active SSE connection gauge
func (m *MetricsCollector) IncrementSSEConnections(ctx context.Context) {
	if m.sseConnections == nil {
		return
	}
	m.sseConnections.Add(ctx, 1)
}

// DecrementSSEConnections decrements the active SSE connection gauge
func (m *MetricsCollector) DecrementSSEConnections(ctx context.Context) {
	if m.sseConnections == nil {
		return
	}
	m.sseConnections.Add(ctx, -1)
}

// RecordSSEConnectionDuration records how long an SSE connection stayed open
func (m *MetricsCollector) RecordSSEConnectionDuration(ctx context.Context, duration time.Duration) {
	if m.sseConnectionDuration == nil {
		return
	}
	m.sseConnectionDuration.Record(ctx, duration.Seconds())
}

// RecordSSEMessage records an SSE event delivery attempt.
func (m *MetricsCollector) RecordSSEMessage(ctx context.Context, eventType, status string, sizeBytes int64) {
	if m == nil {
		return
	}
	if hook := m.testHooks.SSEMessage; hook != nil {
		hook(eventType, status, sizeBytes)
	}
	if m.sseMessages == nil {
		return
	}
	attrs := []attribute.KeyValue{attribute.String("event_type", eventType)}
	if status != "" {
		attrs = append(attrs, attribute.String("status", status))
	}
	m.sseMessages.Add(ctx, 1, metric.WithAttributes(attrs...))
	if m.sseMessageBytes != nil && sizeBytes > 0 {
		m.sseMessageBytes.Record(ctx, sizeBytes, metric.WithAttributes(attribute.String("event_type", eventType)))
	}
}

// RecordTaskExecution records task execution metrics
func (m *MetricsCollector) RecordTaskExecution(ctx context.Context, status string, duration time.Duration) {
	if m == nil {
		return
	}
	if hook := m.testHooks.TaskExecution; hook != nil {
		hook(status, duration)
	}
	if m.taskExecutions == nil || m.taskDuration == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("status", status),
	}
	m.taskExecutions.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.taskDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordWebVital stores reported frontend performance metrics
func (m *MetricsCollector) RecordWebVital(ctx context.Context, name, label, page string, value, delta float64) {
	if m.webVital == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("name", name),
	}
	if label != "" {
		attrs = append(attrs, attribute.String("label", label))
	}
	if page != "" {
		attrs = append(attrs, attribute.String("page", page))
	}
	if delta != 0 {
		attrs = append(attrs, attribute.Float64("delta", delta))
	}
	m.webVital.Record(ctx, value, metric.WithAttributes(attrs...))
}

// RecordNSMWTCR records a Willing-To-Come-back Rate observation for a session.
// value should be in [0, 1].
func (m *MetricsCollector) RecordNSMWTCR(ctx context.Context, sessionID string, value float64) {
	if m == nil || m.nsmWTCR == nil {
		return
	}
	m.nsmWTCR.Record(ctx, value, metric.WithAttributes(
		attribute.String("session_id", sessionID),
	))
}

// RecordNSMTimeSaved records estimated time saved (seconds) for an interaction.
func (m *MetricsCollector) RecordNSMTimeSaved(ctx context.Context, taskType string, seconds float64) {
	if m == nil || m.nsmTimeSaved == nil {
		return
	}
	m.nsmTimeSaved.Record(ctx, seconds, metric.WithAttributes(
		attribute.String("task_type", taskType),
	))
}

// RecordNSMAccuracy records a correctness score for a completed action.
// value should be in [0, 1].
func (m *MetricsCollector) RecordNSMAccuracy(ctx context.Context, toolName string, value float64) {
	if m == nil || m.nsmAccuracy == nil {
		return
	}
	m.nsmAccuracy.Record(ctx, value, metric.WithAttributes(
		attribute.String("tool_name", toolName),
	))
}

// RecordBlockerScan records a blocker radar scan with detected/notified counts.
func (m *MetricsCollector) RecordBlockerScan(ctx context.Context, detected, notified int) {
	if m == nil {
		return
	}
	if hook := m.testHooks.BlockerScan; hook != nil {
		hook(detected, notified)
	}
	if m.leaderBlockerScans == nil {
		return
	}
	m.leaderBlockerScans.Add(ctx, 1)
	m.leaderBlockerDetected.Add(ctx, int64(detected))
	m.leaderBlockerNotified.Add(ctx, int64(notified))
}

// RecordPulseGeneration records a weekly pulse generation with task count and duration.
func (m *MetricsCollector) RecordPulseGeneration(ctx context.Context, taskCount int, duration time.Duration) {
	if m == nil {
		return
	}
	if hook := m.testHooks.PulseGeneration; hook != nil {
		hook(taskCount, duration)
	}
	if m.leaderPulseGenerations == nil {
		return
	}
	m.leaderPulseGenerations.Add(ctx, 1)
	m.leaderPulseDuration.Record(ctx, duration.Seconds())
	m.leaderPulseTaskCount.Record(ctx, int64(taskCount))
}

// RecordAttentionDecision records an attention gate dispatch decision.
func (m *MetricsCollector) RecordAttentionDecision(ctx context.Context, urgencyLevel string, suppressed bool) {
	if m == nil {
		return
	}
	if hook := m.testHooks.AttentionDecision; hook != nil {
		hook(urgencyLevel, suppressed)
	}
	if m.leaderAttentionDecision == nil {
		return
	}
	suppressedStr := "false"
	if suppressed {
		suppressedStr = "true"
	}
	m.leaderAttentionDecision.Add(ctx, 1, metric.WithAttributes(
		attribute.String("urgency", urgencyLevel),
		attribute.String("suppressed", suppressedStr),
	))
}

// RecordFocusTimeSuppress records a message suppressed due to focus time.
func (m *MetricsCollector) RecordFocusTimeSuppress(ctx context.Context, userID string) {
	if m == nil {
		return
	}
	if hook := m.testHooks.FocusTimeSuppress; hook != nil {
		hook(userID)
	}
	if m.leaderFocusSuppress == nil {
		return
	}
	m.leaderFocusSuppress.Add(ctx, 1, metric.WithAttributes(
		attribute.String("user_id", userID),
	))
}

// RecordAlertOutcome records a leader notification lifecycle outcome.
// feature is the leader service name (e.g. "blocker_radar", "weekly_pulse").
// This method satisfies notification.OutcomeRecorder when wrapped with
// MetricsOutcomeRecorder.
func (m *MetricsCollector) RecordAlertOutcome(ctx context.Context, feature, channel, outcome string) {
	if m == nil {
		return
	}
	if hook := m.testHooks.AlertOutcome; hook != nil {
		hook(feature, channel, outcome, 0)
	}
	if m.leaderAlertOutcomes == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("feature", feature),
		attribute.String("channel", channel),
		attribute.String("outcome", outcome),
	}
	m.leaderAlertOutcomes.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordAlertSendLatency records the send latency for a leader notification.
func (m *MetricsCollector) RecordAlertSendLatency(ctx context.Context, feature, channel string, latencyMs float64) {
	if m == nil {
		return
	}
	if hook := m.testHooks.AlertOutcome; hook != nil {
		hook(feature, channel, "", latencyMs)
	}
	if m.leaderAlertSendLatency == nil {
		return
	}
	m.leaderAlertSendLatency.Record(ctx, latencyMs, metric.WithAttributes(
		attribute.String("feature", feature),
		attribute.String("channel", channel),
	))
}

// MetricsOutcomeRecorder adapts MetricsCollector to notification.OutcomeRecorder.
type MetricsOutcomeRecorder struct {
	Metrics *MetricsCollector
}

// RecordAlertOutcome implements notification.OutcomeRecorder.
func (r *MetricsOutcomeRecorder) RecordAlertOutcome(ctx context.Context, feature, channel string, outcome notification.AlertOutcome) {
	if r.Metrics != nil {
		r.Metrics.RecordAlertOutcome(ctx, feature, channel, string(outcome))
	}
}

// EstimateCost returns the USD cost for a single LLM request.
// It consults the models.dev registry first and falls back to a $1.5/1M default.
func EstimateCost(model string, inputTokens, outputTokens int) float64 {
	if info, ok := modelregistry.Lookup(model); ok && info.InputPer1M > 0 {
		return float64(inputTokens)/1e6*info.InputPer1M +
			float64(outputTokens)/1e6*info.OutputPer1M
	}
	// Conservative fallback: $1.5 per 1M tokens (blended input+output).
	return float64(inputTokens+outputTokens) / 1e6 * 1.5
}
