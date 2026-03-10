package llm

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	alexerrors "alex/internal/shared/errors"
)

// ---------- Mock clients for benchmarks ----------

// instantOKMock returns immediately with a success response.
type instantOKMock struct{}

func (m *instantOKMock) Complete(_ context.Context, _ ports.CompletionRequest) (*ports.CompletionResponse, error) {
	return &ports.CompletionResponse{Content: "ok"}, nil
}
func (m *instantOKMock) Model() string { return "bench-mock" }

// rateLimitNMock returns 429 for the first N calls, then succeeds.
type rateLimitNMock struct {
	failCount int
	calls     atomic.Int64
}

func (m *rateLimitNMock) Complete(_ context.Context, _ ports.CompletionRequest) (*ports.CompletionResponse, error) {
	n := m.calls.Add(1)
	if int(n) <= m.failCount {
		return nil, &alexerrors.TransientError{
			Err:        errors.New("429 Too Many Requests"),
			StatusCode: 429,
			Message:    "rate limit",
		}
	}
	return &ports.CompletionResponse{Content: "ok"}, nil
}
func (m *rateLimitNMock) Model() string { return "bench-mock" }

// overloaded529Mock always returns 529.
type overloaded529Mock struct{}

func (m *overloaded529Mock) Complete(_ context.Context, _ ports.CompletionRequest) (*ports.CompletionResponse, error) {
	return nil, &alexerrors.TransientError{
		Err:        errors.New("HTTP 529: overloaded"),
		StatusCode: 529,
		Message:    "Server overloaded",
	}
}
func (m *overloaded529Mock) Model() string { return "bench-primary" }

// instantFallbackMock succeeds immediately.
type instantFallbackMock struct{}

func (m *instantFallbackMock) Complete(_ context.Context, _ ports.CompletionRequest) (*ports.CompletionResponse, error) {
	return &ports.CompletionResponse{Content: "fallback-ok"}, nil
}
func (m *instantFallbackMock) Model() string { return "bench-fallback" }

// ---------- Benchmark 1: Normal request latency ----------

func BenchmarkComplete_NormalRequest(b *testing.B) {
	mock := &instantOKMock{}
	breaker := alexerrors.NewCircuitBreaker("bench", alexerrors.DefaultCircuitBreakerConfig())
	client := NewRetryClient(mock, alexerrors.DefaultRetryConfig(), breaker)

	ctx := context.Background()
	req := ports.CompletionRequest{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		resp, err := client.Complete(ctx, req)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
		if resp.Content != "ok" {
			b.Fatalf("unexpected content: %s", resp.Content)
		}
	}
}

// ---------- Benchmark 2: 429 retry backoff accuracy ----------

func BenchmarkComplete_429RetryBackoff(b *testing.B) {
	// Measures the overhead of a single 429 retry cycle (1 failure + 1 success).
	// sleepFn records the delay but doesn't actually sleep.
	breaker := alexerrors.NewCircuitBreaker("bench", alexerrors.CircuitBreakerConfig{
		FailureThreshold: 100,
		SuccessThreshold: 1,
		Timeout:          time.Minute,
	})

	ctx := context.Background()
	req := ports.CompletionRequest{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		mock := &rateLimitNMock{failCount: 1}
		client := NewRetryClient(mock, alexerrors.RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    30 * time.Second,
		}, breaker)
		rc := client.(*retryClient)
		rc.sleepFn = func(_ context.Context, _ time.Duration) error { return nil }
		b.StartTimer()

		resp, err := rc.Complete(ctx, req)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
		if resp.Content != "ok" {
			b.Fatalf("unexpected content: %s", resp.Content)
		}
	}
}

func TestBenchmarkHelper_429BackoffAccuracy(t *testing.T) {
	// Verify that 429 backoff uses rateLimitBaseDelay (5s) ± jitter.
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.CircuitBreakerConfig{
		FailureThreshold: 100,
		SuccessThreshold: 1,
		Timeout:          time.Minute,
	})

	mock := &rateLimitNMock{failCount: 1}
	client := NewRetryClient(mock, alexerrors.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    30 * time.Second,
	}, breaker)
	rc := client.(*retryClient)

	var waits []time.Duration
	rc.sleepFn = func(_ context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}

	_, err := rc.Complete(context.Background(), ports.CompletionRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waits) != 1 {
		t.Fatalf("expected 1 wait, got %d", len(waits))
	}
	// 429 backoff = rateLimitBaseDelay (5s) ± 25% jitter = [3.75s, 6.25s]
	if waits[0] < 3750*time.Millisecond || waits[0] > 6250*time.Millisecond {
		t.Errorf("429 backoff = %v, want [3.75s, 6.25s]", waits[0])
	}
}

// ---------- Benchmark 3: 529 provider failover switch latency ----------

func BenchmarkComplete_529ProviderFailover(b *testing.B) {
	// Measures the overhead of retries exhausting + fallback provider switch.
	ctx := context.Background()
	req := ports.CompletionRequest{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		breaker := alexerrors.NewCircuitBreaker("bench", alexerrors.CircuitBreakerConfig{
			FailureThreshold: 100,
			SuccessThreshold: 1,
			Timeout:          time.Minute,
		})
		primary := &overloaded529Mock{}
		client := NewRetryClient(primary, alexerrors.RetryConfig{
			MaxAttempts: 1,
			BaseDelay:   10 * time.Millisecond,
			MaxDelay:    50 * time.Millisecond,
		}, breaker)
		rc := client.(*retryClient)
		rc.provider = "anthropic"
		rc.model = "claude-sonnet-4-6"
		rc.sleepFn = func(_ context.Context, _ time.Duration) error { return nil }
		rc.fallbackProvider = "kimi"
		rc.fallbackModel = "kimi-for-coding"
		rc.fallbackClientFn = func() (portsllm.LLMClient, error) {
			return &instantFallbackMock{}, nil
		}
		b.StartTimer()

		resp, err := rc.Complete(ctx, req)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
		if resp.Content != "fallback-ok" {
			b.Fatalf("unexpected content: %s", resp.Content)
		}
	}
}

// ---------- Benchmark 4: Circuit breaker recovery time ----------

func BenchmarkCircuitBreakerRecovery(b *testing.B) {
	// Measures the overhead of the general circuit breaker state transitions:
	// closed → open (after failures) → half-open → closed (after success).
	ctx := context.Background()
	req := ports.CompletionRequest{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		callCount := 0
		failUntil := 5 // match FailureThreshold
		mock := &streamingMockClient{content: "ok"}

		breaker := alexerrors.NewCircuitBreaker("bench-cb", alexerrors.CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 2,
			Timeout:          1 * time.Millisecond, // very short for benchmarking
		})

		client := NewRetryClient(mock, alexerrors.RetryConfig{
			MaxAttempts: 0, // no retry — we control attempts manually
			BaseDelay:   1 * time.Millisecond,
			MaxDelay:    1 * time.Millisecond,
		}, breaker)
		rc := client.(*retryClient)
		rc.sleepFn = func(_ context.Context, _ time.Duration) error { return nil }

		// Override mock to fail first N calls.
		failingMock := &failThenSucceedMock{failUntil: failUntil}
		rc.underlying = failingMock
		_ = callCount

		b.StartTimer()

		// Phase 1: Drive circuit to open with failures.
		for j := 0; j < failUntil; j++ {
			_, _ = rc.Complete(ctx, req)
		}

		// Phase 2: Wait for half-open transition (timeout is 1ms).
		time.Sleep(2 * time.Millisecond)

		// Phase 3: Succeed to close circuit.
		for j := 0; j < 3; j++ {
			_, _ = rc.Complete(ctx, req)
		}
	}
}

// failThenSucceedMock fails for the first `failUntil` calls, then succeeds.
type failThenSucceedMock struct {
	failUntil int
	calls     int
}

func (m *failThenSucceedMock) Complete(_ context.Context, _ ports.CompletionRequest) (*ports.CompletionResponse, error) {
	m.calls++
	if m.calls <= m.failUntil {
		return nil, &alexerrors.TransientError{
			Err:        errors.New("HTTP 500: internal server error"),
			StatusCode: 500,
		}
	}
	return &ports.CompletionResponse{Content: "recovered"}, nil
}
func (m *failThenSucceedMock) Model() string { return "bench-mock" }

// ---------- Benchmark: calculateBackoff isolation ----------

func BenchmarkCalculateBackoff(b *testing.B) {
	rc := &retryClient{
		retryConfig: alexerrors.RetryConfig{
			BaseDelay:    time.Second,
			MaxDelay:     30 * time.Second,
			JitterFactor: 0.25,
		},
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = rc.calculateBackoff(i % 10)
	}
}

// ---------- Benchmark: retryDelay for 429 with Retry-After ----------

func BenchmarkRetryDelay_429WithRetryAfter(b *testing.B) {
	rc := &retryClient{
		retryConfig: alexerrors.RetryConfig{
			BaseDelay: 100 * time.Millisecond,
			MaxDelay:  30 * time.Second,
		},
	}
	err429 := &alexerrors.TransientError{
		Err:        errors.New("429"),
		StatusCode: 429,
		RetryAfter: 10,
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = rc.retryDelay(0, err429)
	}
}

// ---------- Benchmark: Rate-limit circuit check ----------

func BenchmarkCheckRateLimitCircuit_Closed(b *testing.B) {
	breaker := alexerrors.NewCircuitBreaker("bench", alexerrors.DefaultCircuitBreakerConfig())
	rc := &retryClient{
		underlying:     &instantOKMock{},
		circuitBreaker: breaker,
		logger:         &noopLogger{},
		llmLogger:      &noopLogger{},
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = rc.checkRateLimitCircuit()
	}
}

func BenchmarkCheckRateLimitCircuit_Open(b *testing.B) {
	breaker := alexerrors.NewCircuitBreaker("bench", alexerrors.DefaultCircuitBreakerConfig())
	rc := &retryClient{
		underlying:        &instantOKMock{},
		circuitBreaker:    breaker,
		logger:            &noopLogger{},
		llmLogger:         &noopLogger{},
		rlConsecutive429:  rateLimitCircuitThreshold,
		rlCircuitOpenedAt: time.Now(),
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = rc.checkRateLimitCircuit()
	}
}

// ---------- Benchmark: classifyLLMError ----------

func BenchmarkClassifyLLMError_429(b *testing.B) {
	rc := &retryClient{
		logger:    &noopLogger{},
		llmLogger: &noopLogger{},
	}
	err429 := errors.New("API error 429: rate limit exceeded")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = rc.classifyLLMError(err429)
	}
}

func BenchmarkClassifyLLMError_Transient529(b *testing.B) {
	rc := &retryClient{
		logger:    &noopLogger{},
		llmLogger: &noopLogger{},
	}
	err529 := errors.New("HTTP 529: overloaded")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = rc.classifyLLMError(err529)
	}
}

func BenchmarkClassifyLLMError_AlreadyTyped(b *testing.B) {
	rc := &retryClient{
		logger:    &noopLogger{},
		llmLogger: &noopLogger{},
	}
	typed := &alexerrors.TransientError{Err: errors.New("500"), StatusCode: 500}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = rc.classifyLLMError(typed)
	}
}
