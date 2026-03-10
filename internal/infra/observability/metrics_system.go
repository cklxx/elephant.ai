package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func (m *MetricsCollector) initSystemMetrics() error {
	var err error
	if m.sessionsActive, err = m.meter.Int64UpDownCounter("alex.sessions.active", metric.WithDescription("Number of active sessions"), metric.WithUnit("{session}")); err != nil {
		return fmt.Errorf("failed to create sessions_active gauge: %w", err)
	}
	if m.taskExecutions, err = m.meter.Int64Counter("alex.tasks.executions.total", metric.WithDescription("Total background task executions"), metric.WithUnit("{execution}")); err != nil {
		return fmt.Errorf("failed to create task_executions counter: %w", err)
	}
	if m.taskDuration, err = m.meter.Float64Histogram("alex.tasks.execution.duration", metric.WithDescription("Task execution duration in seconds"), metric.WithUnit("s")); err != nil {
		return fmt.Errorf("failed to create task_duration histogram: %w", err)
	}
	if m.webVital, err = m.meter.Float64Histogram("alex.frontend.web_vital", metric.WithDescription("Reported frontend web vital values")); err != nil {
		return fmt.Errorf("failed to create web_vital histogram: %w", err)
	}
	if m.nsmWTCR, err = m.meter.Float64Histogram("alex.nsm.wtcr", metric.WithDescription("Willing-To-Come-back Rate per session (0-1)"), metric.WithUnit("1")); err != nil {
		return fmt.Errorf("failed to create nsm_wtcr histogram: %w", err)
	}
	if m.nsmTimeSaved, err = m.meter.Float64Histogram("alex.nsm.time_saved", metric.WithDescription("Estimated time saved per interaction in seconds"), metric.WithUnit("s")); err != nil {
		return fmt.Errorf("failed to create nsm_time_saved histogram: %w", err)
	}
	if m.nsmAccuracy, err = m.meter.Float64Histogram("alex.nsm.accuracy", metric.WithDescription("Correctness score per action (0-1)"), metric.WithUnit("1")); err != nil {
		return fmt.Errorf("failed to create nsm_accuracy histogram: %w", err)
	}
	return nil
}

// IncrementActiveSessions increments the active sessions counter.
func (m *MetricsCollector) IncrementActiveSessions(ctx context.Context) {
	if m.sessionsActive == nil {
		return
	}
	m.sessionsActive.Add(ctx, 1)
}

// DecrementActiveSessions decrements the active sessions counter.
func (m *MetricsCollector) DecrementActiveSessions(ctx context.Context) {
	if m.sessionsActive == nil {
		return
	}
	m.sessionsActive.Add(ctx, -1)
}

// RecordTaskExecution records task execution metrics.
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

	attrs := []attribute.KeyValue{attribute.String("status", status)}
	m.taskExecutions.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.taskDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordWebVital stores reported frontend performance metrics.
func (m *MetricsCollector) RecordWebVital(ctx context.Context, name, label, page string, value, delta float64) {
	if m.webVital == nil {
		return
	}

	attrs := []attribute.KeyValue{attribute.String("name", name)}
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
	m.nsmWTCR.Record(ctx, value, metric.WithAttributes(attribute.String("session_id", sessionID)))
}

// RecordNSMTimeSaved records estimated time saved (seconds) for an interaction.
func (m *MetricsCollector) RecordNSMTimeSaved(ctx context.Context, taskType string, seconds float64) {
	if m == nil || m.nsmTimeSaved == nil {
		return
	}
	m.nsmTimeSaved.Record(ctx, seconds, metric.WithAttributes(attribute.String("task_type", taskType)))
}

// RecordNSMAccuracy records a correctness score for a completed action.
// value should be in [0, 1].
func (m *MetricsCollector) RecordNSMAccuracy(ctx context.Context, toolName string, value float64) {
	if m == nil || m.nsmAccuracy == nil {
		return
	}
	m.nsmAccuracy.Record(ctx, value, metric.WithAttributes(attribute.String("tool_name", toolName)))
}
