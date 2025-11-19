package llm

import (
	"context"
	"testing"

	"alex/internal/agent/ports"
	alexerrors "alex/internal/errors"
	"github.com/stretchr/testify/require"
)

type streamingMockClient struct {
	content       string
	completeCalls int
	streamCalls   int
}

func (m *streamingMockClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	m.completeCalls++
	return &ports.CompletionResponse{Content: m.content}, nil
}

func (m *streamingMockClient) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	m.streamCalls++
	if callbacks.OnContentDelta != nil {
		callbacks.OnContentDelta(ports.ContentDelta{Delta: m.content})
		callbacks.OnContentDelta(ports.ContentDelta{Final: true})
	}
	return &ports.CompletionResponse{Content: m.content}, nil
}

func (m *streamingMockClient) Model() string { return "mock" }

type nonStreamingMockClient struct {
	content       string
	completeCalls int
}

func (m *nonStreamingMockClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	m.completeCalls++
	return &ports.CompletionResponse{Content: m.content}, nil
}

func (m *nonStreamingMockClient) Model() string { return "mock" }

func TestRetryClientStreamCompleteUsesUnderlyingStreamingClient(t *testing.T) {
	mock := &streamingMockClient{content: "hello"}
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	client := NewRetryClient(mock, alexerrors.DefaultRetryConfig(), breaker)

	streaming, ok := client.(ports.StreamingLLMClient)
	require.True(t, ok)

	var deltas []ports.ContentDelta
	resp, err := streaming.StreamComplete(context.Background(), ports.CompletionRequest{}, ports.CompletionStreamCallbacks{
		OnContentDelta: func(delta ports.ContentDelta) {
			deltas = append(deltas, delta)
		},
	})

	require.NoError(t, err)
	require.Equal(t, "hello", resp.Content)
	require.Equal(t, []ports.ContentDelta{{Delta: "hello"}, {Final: true}}, deltas)
	require.Equal(t, 1, mock.streamCalls)
	require.Equal(t, 0, mock.completeCalls)
}

func TestRetryClientStreamCompleteFallsBackWhenUnavailable(t *testing.T) {
	mock := &nonStreamingMockClient{content: "fallback"}
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	client := NewRetryClient(mock, alexerrors.DefaultRetryConfig(), breaker)

	streaming, ok := client.(ports.StreamingLLMClient)
	require.True(t, ok)

	var deltas []ports.ContentDelta
	resp, err := streaming.StreamComplete(context.Background(), ports.CompletionRequest{}, ports.CompletionStreamCallbacks{
		OnContentDelta: func(delta ports.ContentDelta) {
			deltas = append(deltas, delta)
		},
	})

	require.NoError(t, err)
	require.Equal(t, "fallback", resp.Content)
	require.Equal(t, []ports.ContentDelta{{Delta: "fallback"}, {Final: true}}, deltas)
	require.Equal(t, 1, mock.completeCalls)
}
