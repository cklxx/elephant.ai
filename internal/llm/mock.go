package llm

import (
	"context"

	"alex/internal/agent/ports"
)

// mockClient is a mock LLM client for testing
type mockClient struct{}

// NewMockClient creates a mock LLM client
func NewMockClient() ports.LLMClient {
	return &mockClient{}
}

func (c *mockClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	return &ports.CompletionResponse{
		Content:    "Mock LLM response",
		StopReason: "stop",
		Usage: ports.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 10,
			TotalTokens:      20,
		},
	}, nil
}

func (c *mockClient) Model() string {
	return "mock"
}

// ollamaClient placeholder
type ollamaClient struct {
	model string
}

func NewOllamaClient(model string, config Config) (ports.LLMClient, error) {
	return &ollamaClient{model: model}, nil
}

func (c *ollamaClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	return &ports.CompletionResponse{
		Content:    "Ollama response",
		StopReason: "stop",
		Usage:      ports.TokenUsage{TotalTokens: 100},
	}, nil
}

func (c *ollamaClient) Model() string {
	return c.model
}
