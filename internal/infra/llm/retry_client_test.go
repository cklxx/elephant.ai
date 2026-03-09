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

type streamingTransportRetryMock struct {
	calls int
}

func (m *streamingTransportRetryMock) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	return &ports.CompletionResponse{Content: "ok"}, nil
}

func (m *streamingTransportRetryMock) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	m.calls++
	if m.calls == 1 {
		return nil, errors.New("read stream: stream error: stream ID 13; INTERNAL_ERROR; received from peer")
	}
	if callbacks.OnContentDelta != nil {
		callbacks.OnContentDelta(ports.ContentDelta{Delta: "ok"})
		callbacks.OnContentDelta(ports.ContentDelta{Final: true})
	}
	return &ports.CompletionResponse{Content: "ok"}, nil
}

func (m *streamingTransportRetryMock) Model() string { return "mock" }

type streamingTransportOutputThenFailMock struct {
	calls int
}

func (m *streamingTransportOutputThenFailMock) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	return &ports.CompletionResponse{Content: "ok"}, nil
}

func (m *streamingTransportOutputThenFailMock) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	m.calls++
	if callbacks.OnContentDelta != nil {
		callbacks.OnContentDelta(ports.ContentDelta{Delta: "partial"})
	}
	return nil, errors.New("read stream: stream error: stream ID 13; INTERNAL_ERROR; received from peer")
}

func (m *streamingTransportOutputThenFailMock) Model() string { return "mock" }

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

func TestRetryClientStreamCompleteRetriesTransportStreamError(t *testing.T) {
	mock := &streamingTransportRetryMock{}
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	client := NewRetryClient(mock, alexerrors.RetryConfig{
		MaxAttempts: 1,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    30 * time.Second,
	}, breaker)

	streaming, ok := client.(portsllm.StreamingLLMClient)
	require.True(t, ok)

	resp, err := streaming.StreamComplete(context.Background(), ports.CompletionRequest{}, ports.CompletionStreamCallbacks{})
	require.NoError(t, err)
	require.Equal(t, "ok", resp.Content)
	require.Equal(t, 2, mock.calls)
}

func TestRetryClientStreamCompleteStopsRetryAfterStreamOutput(t *testing.T) {
	mock := &streamingTransportOutputThenFailMock{}
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

	require.Error(t, err)
	require.Nil(t, resp)
	require.Equal(t, 1, mock.calls)
	require.Equal(t, []ports.ContentDelta{{Delta: "partial"}}, deltas)
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

// authRefreshMock returns 401 on the first call and succeeds on the second,
// and implements apiKeyUpdatable to verify the API key was swapped.
type authRefreshMock struct {
	calls  int
	apiKey string
}

func (m *authRefreshMock) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	m.calls++
	if m.calls == 1 {
		return nil, errors.New("HTTP 401: unauthorized")
	}
	return &ports.CompletionResponse{Content: "ok"}, nil
}

func (m *authRefreshMock) Model() string { return "mock" }

func (m *authRefreshMock) SetAPIKey(key string) {
	m.apiKey = key
}

func TestRetryClientAuthRefreshOn401(t *testing.T) {
	mock := &authRefreshMock{}
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	client := NewRetryClient(mock, alexerrors.RetryConfig{
		MaxAttempts: 2,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    50 * time.Millisecond,
	}, breaker)

	rc, ok := client.(*retryClient)
	require.True(t, ok)

	// Wire a mock auth refresher that returns a new token.
	rc.authRefresher = func() (string, error) {
		return "sk-ant-oat01-fresh-token", nil
	}
	rc.sleepFn = func(ctx context.Context, d time.Duration) error { return nil }

	resp, err := rc.Complete(context.Background(), ports.CompletionRequest{})
	require.NoError(t, err)
	require.Equal(t, "ok", resp.Content)
	require.Equal(t, 2, mock.calls, "should have retried after auth refresh")
	require.Equal(t, "sk-ant-oat01-fresh-token", mock.apiKey, "API key should be updated")
}

func TestRetryClientAuthRefreshNotTriggeredWithoutRefresher(t *testing.T) {
	mock := &authRefreshMock{}
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	client := NewRetryClient(mock, alexerrors.RetryConfig{
		MaxAttempts: 1,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    50 * time.Millisecond,
	}, breaker)

	rc, ok := client.(*retryClient)
	require.True(t, ok)
	rc.sleepFn = func(ctx context.Context, d time.Duration) error { return nil }
	// No authRefresher set.

	_, err := rc.Complete(context.Background(), ports.CompletionRequest{})
	require.Error(t, err)
	require.Equal(t, 1, mock.calls, "should not retry when no refresher")
}

func TestRetryClientCalculateBackoffClampsExponentialDelay(t *testing.T) {
	rc := &retryClient{
		retryConfig: alexerrors.RetryConfig{
			BaseDelay: 100 * time.Millisecond,
			MaxDelay:  250 * time.Millisecond,
		},
	}

	delay0 := rc.calculateBackoff(0)
	require.GreaterOrEqual(t, delay0, 75*time.Millisecond)
	require.LessOrEqual(t, delay0, 125*time.Millisecond)

	delay1 := rc.calculateBackoff(1)
	require.GreaterOrEqual(t, delay1, 150*time.Millisecond)
	require.LessOrEqual(t, delay1, 250*time.Millisecond)

	// Attempts 2+ are clamped to MaxDelay before jitter is applied.
	delay2 := rc.calculateBackoff(2)
	require.GreaterOrEqual(t, delay2, 187500*time.Microsecond)
	require.LessOrEqual(t, delay2, 312500*time.Microsecond)

	delay3 := rc.calculateBackoff(3)
	require.GreaterOrEqual(t, delay3, 187500*time.Microsecond)
	require.LessOrEqual(t, delay3, 312500*time.Microsecond)
}
