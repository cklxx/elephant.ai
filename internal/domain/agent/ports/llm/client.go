package llm

import (
	"context"

	core "alex/internal/agent/ports"
)

// LLMClient represents any LLM provider.
type LLMClient interface {
	// Complete sends messages and returns a response (non-streaming).
	Complete(ctx context.Context, req core.CompletionRequest) (*core.CompletionResponse, error)

	// Model returns the model identifier.
	Model() string
}

// StreamingLLMClient extends LLMClient with native streaming support.
// Implementations can surface incremental content while still returning the
// aggregated completion response when the stream ends.
type StreamingLLMClient interface {
	LLMClient

	// StreamComplete behaves like Complete but delivers incremental deltas
	// via the provided callbacks before returning the final response.
	StreamComplete(ctx context.Context, req core.CompletionRequest, callbacks core.CompletionStreamCallbacks) (*core.CompletionResponse, error)
}

// LLMClientFactory creates LLM clients for different providers.
// This interface abstracts the concrete llm.Factory implementation.
type LLMClientFactory interface {
	// GetClient creates or retrieves a cached LLM client.
	GetClient(provider, model string, config LLMConfig) (LLMClient, error)

	// GetIsolatedClient creates a new non-cached client for session isolation.
	GetIsolatedClient(provider, model string, config LLMConfig) (LLMClient, error)

	// DisableRetry disables retry logic for all clients created by this factory.
	DisableRetry()
}

// LLMConfig contains configuration for LLM client creation.
type LLMConfig struct {
	APIKey     string
	BaseURL    string
	Timeout    int
	MaxRetries int
	Headers    map[string]string
}

// UsageTrackingClient extends LLMClient with usage tracking.
type UsageTrackingClient interface {
	LLMClient
	// SetUsageCallback sets a callback to be invoked after each API call.
	SetUsageCallback(callback func(usage core.TokenUsage, model string, provider string))
}
