package observability

import (
	"context"
	"testing"
	"time"

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
