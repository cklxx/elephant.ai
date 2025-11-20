package app

import (
	"context"
	"strings"

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

var (
	_ ports.LLMClient          = (*costTrackingWrapper)(nil)
	_ ports.StreamingLLMClient = (*costTrackingWrapper)(nil)
)

func (w *costTrackingWrapper) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	// Call underlying client
	resp, err := w.client.Complete(ctx, req)
	if err != nil {
		return resp, err
	}

	w.recordUsage(resp)
	return resp, nil
}

func (w *costTrackingWrapper) Model() string {
	return w.client.Model()
}

func (w *costTrackingWrapper) StreamComplete(
	ctx context.Context,
	req ports.CompletionRequest,
	callbacks ports.CompletionStreamCallbacks,
) (*ports.CompletionResponse, error) {
	if streaming, ok := w.client.(ports.StreamingLLMClient); ok {
		resp, err := streaming.StreamComplete(ctx, req, callbacks)
		if err == nil {
			w.recordUsage(resp)
		}
		return resp, err
	}

	resp, err := w.client.Complete(ctx, req)
	if err != nil {
		return resp, err
	}

	if cb := callbacks.OnContentDelta; cb != nil {
		if resp != nil && resp.Content != "" {
			cb(ports.ContentDelta{Delta: resp.Content})
		}
		cb(ports.ContentDelta{Final: true})
	}

	w.recordUsage(resp)
	return resp, nil
}

func (w *costTrackingWrapper) recordUsage(resp *ports.CompletionResponse) {
	if w.tracker == nil || resp == nil {
		return
	}

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

// inferProvider attempts to infer the provider from the model name
func inferProvider(model string) string {
	// Normalize to lowercase for case-insensitive matching
	lower := strings.ToLower(model)

	// Check for provider prefixes (e.g., "openrouter/anthropic/claude", "anthropic/claude")
	if strings.Contains(lower, "anthropic/") || strings.HasPrefix(lower, "claude") {
		return "anthropic"
	}
	if strings.Contains(lower, "openai/") || strings.HasPrefix(lower, "gpt") {
		return "openai"
	}
	if strings.Contains(lower, "deepseek/") || strings.HasPrefix(lower, "deepseek") {
		return "deepseek"
	}

	// Check for model name patterns
	if strings.HasPrefix(lower, "gpt-") {
		return "openai"
	}
	if strings.HasPrefix(lower, "claude-") {
		return "anthropic"
	}

	// Default to openrouter if we can't determine
	// (most unknown models are likely accessed via openrouter)
	return "openrouter"
}
