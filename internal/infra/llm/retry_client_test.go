package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	alexerrors "alex/internal/shared/errors"
	"alex/internal/shared/logging"
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

// overloadedCompleteMock always returns a 529 overloaded transient error.
type overloadedCompleteMock struct {
	calls int
}

func (m *overloadedCompleteMock) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	m.calls++
	return nil, &alexerrors.TransientError{
		Err:        errors.New("HTTP 529: overloaded"),
		StatusCode: 529,
		Message:    "Server overloaded (529). Retrying request.",
	}
}

func (m *overloadedCompleteMock) Model() string { return "claude-sonnet-4-6" }

// fallbackCompleteMock always succeeds.
type fallbackCompleteMock struct {
	calls int
}

func (m *fallbackCompleteMock) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	m.calls++
	return &ports.CompletionResponse{Content: "fallback-ok"}, nil
}

func (m *fallbackCompleteMock) Model() string { return "kimi-for-coding" }

func TestRetryClientFallbackOnTransientExhaustion(t *testing.T) {
	primary := &overloadedCompleteMock{}
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	client := NewRetryClient(primary, alexerrors.RetryConfig{
		MaxAttempts: 1,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    50 * time.Millisecond,
	}, breaker)

	rc, ok := client.(*retryClient)
	require.True(t, ok)

	rc.provider = "anthropic"
	rc.model = "claude-sonnet-4-6"
	rc.sleepFn = func(ctx context.Context, d time.Duration) error { return nil }

	fallback := &fallbackCompleteMock{}
	rc.fallbackProvider = "kimi"
	rc.fallbackModel = "kimi-for-coding"
	rc.fallbackClientFn = func() (portsllm.LLMClient, error) {
		return fallback, nil
	}

	resp, err := rc.Complete(context.Background(), ports.CompletionRequest{})
	require.NoError(t, err)
	require.Equal(t, "fallback-ok", resp.Content)
	require.Equal(t, 2, primary.calls, "primary should be called 2 times (1 + 1 retry)")
	require.Equal(t, 1, fallback.calls, "fallback should be called once")
}

func TestRetryClientNoFallbackOnPermanentError(t *testing.T) {
	mock := &authRefreshMock{} // returns 401 permanent on first call
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	client := NewRetryClient(mock, alexerrors.RetryConfig{
		MaxAttempts: 0,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    50 * time.Millisecond,
	}, breaker)

	rc, ok := client.(*retryClient)
	require.True(t, ok)
	rc.sleepFn = func(ctx context.Context, d time.Duration) error { return nil }

	fallbackCalled := false
	rc.fallbackProvider = "kimi"
	rc.fallbackModel = "kimi-for-coding"
	rc.fallbackClientFn = func() (portsllm.LLMClient, error) {
		fallbackCalled = true
		return &fallbackCompleteMock{}, nil
	}

	_, err := rc.Complete(context.Background(), ports.CompletionRequest{})
	require.Error(t, err)
	require.False(t, fallbackCalled, "fallback should NOT be called for permanent errors")
}

func TestRetryClientFallbackStreamingOnTransientExhaustion(t *testing.T) {
	primary := &overloadedStreamMock{}
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	client := NewRetryClient(primary, alexerrors.RetryConfig{
		MaxAttempts: 1,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    50 * time.Millisecond,
	}, breaker)

	rc, ok := client.(*retryClient)
	require.True(t, ok)

	rc.provider = "anthropic"
	rc.model = "claude-sonnet-4-6"
	rc.sleepFn = func(ctx context.Context, d time.Duration) error { return nil }

	fallbackStream := &fallbackStreamMock{}
	rc.fallbackProvider = "kimi"
	rc.fallbackModel = "kimi-for-coding"
	rc.fallbackClientFn = func() (portsllm.LLMClient, error) {
		return fallbackStream, nil
	}

	var deltas []ports.ContentDelta
	streaming := portsllm.StreamingLLMClient(rc)
	resp, err := streaming.StreamComplete(context.Background(), ports.CompletionRequest{}, ports.CompletionStreamCallbacks{
		OnContentDelta: func(delta ports.ContentDelta) {
			deltas = append(deltas, delta)
		},
	})
	require.NoError(t, err)
	require.Equal(t, "fallback-stream-ok", resp.Content)
	require.Equal(t, 1, fallbackStream.calls, "fallback should be called once")
}

// overloadedStreamMock always returns 529 on StreamComplete.
type overloadedStreamMock struct {
	calls int
}

func (m *overloadedStreamMock) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	return nil, &alexerrors.TransientError{Err: errors.New("529 overloaded"), StatusCode: 529}
}

func (m *overloadedStreamMock) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	m.calls++
	return nil, &alexerrors.TransientError{
		Err:        errors.New("HTTP 529: overloaded"),
		StatusCode: 529,
		Message:    "Server overloaded (529). Retrying request.",
	}
}

func (m *overloadedStreamMock) Model() string { return "claude-sonnet-4-6" }

// fallbackStreamMock succeeds on StreamComplete.
type fallbackStreamMock struct {
	calls int
}

func (m *fallbackStreamMock) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	return &ports.CompletionResponse{Content: "fallback-stream-ok"}, nil
}

func (m *fallbackStreamMock) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	m.calls++
	if callbacks.OnContentDelta != nil {
		callbacks.OnContentDelta(ports.ContentDelta{Delta: "fallback-stream-ok"})
		callbacks.OnContentDelta(ports.ContentDelta{Final: true})
	}
	return &ports.CompletionResponse{Content: "fallback-stream-ok"}, nil
}

func (m *fallbackStreamMock) Model() string { return "kimi-for-coding" }

func TestRetryClientClassify529AsTransient(t *testing.T) {
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	rc := &retryClient{
		underlying:     &streamingMockClient{content: "ok"},
		circuitBreaker: breaker,
		logger:         &noopLogger{},
		llmLogger:      &noopLogger{},
	}

	err := rc.classifyLLMError(errors.New("HTTP 529: overloaded"))
	require.True(t, alexerrors.IsTransient(err), "529/overloaded should be classified as transient")
}

type noopLogger struct{}

func (noopLogger) Debug(format string, args ...any) {}
func (noopLogger) Info(format string, args ...any)  {}
func (noopLogger) Warn(format string, args ...any)  {}
func (noopLogger) Error(format string, args ...any) {}

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

// --- Rate-limit backoff and circuit tests ---

// rateLimitMock always returns 429 errors.
type rateLimitMock struct {
	calls int
}

func (m *rateLimitMock) Complete(_ context.Context, _ ports.CompletionRequest) (*ports.CompletionResponse, error) {
	m.calls++
	return nil, &alexerrors.TransientError{
		Err:        errors.New("429 Too Many Requests"),
		StatusCode: 429,
		Message:    "rate limit",
	}
}

func (m *rateLimitMock) Model() string { return "mock" }

// rateLimitThenOKMock returns 429 for the first N calls, then succeeds.
type rateLimitThenOKMock struct {
	failCount int // how many 429s to return
	calls     int
}

func (m *rateLimitThenOKMock) Complete(_ context.Context, _ ports.CompletionRequest) (*ports.CompletionResponse, error) {
	m.calls++
	if m.calls <= m.failCount {
		return nil, &alexerrors.TransientError{
			Err:        errors.New("429 Too Many Requests"),
			StatusCode: 429,
			Message:    "rate limit",
		}
	}
	return &ports.CompletionResponse{Content: "ok"}, nil
}

func (m *rateLimitThenOKMock) Model() string { return "mock" }

func TestRetryClient429UsesAggressiveBackoff(t *testing.T) {
	mock := &rateLimitThenOKMock{failCount: 1}
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.CircuitBreakerConfig{
		FailureThreshold: 10, // high so general breaker doesn't trip
		SuccessThreshold: 1,
		Timeout:          time.Minute,
	})
	client := NewRetryClient(mock, alexerrors.RetryConfig{
		MaxAttempts: 2,
		BaseDelay:   100 * time.Millisecond, // low default — 429 should override
		MaxDelay:    30 * time.Second,
	}, breaker)

	rc := client.(*retryClient)
	var waits []time.Duration
	rc.sleepFn = func(_ context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}

	resp, err := rc.Complete(context.Background(), ports.CompletionRequest{})
	require.NoError(t, err)
	require.Equal(t, "ok", resp.Content)
	require.Len(t, waits, 1)
	// Should use rateLimitBaseDelay (5s) ± jitter, NOT the 100ms BaseDelay.
	require.GreaterOrEqual(t, waits[0], 3750*time.Millisecond, "429 backoff should be >= 5s * 0.75 (jitter)")
	require.LessOrEqual(t, waits[0], 6250*time.Millisecond, "429 backoff should be <= 5s * 1.25 (jitter)")
}

func TestRetryClient429RespectsRetryAfterUncapped(t *testing.T) {
	// Verify that Retry-After for 429 is NOT capped by MaxDelay.
	rc := &retryClient{
		retryConfig: alexerrors.RetryConfig{
			BaseDelay: 100 * time.Millisecond,
			MaxDelay:  2 * time.Second, // very low MaxDelay
		},
	}
	// 429 with Retry-After=60s — should NOT be capped to 2s.
	delay := rc.retryDelay(0, &alexerrors.TransientError{
		Err:        errors.New("429"),
		StatusCode: 429,
		RetryAfter: 60,
	})
	require.Equal(t, 60*time.Second, delay, "429 Retry-After must not be capped by MaxDelay")
}

func TestRetryClientNon429RetryAfterStillCapped(t *testing.T) {
	// Verify that Retry-After for non-429 errors IS still capped by MaxDelay.
	rc := &retryClient{
		retryConfig: alexerrors.RetryConfig{
			BaseDelay: 100 * time.Millisecond,
			MaxDelay:  2 * time.Second,
		},
	}
	delay := rc.retryDelay(0, &alexerrors.TransientError{
		Err:        errors.New("503"),
		StatusCode: 503,
		RetryAfter: 60,
	})
	require.Equal(t, 2*time.Second, delay, "non-429 Retry-After should be capped by MaxDelay")
}

func TestRetryClientRateLimitCircuitOpensAfter3Consecutive429s(t *testing.T) {
	mock := &rateLimitMock{}
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.CircuitBreakerConfig{
		FailureThreshold: 10, // high so general breaker doesn't trip
		SuccessThreshold: 1,
		Timeout:          time.Minute,
	})
	client := NewRetryClient(mock, alexerrors.RetryConfig{
		MaxAttempts: 5, // enough attempts to hit the RL circuit
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    30 * time.Second,
	}, breaker)

	rc := client.(*retryClient)
	rc.sleepFn = func(_ context.Context, _ time.Duration) error { return nil }

	_, err := rc.Complete(context.Background(), ports.CompletionRequest{})
	require.Error(t, err)

	// After 3 consecutive 429s the rate-limit circuit should be open.
	// The 4th attempt should be blocked by the circuit, so total calls <= 3.
	require.LessOrEqual(t, mock.calls, rateLimitCircuitThreshold,
		"rate-limit circuit should block further attempts after %d consecutive 429s", rateLimitCircuitThreshold)

	// Complete() formats DegradedErrors into plain strings for the LLM,
	// so we check the message content instead of the error type.
	require.Contains(t, err.Error(), "Rate limit circuit open",
		"error should indicate rate-limit circuit is open")
}

func TestRetryClientRateLimitCircuitResetsOnSuccess(t *testing.T) {
	// 2 consecutive 429s, then success — circuit should NOT trip (threshold is 3).
	mock := &rateLimitThenOKMock{failCount: 2}
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.CircuitBreakerConfig{
		FailureThreshold: 10,
		SuccessThreshold: 1,
		Timeout:          time.Minute,
	})
	client := NewRetryClient(mock, alexerrors.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    30 * time.Second,
	}, breaker)

	rc := client.(*retryClient)
	rc.sleepFn = func(_ context.Context, _ time.Duration) error { return nil }

	resp, err := rc.Complete(context.Background(), ports.CompletionRequest{})
	require.NoError(t, err)
	require.Equal(t, "ok", resp.Content)
	require.Equal(t, 3, mock.calls, "should retry twice then succeed")

	// Counter should be reset after the success.
	rc.rlMu.Lock()
	require.Equal(t, 0, rc.rlConsecutive429, "consecutive counter should reset after success")
	rc.rlMu.Unlock()
}

func TestRetryClientClassifyLLMErrorSets429StatusCode(t *testing.T) {
	rc := &retryClient{
		logger: logging.NewComponentLogger("test"),
	}
	// An error containing "429" that is not already a TransientError.
	classified := rc.classifyLLMError(errors.New("API error 429: rate limit exceeded"))
	var terr *alexerrors.TransientError
	require.True(t, errors.As(classified, &terr))
	require.Equal(t, 429, terr.StatusCode, "classifyLLMError should set StatusCode=429 for rate limit errors")
}
