package mocks

import (
	"alex/internal/agent/ports"
	"context"
)

type MockLLMClient struct {
	CompleteFunc func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error)
	ModelFunc    func() string
}

func (m *MockLLMClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	if m.CompleteFunc != nil {
		return m.CompleteFunc(ctx, req)
	}
	return &ports.CompletionResponse{
		Content:    "Mock response",
		StopReason: "stop",
		Usage:      ports.TokenUsage{TotalTokens: 100},
	}, nil
}

func (m *MockLLMClient) Model() string {
	if m.ModelFunc != nil {
		return m.ModelFunc()
	}
	return "mock-model"
}
