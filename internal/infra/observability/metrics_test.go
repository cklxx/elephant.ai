package observability

import (
	"context"
	"testing"
	"time"

	"alex/internal/shared/notification"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetricsCollector(t *testing.T) {
	tests := []struct {
		name    string
		config  MetricsConfig
		wantErr bool
	}{
		{
			name: "disabled metrics",
			config: MetricsConfig{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "enabled metrics without server",
			config: MetricsConfig{
				Enabled:        true,
				PrometheusPort: 0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector, err := NewMetricsCollector(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, collector)

				// Cleanup
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = collector.Shutdown(ctx)
			}
		})
	}
}

func TestMetricsCollector_RecordLLMRequest(t *testing.T) {
	collector, err := NewMetricsCollector(MetricsConfig{
		Enabled:        true,
		PrometheusPort: 0,
	})
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = collector.Shutdown(ctx)
	}()

	ctx := context.Background()

	// Record successful request
	collector.RecordLLMRequest(ctx, "gpt-4", "success", 1*time.Second, 100, 50, 0.002)

	// Record failed request
	collector.RecordLLMRequest(ctx, "gpt-4", "error", 500*time.Millisecond, 0, 0, 0)

	// No assertions - just verify no panics
}

func TestMetricsCollector_RecordToolExecution(t *testing.T) {
	collector, err := NewMetricsCollector(MetricsConfig{
		Enabled:        true,
		PrometheusPort: 0,
	})
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = collector.Shutdown(ctx)
	}()

	ctx := context.Background()

	// Record successful execution
	collector.RecordToolExecution(ctx, "file_read", "success", 100*time.Millisecond)

	// Record failed execution
	collector.RecordToolExecution(ctx, "bash", "error", 50*time.Millisecond)

	// No assertions - just verify no panics
}

func TestMetricsCollector_SessionMetrics(t *testing.T) {
	collector, err := NewMetricsCollector(MetricsConfig{
		Enabled:        true,
		PrometheusPort: 0,
	})
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = collector.Shutdown(ctx)
	}()

	ctx := context.Background()

	// Test increment/decrement
	collector.IncrementActiveSessions(ctx)
	collector.IncrementActiveSessions(ctx)
	collector.DecrementActiveSessions(ctx)

	// No assertions - just verify no panics
}

func TestEstimateCost(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		inputTokens  int
		outputTokens int
		minCost      float64
		maxCost      float64
	}{
		{
			name:         "gpt-4",
			model:        "gpt-4",
			inputTokens:  1000,
			outputTokens: 500,
			minCost:      0.00001,
			maxCost:      1.0,
		},
		{
			name:         "gpt-3.5-turbo",
			model:        "gpt-3.5-turbo",
			inputTokens:  10000,
			outputTokens: 5000,
			minCost:      0.00001,
			maxCost:      1.0,
		},
		{
			name:         "claude-3-opus",
			model:        "claude-3-opus",
			inputTokens:  5000,
			outputTokens: 2000,
			minCost:      0.00001,
			maxCost:      1.0,
		},
		{
			name:         "unknown model",
			model:        "unknown-model",
			inputTokens:  1000,
			outputTokens: 500,
			minCost:      0.00001,
			maxCost:      1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := EstimateCost(tt.model, tt.inputTokens, tt.outputTokens)
			assert.Greater(t, cost, tt.minCost)
			assert.Less(t, cost, tt.maxCost)
		})
	}
}

// --- Leader agent metrics ---

func TestMetricsCollector_RecordBlockerScan(t *testing.T) {
	collector, err := NewMetricsCollector(MetricsConfig{Enabled: true, PrometheusPort: 0})
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = collector.Shutdown(ctx)
	}()

	// No panic with real OTel metrics.
	collector.RecordBlockerScan(context.Background(), 3, 2)
}

func TestMetricsCollector_RecordBlockerScan_TestHook(t *testing.T) {
	collector := &MetricsCollector{}
	var gotDetected, gotNotified int
	collector.SetTestHooks(MetricsTestHooks{
		BlockerScan: func(detected, notified int) {
			gotDetected = detected
			gotNotified = notified
		},
	})
	collector.RecordBlockerScan(context.Background(), 5, 3)
	assert.Equal(t, 5, gotDetected)
	assert.Equal(t, 3, gotNotified)
}

func TestMetricsCollector_RecordPulseGeneration(t *testing.T) {
	collector, err := NewMetricsCollector(MetricsConfig{Enabled: true, PrometheusPort: 0})
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = collector.Shutdown(ctx)
	}()

	collector.RecordPulseGeneration(context.Background(), 7, 2*time.Second)
}

func TestMetricsCollector_RecordPulseGeneration_TestHook(t *testing.T) {
	collector := &MetricsCollector{}
	var gotCount int
	var gotDuration time.Duration
	collector.SetTestHooks(MetricsTestHooks{
		PulseGeneration: func(taskCount int, duration time.Duration) {
			gotCount = taskCount
			gotDuration = duration
		},
	})
	collector.RecordPulseGeneration(context.Background(), 10, 500*time.Millisecond)
	assert.Equal(t, 10, gotCount)
	assert.Equal(t, 500*time.Millisecond, gotDuration)
}

func TestMetricsCollector_RecordAttentionDecision(t *testing.T) {
	collector, err := NewMetricsCollector(MetricsConfig{Enabled: true, PrometheusPort: 0})
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = collector.Shutdown(ctx)
	}()

	collector.RecordAttentionDecision(context.Background(), "high", false)
	collector.RecordAttentionDecision(context.Background(), "low", true)
}

func TestMetricsCollector_RecordAttentionDecision_TestHook(t *testing.T) {
	collector := &MetricsCollector{}
	var gotLevel string
	var gotSuppressed bool
	collector.SetTestHooks(MetricsTestHooks{
		AttentionDecision: func(urgencyLevel string, suppressed bool) {
			gotLevel = urgencyLevel
			gotSuppressed = suppressed
		},
	})
	collector.RecordAttentionDecision(context.Background(), "high", true)
	assert.Equal(t, "high", gotLevel)
	assert.True(t, gotSuppressed)
}

func TestMetricsCollector_RecordFocusTimeSuppress(t *testing.T) {
	collector, err := NewMetricsCollector(MetricsConfig{Enabled: true, PrometheusPort: 0})
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = collector.Shutdown(ctx)
	}()

	collector.RecordFocusTimeSuppress(context.Background(), "alice")
}

func TestMetricsCollector_RecordFocusTimeSuppress_TestHook(t *testing.T) {
	collector := &MetricsCollector{}
	var gotUserID string
	collector.SetTestHooks(MetricsTestHooks{
		FocusTimeSuppress: func(userID string) {
			gotUserID = userID
		},
	})
	collector.RecordFocusTimeSuppress(context.Background(), "bob")
	assert.Equal(t, "bob", gotUserID)
}

func TestMetricsCollector_LeaderMetrics_NilSafe(t *testing.T) {
	var collector *MetricsCollector
	ctx := context.Background()
	// All should be no-ops on nil receiver.
	collector.RecordBlockerScan(ctx, 1, 1)
	collector.RecordPulseGeneration(ctx, 1, time.Second)
	collector.RecordAttentionDecision(ctx, "low", false)
	collector.RecordFocusTimeSuppress(ctx, "user")
	collector.RecordAlertOutcome(ctx, "blocker_radar", "lark", "sent")
	collector.RecordAlertSendLatency(ctx, "blocker_radar", "lark", 42.0)
}

func TestMetricsCollector_LeaderMetrics_DisabledNoPanic(t *testing.T) {
	collector, err := NewMetricsCollector(MetricsConfig{Enabled: false})
	require.NoError(t, err)
	ctx := context.Background()
	collector.RecordBlockerScan(ctx, 1, 1)
	collector.RecordPulseGeneration(ctx, 1, time.Second)
	collector.RecordAttentionDecision(ctx, "low", false)
	collector.RecordFocusTimeSuppress(ctx, "user")
	collector.RecordAlertOutcome(ctx, "blocker_radar", "lark", "sent")
	collector.RecordAlertSendLatency(ctx, "weekly_pulse", "lark", 10.0)
}

// --- Alert outcome metrics ---

func TestMetricsCollector_RecordAlertOutcome(t *testing.T) {
	collector, err := NewMetricsCollector(MetricsConfig{Enabled: true, PrometheusPort: 0})
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = collector.Shutdown(ctx)
	}()

	ctx := context.Background()
	collector.RecordAlertOutcome(ctx, "blocker_radar", "lark", "sent")
	collector.RecordAlertOutcome(ctx, "blocker_radar", "lark", "delivered")
	collector.RecordAlertOutcome(ctx, "weekly_pulse", "lark", "failed")
	collector.RecordAlertOutcome(ctx, "prep_brief", "lark", "opened")
	collector.RecordAlertOutcome(ctx, "milestone_checkin", "lark", "dismissed")
	collector.RecordAlertOutcome(ctx, "blocker_radar", "lark", "acted_on")
}

func TestMetricsCollector_RecordAlertOutcome_TestHook(t *testing.T) {
	collector := &MetricsCollector{}
	var gotFeature, gotChannel, gotOutcome string
	collector.SetTestHooks(MetricsTestHooks{
		AlertOutcome: func(feature, channel, outcome string, latencyMs float64) {
			gotFeature = feature
			gotChannel = channel
			gotOutcome = outcome
		},
	})
	collector.RecordAlertOutcome(context.Background(), "blocker_radar", "lark", "delivered")
	assert.Equal(t, "blocker_radar", gotFeature)
	assert.Equal(t, "lark", gotChannel)
	assert.Equal(t, "delivered", gotOutcome)
}

func TestMetricsCollector_RecordAlertSendLatency(t *testing.T) {
	collector, err := NewMetricsCollector(MetricsConfig{Enabled: true, PrometheusPort: 0})
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = collector.Shutdown(ctx)
	}()

	collector.RecordAlertSendLatency(context.Background(), "blocker_radar", "lark", 42.5)
}

func TestMetricsCollector_RecordAlertSendLatency_TestHook(t *testing.T) {
	collector := &MetricsCollector{}
	var gotLatency float64
	collector.SetTestHooks(MetricsTestHooks{
		AlertOutcome: func(feature, channel, outcome string, latencyMs float64) {
			gotLatency = latencyMs
		},
	})
	collector.RecordAlertSendLatency(context.Background(), "weekly_pulse", "lark", 99.5)
	assert.InDelta(t, 99.5, gotLatency, 0.01)
}

func TestMetricsOutcomeRecorder(t *testing.T) {
	collector := &MetricsCollector{}
	var gotFeature, gotChannel, gotOutcome string
	collector.SetTestHooks(MetricsTestHooks{
		AlertOutcome: func(feature, channel, outcome string, latencyMs float64) {
			gotFeature = feature
			gotChannel = channel
			gotOutcome = outcome
		},
	})

	recorder := &MetricsOutcomeRecorder{Metrics: collector}
	recorder.RecordAlertOutcome(context.Background(), "prep_brief", "lark", notification.OutcomeOpened)
	assert.Equal(t, "prep_brief", gotFeature)
	assert.Equal(t, "lark", gotChannel)
	assert.Equal(t, "opened", gotOutcome)
}

func TestMetricsOutcomeRecorder_NilMetrics(t *testing.T) {
	recorder := &MetricsOutcomeRecorder{Metrics: nil}
	// Should not panic.
	recorder.RecordAlertOutcome(context.Background(), "blocker_radar", "lark", notification.OutcomeSent)
}

func TestMetricsCollector_DisabledMetrics(t *testing.T) {
	collector, err := NewMetricsCollector(MetricsConfig{
		Enabled: false,
	})
	require.NoError(t, err)

	ctx := context.Background()

	// These should not panic even when disabled
	collector.RecordLLMRequest(ctx, "gpt-4", "success", 1*time.Second, 100, 50, 0.002)
	collector.RecordToolExecution(ctx, "file_read", "success", 100*time.Millisecond)
	collector.IncrementActiveSessions(ctx)
	collector.DecrementActiveSessions(ctx)
}
