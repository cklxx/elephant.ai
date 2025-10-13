package app

import (
	"context"

	"alex/internal/agent/ports"
)

// CostTrackingDecorator creates isolated wrappers for LLM clients to track costs per session
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

// Wrap returns a new wrapper client that tracks costs without modifying the original client
// This ensures session-level cost isolation even when the underlying client is shared
func (d *CostTrackingDecorator) Wrap(ctx context.Context, sessionID string, client ports.LLMClient) ports.LLMClient {
	if d.tracker == nil {
		return client
	}

	// Return a wrapper that intercepts Complete() calls
	return &costTrackingWrapper{
		client:    client,
		sessionID: sessionID,
		tracker:   d.tracker,
		logger:    d.logger,
		clock:     d.clock,
		ctx:       ctx,
	}
}

// Attach is deprecated - use Wrap instead for proper session isolation
// This method is kept for backward compatibility but will modify shared client state
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

// costTrackingWrapper wraps an LLMClient and tracks usage per session
type costTrackingWrapper struct {
	client    ports.LLMClient
	sessionID string
	tracker   ports.CostTracker
	logger    ports.Logger
	clock     ports.Clock
	ctx       context.Context
}

func (w *costTrackingWrapper) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	// Call underlying client
	resp, err := w.client.Complete(ctx, req)
	if err != nil {
		return resp, err
	}

	// Track usage
	if resp != nil {
		record := ports.UsageRecord{
			SessionID:    w.sessionID,
			Model:        w.client.Model(),
			Provider:     inferProvider(w.client.Model()),
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
			Timestamp:    w.clock.Now(),
		}

		record.InputCost, record.OutputCost, record.TotalCost = ports.CalculateCost(
			resp.Usage.PromptTokens,
			resp.Usage.CompletionTokens,
			w.client.Model(),
		)

		if err := w.tracker.RecordUsage(w.ctx, record); err != nil {
			w.logger.Warn("Failed to record cost for session %s: %v", w.sessionID, err)
		}
	}

	return resp, nil
}

func (w *costTrackingWrapper) Model() string {
	return w.client.Model()
}

// inferProvider attempts to infer the provider from the model name
func inferProvider(model string) string {
	// Simple heuristic - in production, this should be more robust
	if len(model) >= 3 {
		switch model[:3] {
		case "gpt":
			return "openai"
		case "cla":
			return "anthropic"
		case "dee":
			return "deepseek"
		}
	}
	return "unknown"
}
