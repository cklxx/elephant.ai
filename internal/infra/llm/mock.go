package llm

import (
	"context"
	"strings"

	"alex/internal/agent/ports"
	portsllm "alex/internal/agent/ports/llm"
)

// Ensure the mock client satisfies the streaming interfaces used by the agent.
var (
	_ portsllm.LLMClient          = (*mockClient)(nil)
	_ portsllm.StreamingLLMClient = (*mockClient)(nil)
)

// mockClient is a mock LLM client for testing
type mockClient struct{}

// NewMockClient creates a mock LLM client
func NewMockClient() portsllm.LLMClient {
	return &mockClient{}
}

func (c *mockClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	resp, _ := buildMockResponse(req)
	return resp, nil
}

func (c *mockClient) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	resp, chunks := buildMockResponse(req)

	if cb := callbacks.OnContentDelta; cb != nil {
		for idx, chunk := range chunks {
			cb(ports.ContentDelta{
				Delta: chunk,
				Final: idx == len(chunks)-1,
			})
		}

		if len(chunks) == 0 {
			cb(ports.ContentDelta{Final: true})
		}
	}

	return resp, nil
}

func (c *mockClient) Model() string {
	return "mock"
}

func buildMockResponse(req ports.CompletionRequest) (*ports.CompletionResponse, []string) {
	scenario := selectMockScenario(req)
	chunks := append([]string(nil), scenario.chunks...)
	if len(chunks) == 0 {
		chunks = append(chunks, "Mock LLM response")
	}

	var builder strings.Builder
	for _, chunk := range chunks {
		builder.WriteString(chunk)
	}

	resp := &ports.CompletionResponse{
		Content:    builder.String(),
		StopReason: "stop",
		Usage: ports.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 10,
			TotalTokens:      20,
		},
	}

	return resp, chunks
}
