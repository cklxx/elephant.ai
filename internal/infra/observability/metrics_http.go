package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func (m *MetricsCollector) initHTTPMetrics() error {
	var err error
	if m.httpRequests, err = m.meter.Int64Counter("alex.http.requests.total", metric.WithDescription("Total HTTP requests handled by the server"), metric.WithUnit("{request}")); err != nil {
		return fmt.Errorf("failed to create http_requests counter: %w", err)
	}
	if m.httpLatency, err = m.meter.Float64Histogram("alex.http.latency", metric.WithDescription("HTTP request latency in seconds"), metric.WithUnit("s")); err != nil {
		return fmt.Errorf("failed to create http_latency histogram: %w", err)
	}
	if m.httpResponseSize, err = m.meter.Int64Histogram("alex.http.response.size", metric.WithDescription("HTTP response payload sizes in bytes"), metric.WithUnit("By")); err != nil {
		return fmt.Errorf("failed to create http_response_size histogram: %w", err)
	}
	if m.sseConnections, err = m.meter.Int64UpDownCounter("alex.sse.connections.active", metric.WithDescription("Active SSE connections"), metric.WithUnit("{connection}")); err != nil {
		return fmt.Errorf("failed to create sse_connections gauge: %w", err)
	}
	if m.sseConnectionDuration, err = m.meter.Float64Histogram("alex.sse.connection.duration", metric.WithDescription("SSE connection lifetimes in seconds"), metric.WithUnit("s")); err != nil {
		return fmt.Errorf("failed to create sse_connection_duration histogram: %w", err)
	}
	if m.sseMessages, err = m.meter.Int64Counter("alex.sse.messages.total", metric.WithDescription("Total SSE events delivered")); err != nil {
		return fmt.Errorf("failed to create sse_messages counter: %w", err)
	}
	if m.sseMessageBytes, err = m.meter.Int64Histogram("alex.sse.message.size", metric.WithDescription("SSE payload sizes in bytes"), metric.WithUnit("By")); err != nil {
		return fmt.Errorf("failed to create sse_message_size histogram: %w", err)
	}
	return nil
}

// RecordHTTPServerRequest records metrics for an HTTP request lifecycle.
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

// IncrementSSEConnections increments the active SSE connection gauge.
func (m *MetricsCollector) IncrementSSEConnections(ctx context.Context) {
	if m.sseConnections == nil {
		return
	}
	m.sseConnections.Add(ctx, 1)
}

// DecrementSSEConnections decrements the active SSE connection gauge.
func (m *MetricsCollector) DecrementSSEConnections(ctx context.Context) {
	if m.sseConnections == nil {
		return
	}
	m.sseConnections.Add(ctx, -1)
}

// RecordSSEConnectionDuration records how long an SSE connection stayed open.
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
