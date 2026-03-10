package llm

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	alexerrors "alex/internal/shared/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthRegistry_Register(t *testing.T) {
	t.Parallel()

	hr := newHealthRegistry()
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	hr.register("openai", "gpt-4", breaker)

	h := hr.getHealth("openai", "gpt-4")
	assert.Equal(t, "openai", h.Provider)
	assert.Equal(t, "gpt-4", h.Model)
	assert.Equal(t, HealthStateHealthy, h.State)
	assert.Equal(t, 0, h.FailureCount)
	assert.Empty(t, h.LastError)
}

func TestHealthRegistry_RegisterUnknownProvider(t *testing.T) {
	t.Parallel()

	hr := newHealthRegistry()
	// Getting health for an unregistered provider returns a default healthy snapshot.
	h := hr.getHealth("unknown", "model")
	assert.Equal(t, "unknown", h.Provider)
	assert.Equal(t, "model", h.Model)
	assert.Equal(t, HealthStateHealthy, h.State)
}

func TestHealthRegistry_RecordLatency(t *testing.T) {
	t.Parallel()

	hr := newHealthRegistry()
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	hr.register("openai", "gpt-4", breaker)

	// Record deterministic latencies for P50/P95 verification.
	// 20 samples: 1ms, 2ms, ..., 20ms
	for i := 1; i <= 20; i++ {
		hr.recordLatency("openai", "gpt-4", time.Duration(i)*time.Millisecond)
	}

	h := hr.getHealth("openai", "gpt-4")

	// P50 of 1..20 sorted: index = int(0.50 * 19) = 9 -> 10ms
	assert.Equal(t, 10*time.Millisecond, h.Latency.P50)

	// P95 of 1..20 sorted: index = int(0.95 * 19) = 18 -> 19ms
	assert.Equal(t, 19*time.Millisecond, h.Latency.P95)

	// Avg = sum(1..20)/20 = 210/20 = 10.5ms
	assert.Equal(t, 10*time.Millisecond+500*time.Microsecond, h.Latency.Avg)
}

func TestHealthRegistry_RecordError(t *testing.T) {
	t.Parallel()

	hr := newHealthRegistry()
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	hr.register("anthropic", "claude-3", breaker)

	hr.recordError("anthropic", "claude-3", fmt.Errorf("rate limit exceeded"))
	hr.recordError("anthropic", "claude-3", fmt.Errorf("server error 500"))

	h := hr.getHealth("anthropic", "claude-3")
	assert.Equal(t, 2, h.FailureCount)
	assert.Equal(t, "server error 500", h.LastError)
}

func TestHealthRegistry_HealthyState(t *testing.T) {
	t.Parallel()

	hr := newHealthRegistry()
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	hr.register("openai", "gpt-4", breaker)

	// Only successes.
	for i := 0; i < 10; i++ {
		hr.recordLatency("openai", "gpt-4", 50*time.Millisecond)
	}

	h := hr.getHealth("openai", "gpt-4")
	assert.Equal(t, HealthStateHealthy, h.State)
}

func TestHealthRegistry_DegradedState(t *testing.T) {
	t.Parallel()

	// Use a circuit breaker in half-open state.
	cfg := alexerrors.CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          1 * time.Millisecond, // Very short so it transitions quickly.
	}
	breaker := alexerrors.NewCircuitBreaker("test", cfg)

	hr := newHealthRegistry()
	hr.register("openai", "gpt-4", breaker)

	// Drive breaker to open: 2 failures.
	breaker.Mark(fmt.Errorf("fail"))
	breaker.Mark(fmt.Errorf("fail"))

	// Breaker should now be open.
	assert.Equal(t, alexerrors.StateOpen, breaker.State())

	// Wait for timeout so it transitions to half-open on next Allow().
	time.Sleep(5 * time.Millisecond)
	err := breaker.Allow()
	require.NoError(t, err)
	assert.Equal(t, alexerrors.StateHalfOpen, breaker.State())

	h := hr.getHealth("openai", "gpt-4")
	assert.Equal(t, HealthStateDegraded, h.State)
}

func TestHealthRegistry_DegradedStateFromErrorRate(t *testing.T) {
	t.Parallel()

	hr := newHealthRegistry()
	// No circuit breaker — derive state from error rate.
	hr.register("deepseek", "chat", nil)

	// 90 successes + 10 errors = 10% error rate -> degraded (5-20%).
	for i := 0; i < 90; i++ {
		hr.recordLatency("deepseek", "chat", 10*time.Millisecond)
	}
	for i := 0; i < 10; i++ {
		hr.recordError("deepseek", "chat", fmt.Errorf("error"))
	}

	h := hr.getHealth("deepseek", "chat")
	assert.Equal(t, HealthStateDegraded, h.State)
}

func TestHealthRegistry_DownState(t *testing.T) {
	t.Parallel()

	cfg := alexerrors.CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}
	breaker := alexerrors.NewCircuitBreaker("test", cfg)

	hr := newHealthRegistry()
	hr.register("openai", "gpt-4", breaker)

	// Drive breaker to open.
	breaker.Mark(fmt.Errorf("fail"))
	breaker.Mark(fmt.Errorf("fail"))
	assert.Equal(t, alexerrors.StateOpen, breaker.State())

	h := hr.getHealth("openai", "gpt-4")
	assert.Equal(t, HealthStateDown, h.State)
}

func TestHealthRegistry_DownStateFromErrorRate(t *testing.T) {
	t.Parallel()

	hr := newHealthRegistry()
	hr.register("deepseek", "chat", nil)

	// 70 successes + 30 errors = 30% error rate -> down (>20%).
	for i := 0; i < 70; i++ {
		hr.recordLatency("deepseek", "chat", 10*time.Millisecond)
	}
	for i := 0; i < 30; i++ {
		hr.recordError("deepseek", "chat", fmt.Errorf("error"))
	}

	h := hr.getHealth("deepseek", "chat")
	assert.Equal(t, HealthStateDown, h.State)
}

func TestHealthRegistry_Concurrent(t *testing.T) {
	t.Parallel()

	hr := newHealthRegistry()
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	hr.register("openai", "gpt-4", breaker)

	var wg sync.WaitGroup
	const goroutines = 50
	const opsPerGoroutine = 100

	// Writers: record latency.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				hr.recordLatency("openai", "gpt-4", time.Duration(j)*time.Microsecond)
			}
		}()
	}

	// Writers: record errors.
	for i := 0; i < goroutines/5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				hr.recordError("openai", "gpt-4", fmt.Errorf("err %d", j))
			}
		}()
	}

	// Readers: get health.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				_ = hr.getHealth("openai", "gpt-4")
			}
		}()
	}

	// Readers: get all health.
	for i := 0; i < goroutines/5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				_ = hr.getAllHealth()
			}
		}()
	}

	wg.Wait()

	// Should not panic; just verify we get a result.
	h := hr.getHealth("openai", "gpt-4")
	assert.NotEmpty(t, h.Provider)
}

func TestHealthRegistry_RollingWindow(t *testing.T) {
	t.Parallel()

	hr := newHealthRegistry()
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	hr.register("openai", "gpt-4", breaker)

	// Fill the latency window completely with 10ms values.
	for i := 0; i < latencyWindowSize; i++ {
		hr.recordLatency("openai", "gpt-4", 10*time.Millisecond)
	}

	h1 := hr.getHealth("openai", "gpt-4")
	assert.Equal(t, 10*time.Millisecond, h1.Latency.Avg)

	// Now overwrite entire window with 20ms values. Old 10ms entries should be gone.
	for i := 0; i < latencyWindowSize; i++ {
		hr.recordLatency("openai", "gpt-4", 20*time.Millisecond)
	}

	h2 := hr.getHealth("openai", "gpt-4")
	assert.Equal(t, 20*time.Millisecond, h2.Latency.Avg)
	assert.Equal(t, 20*time.Millisecond, h2.Latency.P50)
	assert.Equal(t, 20*time.Millisecond, h2.Latency.P95)
}

func TestHealthRegistry_GetAllHealth(t *testing.T) {
	t.Parallel()

	hr := newHealthRegistry()
	b1 := alexerrors.NewCircuitBreaker("test-a", alexerrors.DefaultCircuitBreakerConfig())
	b2 := alexerrors.NewCircuitBreaker("test-b", alexerrors.DefaultCircuitBreakerConfig())
	hr.register("anthropic", "claude-3", b1)
	hr.register("openai", "gpt-4", b2)

	all := hr.getAllHealth()
	require.Len(t, all, 2)
	// Sorted by key: "anthropic:claude-3" < "openai:gpt-4"
	assert.Equal(t, "anthropic", all[0].Provider)
	assert.Equal(t, "openai", all[1].Provider)
}

func TestHealthRegistry_ReRegister(t *testing.T) {
	t.Parallel()

	hr := newHealthRegistry()
	b1 := alexerrors.NewCircuitBreaker("test-old", alexerrors.DefaultCircuitBreakerConfig())
	hr.register("openai", "gpt-4", b1)
	hr.recordError("openai", "gpt-4", fmt.Errorf("old error"))

	// Re-register with a new breaker; failure count should be preserved.
	b2 := alexerrors.NewCircuitBreaker("test-new", alexerrors.DefaultCircuitBreakerConfig())
	hr.register("openai", "gpt-4", b2)

	h := hr.getHealth("openai", "gpt-4")
	assert.Equal(t, 1, h.FailureCount)
	assert.Equal(t, "old error", h.LastError)
}

func TestHealthRegistry_RecordWithoutRegister(t *testing.T) {
	t.Parallel()

	hr := newHealthRegistry()

	// Recording without explicit registration should auto-create the entry.
	hr.recordLatency("deepseek", "deepseek-chat", 5*time.Millisecond)
	hr.recordError("deepseek", "deepseek-chat", fmt.Errorf("connection refused"))

	h := hr.getHealth("deepseek", "deepseek-chat")
	assert.Equal(t, "deepseek", h.Provider)
	assert.Equal(t, 1, h.FailureCount)
	assert.Equal(t, "connection refused", h.LastError)
}

func TestProviderHealth_Sanitize_StripsRawError(t *testing.T) {
	t.Parallel()

	h := ProviderHealth{
		Provider:     "openai",
		Model:        "gpt-4",
		State:        HealthStateDegraded,
		LastError:    "POST https://api.openai.com/v1/chat: 429 rate limit exceeded (key=sk-proj-abc123...)",
		FailureCount: 5,
		ErrorRate:    0.15,
		HealthScore:  44.0,
		LastChecked:  time.Now(),
		Latency:      latencyStats{P50: 100 * time.Millisecond, P95: 500 * time.Millisecond, Avg: 150 * time.Millisecond},
	}

	s := h.Sanitize()

	// Preserved fields.
	assert.Equal(t, "gpt-4", s.Model)
	assert.Equal(t, HealthStateDegraded, s.State)
	assert.Equal(t, 0.15, s.ErrorRate)
	assert.Equal(t, 44.0, s.HealthScore)
	assert.False(t, s.LastChecked.IsZero())
	assert.Equal(t, "transient", s.ErrorClass)

	// Sensitive fields must be absent.
	assert.Empty(t, sanitizedJSON(t, s, "api.openai.com"), "endpoint URL leaked")
	assert.Empty(t, sanitizedJSON(t, s, "sk-proj"), "API key leaked")
	assert.Empty(t, sanitizedJSON(t, s, "429 rate limit"), "raw error leaked")
	assert.Empty(t, sanitizedJSON(t, s, "openai"), "provider name leaked")
}

func TestProviderHealth_Sanitize_NoError(t *testing.T) {
	t.Parallel()

	h := ProviderHealth{
		Provider:    "anthropic",
		Model:       "claude-3-opus",
		State:       HealthStateHealthy,
		ErrorRate:   0,
		HealthScore: 100,
		LastChecked: time.Now(),
	}

	s := h.Sanitize()
	assert.Equal(t, "claude-3-opus", s.Model)
	assert.Equal(t, HealthStateHealthy, s.State)
	assert.Empty(t, s.ErrorClass)
}

func TestProviderHealth_Sanitize_PermanentError(t *testing.T) {
	t.Parallel()

	h := ProviderHealth{
		Provider:  "my-internal-provider",
		Model:     "gpt-4",
		State:     HealthStateDown,
		LastError: "authentication failed: invalid API key sk-abc123",
	}

	s := h.Sanitize()
	assert.Equal(t, "permanent", s.ErrorClass)
	assert.Empty(t, sanitizedJSON(t, s, "invalid API key"), "raw error leaked")
	assert.Empty(t, sanitizedJSON(t, s, "sk-abc123"), "API key leaked")
	assert.Empty(t, sanitizedJSON(t, s, "my-internal-provider"), "provider name leaked")
}

func TestSanitizeAll(t *testing.T) {
	t.Parallel()

	healths := []ProviderHealth{
		{Provider: "openai", Model: "gpt-4", LastError: "timeout after 30s to https://internal.proxy:8443/v1/chat", ErrorRate: 0.1, HealthScore: 90},
		{Provider: "anthropic", Model: "claude-3", LastError: "", ErrorRate: 0, HealthScore: 100},
	}

	sanitized := SanitizeAll(healths)
	require.Len(t, sanitized, 2)

	// First entry: error was present.
	assert.Equal(t, "gpt-4", sanitized[0].Model)
	assert.Equal(t, "transient", sanitized[0].ErrorClass)
	assert.Empty(t, sanitizedJSON(t, sanitized[0], "internal.proxy"), "internal URL leaked")
	assert.Empty(t, sanitizedJSON(t, sanitized[0], "8443"), "internal port leaked")

	// Second entry: no error.
	assert.Equal(t, "claude-3", sanitized[1].Model)
	assert.Empty(t, sanitized[1].ErrorClass)
}

func TestSanitizeAll_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, SanitizeAll(nil))
}

func TestClassifyError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg  string
		want string
	}{
		{"rate limit exceeded", "transient"},
		{"HTTP 429 Too Many Requests", "transient"},
		{"connection refused", "transient"},
		{"context deadline exceeded", "transient"},
		{"timeout after 30s", "transient"},
		{"502 Bad Gateway", "transient"},
		{"503 Service Unavailable", "transient"},
		{"server error 500", "transient"},
		{"circuit breaker open", "transient"},
		{"service overloaded", "transient"},
		{"authentication failed: invalid API key", "permanent"},
		{"model not found: gpt-5-turbo", "permanent"},
		{"invalid request: max_tokens exceeds limit", "permanent"},
		{"billing quota exceeded", "permanent"},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			assert.Equal(t, tt.want, classifyError(tt.msg))
		})
	}
}

// sanitizedJSON marshals v to JSON and returns any occurrence of needle, or empty string if clean.
func sanitizedJSON(t *testing.T, v interface{}, needle string) string {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	s := string(data)
	if idx := indexOf(s, needle); idx >= 0 {
		return s
	}
	return ""
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
