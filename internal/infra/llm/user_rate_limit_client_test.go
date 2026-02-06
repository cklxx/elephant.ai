package llm

import (
	"context"
	"testing"

	"alex/internal/agent/ports"
	portsllm "alex/internal/agent/ports/llm"
	"github.com/stretchr/testify/require"
)

type nonStreamingRateLimitedMock struct {
	content       string
	completeCalls int
}

func (m *nonStreamingRateLimitedMock) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	m.completeCalls++
	return &ports.CompletionResponse{Content: m.content}, nil
}

func (m *nonStreamingRateLimitedMock) Model() string { return "mock-rate-limit" }

func TestWrapWithUserRateLimitProvidesStreamingAdapter(t *testing.T) {
	mock := &nonStreamingRateLimitedMock{content: "hello"}

	client := WrapWithUserRateLimit(mock, 1, 1)
	streaming, ok := client.(portsllm.StreamingLLMClient)
	require.True(t, ok, "wrapped client should expose streaming interface")

	var deltas []ports.ContentDelta
	resp, err := streaming.StreamComplete(context.Background(), ports.CompletionRequest{}, ports.CompletionStreamCallbacks{
		OnContentDelta: func(delta ports.ContentDelta) { deltas = append(deltas, delta) },
	})

	require.NoError(t, err)
	require.Equal(t, "hello", resp.Content)
	require.Equal(t, []ports.ContentDelta{{Delta: "hello"}, {Final: true}}, deltas)
	require.Equal(t, 1, mock.completeCalls)
}
