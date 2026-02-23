package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	alexerrors "alex/internal/shared/errors"
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

	streaming, ok := client.(portsllm.StreamingLLMClient)
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

	streaming, ok := client.(portsllm.StreamingLLMClient)
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

type retryAfterCompleteMock struct {
	calls int
}

func (m *retryAfterCompleteMock) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	m.calls++
	if m.calls == 1 {
		return nil, &alexerrors.TransientError{
			Err:        errors.New("429 rate limit"),
			RetryAfter: 7,
			StatusCode: 429,
		}
	}
	return &ports.CompletionResponse{Content: "ok"}, nil
}

func (m *retryAfterCompleteMock) Model() string { return "mock" }

func TestRetryClientCompleteHonorsRetryAfter(t *testing.T) {
	mock := &retryAfterCompleteMock{}
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	client := NewRetryClient(mock, alexerrors.RetryConfig{
		MaxAttempts: 1,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    30 * time.Second,
	}, breaker)

	rc, ok := client.(*retryClient)
	require.True(t, ok)

	var waits []time.Duration
	rc.sleepFn = func(ctx context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	}

	resp, err := rc.Complete(context.Background(), ports.CompletionRequest{})
	require.NoError(t, err)
	require.Equal(t, "ok", resp.Content)
	require.Equal(t, 2, mock.calls)
	require.Equal(t, []time.Duration{7 * time.Second}, waits)
}

type retryAfterStreamingMock struct {
	calls int
}

func (m *retryAfterStreamingMock) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	return &ports.CompletionResponse{Content: "unused"}, nil
}

func (m *retryAfterStreamingMock) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	m.calls++
	if m.calls == 1 {
		return nil, &alexerrors.TransientError{
			Err:        errors.New("429 rate limit"),
			RetryAfter: 3,
			StatusCode: 429,
		}
	}
	if callbacks.OnContentDelta != nil {
		callbacks.OnContentDelta(ports.ContentDelta{Delta: "ok"})
		callbacks.OnContentDelta(ports.ContentDelta{Final: true})
	}
	return &ports.CompletionResponse{Content: "ok"}, nil
}

func (m *retryAfterStreamingMock) Model() string { return "mock" }

func TestRetryClientStreamCompleteHonorsRetryAfter(t *testing.T) {
	mock := &retryAfterStreamingMock{}
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	client := NewRetryClient(mock, alexerrors.RetryConfig{
		MaxAttempts: 1,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    30 * time.Second,
	}, breaker)

	rc, ok := client.(*retryClient)
	require.True(t, ok)

	var waits []time.Duration
	rc.sleepFn = func(ctx context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	}

	var streaming portsllm.StreamingLLMClient = rc

	resp, err := streaming.StreamComplete(context.Background(), ports.CompletionRequest{}, ports.CompletionStreamCallbacks{})
	require.NoError(t, err)
	require.Equal(t, "ok", resp.Content)
	require.Equal(t, 2, mock.calls)
	require.Equal(t, []time.Duration{3 * time.Second}, waits)
}

func TestRetryClientRetryAfterRespectsMaxDelay(t *testing.T) {
	rc := &retryClient{
		retryConfig: alexerrors.RetryConfig{
			BaseDelay: 10 * time.Millisecond,
			MaxDelay:  2 * time.Second,
		},
	}
	delay := rc.retryDelay(0, &alexerrors.TransientError{Err: errors.New("429"), RetryAfter: 9})
	require.Equal(t, 2*time.Second, delay)
}
