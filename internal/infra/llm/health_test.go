package llm

import (
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

	hr := NewHealthRegistry()
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	hr.Register("openai", "gpt-4", breaker)

	h := hr.GetHealth("openai", "gpt-4")
	assert.Equal(t, "openai", h.Provider)
	assert.Equal(t, "gpt-4", h.Model)
	assert.Equal(t, HealthStateHealthy, h.State)
	assert.Equal(t, 0, h.FailureCount)
	assert.Empty(t, h.LastError)
}

func TestHealthRegistry_RegisterUnknownProvider(t *testing.T) {
	t.Parallel()

	hr := NewHealthRegistry()
	// Getting health for an unregistered provider returns a default healthy snapshot.
	h := hr.GetHealth("unknown", "model")
	assert.Equal(t, "unknown", h.Provider)
	assert.Equal(t, "model", h.Model)
	assert.Equal(t, HealthStateHealthy, h.State)
}

func TestHealthRegistry_RecordLatency(t *testing.T) {
	t.Parallel()

	hr := NewHealthRegistry()
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	hr.Register("openai", "gpt-4", breaker)

	// Record deterministic latencies for P50/P95 verification.
	// 20 samples: 1ms, 2ms, ..., 20ms
	for i := 1; i <= 20; i++ {
		hr.RecordLatency("openai", "gpt-4", time.Duration(i)*time.Millisecond)
	}

	h := hr.GetHealth("openai", "gpt-4")

	// P50 of 1..20 sorted: index = int(0.50 * 19) = 9 -> 10ms
	assert.Equal(t, 10*time.Millisecond, h.Latency.P50)

	// P95 of 1..20 sorted: index = int(0.95 * 19) = 18 -> 19ms
	assert.Equal(t, 19*time.Millisecond, h.Latency.P95)

	// Avg = sum(1..20)/20 = 210/20 = 10.5ms
	assert.Equal(t, 10*time.Millisecond+500*time.Microsecond, h.Latency.Avg)
}

func TestHealthRegistry_RecordError(t *testing.T) {
	t.Parallel()

	hr := NewHealthRegistry()
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	hr.Register("anthropic", "claude-3", breaker)

	hr.RecordError("anthropic", "claude-3", fmt.Errorf("rate limit exceeded"))
	hr.RecordError("anthropic", "claude-3", fmt.Errorf("server error 500"))

	h := hr.GetHealth("anthropic", "claude-3")
	assert.Equal(t, 2, h.FailureCount)
	assert.Equal(t, "server error 500", h.LastError)
}

func TestHealthRegistry_HealthyState(t *testing.T) {
	t.Parallel()

	hr := NewHealthRegistry()
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	hr.Register("openai", "gpt-4", breaker)

	// Only successes.
	for i := 0; i < 10; i++ {
		hr.RecordLatency("openai", "gpt-4", 50*time.Millisecond)
	}

	h := hr.GetHealth("openai", "gpt-4")
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

	hr := NewHealthRegistry()
	hr.Register("openai", "gpt-4", breaker)

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

	h := hr.GetHealth("openai", "gpt-4")
	assert.Equal(t, HealthStateDegraded, h.State)
}

func TestHealthRegistry_DegradedStateFromErrorRate(t *testing.T) {
	t.Parallel()

	hr := NewHealthRegistry()
	// No circuit breaker — derive state from error rate.
	hr.Register("deepseek", "chat", nil)

	// 90 successes + 10 errors = 10% error rate -> degraded (5-20%).
	for i := 0; i < 90; i++ {
		hr.RecordLatency("deepseek", "chat", 10*time.Millisecond)
	}
	for i := 0; i < 10; i++ {
		hr.RecordError("deepseek", "chat", fmt.Errorf("error"))
	}

	h := hr.GetHealth("deepseek", "chat")
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

	hr := NewHealthRegistry()
	hr.Register("openai", "gpt-4", breaker)

	// Drive breaker to open.
	breaker.Mark(fmt.Errorf("fail"))
	breaker.Mark(fmt.Errorf("fail"))
	assert.Equal(t, alexerrors.StateOpen, breaker.State())

	h := hr.GetHealth("openai", "gpt-4")
	assert.Equal(t, HealthStateDown, h.State)
}

func TestHealthRegistry_DownStateFromErrorRate(t *testing.T) {
	t.Parallel()

	hr := NewHealthRegistry()
	hr.Register("deepseek", "chat", nil)

	// 70 successes + 30 errors = 30% error rate -> down (>20%).
	for i := 0; i < 70; i++ {
		hr.RecordLatency("deepseek", "chat", 10*time.Millisecond)
	}
	for i := 0; i < 30; i++ {
		hr.RecordError("deepseek", "chat", fmt.Errorf("error"))
	}

	h := hr.GetHealth("deepseek", "chat")
	assert.Equal(t, HealthStateDown, h.State)
}

func TestHealthRegistry_Concurrent(t *testing.T) {
	t.Parallel()

	hr := NewHealthRegistry()
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	hr.Register("openai", "gpt-4", breaker)

	var wg sync.WaitGroup
	const goroutines = 50
	const opsPerGoroutine = 100

	// Writers: record latency.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				hr.RecordLatency("openai", "gpt-4", time.Duration(j)*time.Microsecond)
			}
		}()
	}

	// Writers: record errors.
	for i := 0; i < goroutines/5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				hr.RecordError("openai", "gpt-4", fmt.Errorf("err %d", j))
			}
		}()
	}

	// Readers: get health.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				_ = hr.GetHealth("openai", "gpt-4")
			}
		}()
	}

	// Readers: get all health.
	for i := 0; i < goroutines/5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				_ = hr.GetAllHealth()
			}
		}()
	}

	wg.Wait()

	// Should not panic; just verify we get a result.
	h := hr.GetHealth("openai", "gpt-4")
	assert.NotEmpty(t, h.Provider)
}

func TestHealthRegistry_RollingWindow(t *testing.T) {
	t.Parallel()

	hr := NewHealthRegistry()
	breaker := alexerrors.NewCircuitBreaker("test", alexerrors.DefaultCircuitBreakerConfig())
	hr.Register("openai", "gpt-4", breaker)

	// Fill the latency window completely with 10ms values.
	for i := 0; i < latencyWindowSize; i++ {
		hr.RecordLatency("openai", "gpt-4", 10*time.Millisecond)
	}

	h1 := hr.GetHealth("openai", "gpt-4")
	assert.Equal(t, 10*time.Millisecond, h1.Latency.Avg)

	// Now overwrite entire window with 20ms values. Old 10ms entries should be gone.
	for i := 0; i < latencyWindowSize; i++ {
		hr.RecordLatency("openai", "gpt-4", 20*time.Millisecond)
	}

	h2 := hr.GetHealth("openai", "gpt-4")
	assert.Equal(t, 20*time.Millisecond, h2.Latency.Avg)
	assert.Equal(t, 20*time.Millisecond, h2.Latency.P50)
	assert.Equal(t, 20*time.Millisecond, h2.Latency.P95)
}

func TestHealthRegistry_GetAllHealth(t *testing.T) {
	t.Parallel()

	hr := NewHealthRegistry()
	b1 := alexerrors.NewCircuitBreaker("test-a", alexerrors.DefaultCircuitBreakerConfig())
	b2 := alexerrors.NewCircuitBreaker("test-b", alexerrors.DefaultCircuitBreakerConfig())
	hr.Register("anthropic", "claude-3", b1)
	hr.Register("openai", "gpt-4", b2)

	all := hr.GetAllHealth()
	require.Len(t, all, 2)
	// Sorted by key: "anthropic:claude-3" < "openai:gpt-4"
	assert.Equal(t, "anthropic", all[0].Provider)
	assert.Equal(t, "openai", all[1].Provider)
}

func TestHealthRegistry_ReRegister(t *testing.T) {
	t.Parallel()

	hr := NewHealthRegistry()
	b1 := alexerrors.NewCircuitBreaker("test-old", alexerrors.DefaultCircuitBreakerConfig())
	hr.Register("openai", "gpt-4", b1)
	hr.RecordError("openai", "gpt-4", fmt.Errorf("old error"))

	// Re-register with a new breaker; failure count should be preserved.
	b2 := alexerrors.NewCircuitBreaker("test-new", alexerrors.DefaultCircuitBreakerConfig())
	hr.Register("openai", "gpt-4", b2)

	h := hr.GetHealth("openai", "gpt-4")
	assert.Equal(t, 1, h.FailureCount)
	assert.Equal(t, "old error", h.LastError)
}

func TestHealthRegistry_RecordWithoutRegister(t *testing.T) {
	t.Parallel()

	hr := NewHealthRegistry()

	// Recording without explicit registration should auto-create the entry.
	hr.RecordLatency("deepseek", "deepseek-chat", 5*time.Millisecond)
	hr.RecordError("deepseek", "deepseek-chat", fmt.Errorf("connection refused"))

	h := hr.GetHealth("deepseek", "deepseek-chat")
	assert.Equal(t, "deepseek", h.Provider)
	assert.Equal(t, 1, h.FailureCount)
	assert.Equal(t, "connection refused", h.LastError)
}
