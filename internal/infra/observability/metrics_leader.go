package observability

import (
	"context"
	"fmt"
	"time"

	"alex/internal/shared/notification"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func (m *MetricsCollector) initLeaderMetrics() error {
	var err error
	if m.leaderBlockerScans, err = m.meter.Int64Counter("alex.leader.blocker.scans.total", metric.WithDescription("Total blocker radar scans"), metric.WithUnit("{scan}")); err != nil {
		return fmt.Errorf("failed to create leader_blocker_scans counter: %w", err)
	}
	if m.leaderBlockerDetected, err = m.meter.Int64Counter("alex.leader.blocker.detected.total", metric.WithDescription("Total blocked tasks detected"), metric.WithUnit("{task}")); err != nil {
		return fmt.Errorf("failed to create leader_blocker_detected counter: %w", err)
	}
	if m.leaderBlockerNotified, err = m.meter.Int64Counter("alex.leader.blocker.notified.total", metric.WithDescription("Total blocker notifications sent"), metric.WithUnit("{notification}")); err != nil {
		return fmt.Errorf("failed to create leader_blocker_notified counter: %w", err)
	}
	if m.leaderPulseGenerations, err = m.meter.Int64Counter("alex.leader.pulse.generations.total", metric.WithDescription("Total weekly pulse generations"), metric.WithUnit("{generation}")); err != nil {
		return fmt.Errorf("failed to create leader_pulse_generations counter: %w", err)
	}
	if m.leaderPulseDuration, err = m.meter.Float64Histogram("alex.leader.pulse.duration", metric.WithDescription("Pulse generation duration in seconds"), metric.WithUnit("s")); err != nil {
		return fmt.Errorf("failed to create leader_pulse_duration histogram: %w", err)
	}
	if m.leaderPulseTaskCount, err = m.meter.Int64Histogram("alex.leader.pulse.task_count", metric.WithDescription("Number of tasks included in pulse digest"), metric.WithUnit("{task}")); err != nil {
		return fmt.Errorf("failed to create leader_pulse_task_count histogram: %w", err)
	}
	if m.leaderAttentionDecision, err = m.meter.Int64Counter("alex.leader.attention.decisions.total", metric.WithDescription("Total attention gate dispatch decisions"), metric.WithUnit("{decision}")); err != nil {
		return fmt.Errorf("failed to create leader_attention_decision counter: %w", err)
	}
	if m.leaderFocusSuppress, err = m.meter.Int64Counter("alex.leader.focus.suppressions.total", metric.WithDescription("Total messages suppressed during focus time"), metric.WithUnit("{suppression}")); err != nil {
		return fmt.Errorf("failed to create leader_focus_suppress counter: %w", err)
	}
	if m.leaderAlertOutcomes, err = m.meter.Int64Counter("alex.leader.alert.outcomes.total", metric.WithDescription("Leader notification lifecycle outcomes (sent, delivered, failed, opened, dismissed, acted_on)"), metric.WithUnit("{outcome}")); err != nil {
		return fmt.Errorf("failed to create leader_alert_outcomes counter: %w", err)
	}
	if m.leaderAlertSendLatency, err = m.meter.Float64Histogram("alex.leader.alert.send.latency", metric.WithDescription("Leader alert send latency in milliseconds"), metric.WithUnit("ms")); err != nil {
		return fmt.Errorf("failed to create leader_alert_send_latency histogram: %w", err)
	}
	return nil
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

	m.leaderFocusSuppress.Add(ctx, 1, metric.WithAttributes(attribute.String("user_id", userID)))
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
