package llm

import (
	"context"

	"alex/internal/agent/ports"
)

// httpClient implements ports.LLMClient for HTTP-based providers
type httpClient struct {
	model  string
	config Config
	// TODO: Add actual HTTP client implementation
}

// NewHTTPClient creates an HTTP-based LLM client
func NewHTTPClient(model string, config Config) (ports.LLMClient, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://openrouter.ai/api/v1"
	}

	return &httpClient{
		model:  model,
		config: config,
	}, nil
}

func (c *httpClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	// TODO: Implement actual HTTP call to LLM provider
	// For now, return mock response
	return &ports.CompletionResponse{
		Content:    "Mock response from " + c.model,
		StopReason: "stop",
		Usage: ports.TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}, nil
}

func (c *httpClient) Model() string {
	return c.model
}
