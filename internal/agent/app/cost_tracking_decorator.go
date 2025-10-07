package app

import (
	"context"

	"alex/internal/agent/ports"
)

// CostTrackingDecorator attaches usage callbacks to LLM clients when supported.
type CostTrackingDecorator struct {
	tracker ports.CostTracker
	logger  ports.Logger
	clock   ports.Clock
}

// NewCostTrackingDecorator creates a new decorator instance.
func NewCostTrackingDecorator(tracker ports.CostTracker, logger ports.Logger, clock ports.Clock) *CostTrackingDecorator {
	if logger == nil {
		logger = ports.NoopLogger{}
	}
	if clock == nil {
		clock = ports.SystemClock{}
	}
	return &CostTrackingDecorator{tracker: tracker, logger: logger, clock: clock}
}

// Attach registers a usage callback on the provided client when tracking is enabled.
func (d *CostTrackingDecorator) Attach(ctx context.Context, sessionID string, client ports.LLMClient) ports.LLMClient {
	if d.tracker == nil {
		return client
	}

	trackingClient, ok := client.(ports.UsageTrackingClient)
	if !ok {
		return client
	}

	trackingClient.SetUsageCallback(func(usage ports.TokenUsage, model string, provider string) {
		record := ports.UsageRecord{
			SessionID:    sessionID,
			Model:        model,
			Provider:     provider,
			InputTokens:  usage.PromptTokens,
			OutputTokens: usage.CompletionTokens,
			TotalTokens:  usage.TotalTokens,
			Timestamp:    d.clock.Now(),
		}

		record.InputCost, record.OutputCost, record.TotalCost = ports.CalculateCost(
			usage.PromptTokens,
			usage.CompletionTokens,
			model,
		)

		if err := d.tracker.RecordUsage(ctx, record); err != nil {
			d.logger.Warn("Failed to record cost: %v", err)
		}
	})

	d.logger.Debug("Cost tracking enabled for session: %s", sessionID)
	return client
}
