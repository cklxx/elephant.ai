package mocks

import (
	"alex/internal/agent/ports"
	"context"
)

type MockLLMClient struct {
	CompleteFunc       func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error)
	ModelFunc          func() string
	StreamCompleteFunc func(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error)
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

func (m *MockLLMClient) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	if m.StreamCompleteFunc != nil {
		return m.StreamCompleteFunc(ctx, req, callbacks)
	}

	resp, err := m.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	if callbacks.OnContentDelta != nil {
		delta := resp.Content
		if delta != "" {
			callbacks.OnContentDelta(ports.ContentDelta{Delta: delta})
		}
		callbacks.OnContentDelta(ports.ContentDelta{Final: true})
	}

	return resp, nil
}
