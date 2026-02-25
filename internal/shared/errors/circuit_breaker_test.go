package errors

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCircuitBreaker_ClosedState(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	}

	cb := NewCircuitBreaker("test", config)

	// Circuit should start closed
	if cb.State() != StateClosed {
		t.Errorf("Circuit breaker should start in closed state, got %v", cb.State())
	}

	// Successful requests should keep circuit closed
	for i := 0; i < 5; i++ {
		err := cb.Execute(context.Background(), func(ctx context.Context) error {
			return nil
		})
		if err != nil {
			t.Errorf("Execute() returned error: %v", err)
		}
	}

	if cb.State() != StateClosed {
		t.Errorf("Circuit breaker should remain closed after successful requests, got %v", cb.State())
	}
}

func TestCircuitBreaker_OpenAfterFailures(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	}

	cb := NewCircuitBreaker("test", config)

	// Trigger failures to open circuit
	for i := 0; i < 3; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return errors.New("failure")
		})
	}

	// Circuit should be open
	if cb.State() != StateOpen {
		t.Errorf("Circuit breaker should be open after %d failures, got %v", config.FailureThreshold, cb.State())
	}

	// Subsequent requests should be rejected immediately
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		t.Error("Function should not be called when circuit is open")
		return nil
	})

	if err == nil {
		t.Error("Execute() should return error when circuit is open")
	}

	if !IsDegraded(err) {
		t.Errorf("Error should be degraded error, got: %v", err)
	}
}

func TestCircuitBreaker_HalfOpenTransition(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	}

	cb := NewCircuitBreaker("test", config)

	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return errors.New("failure")
		})
	}

	if cb.State() != StateOpen {
		t.Errorf("Circuit should be open, got %v", cb.State())
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Next request should transition to half-open
	executed := false
	_ = cb.Execute(context.Background(), func(ctx context.Context) error {
		executed = true
		return nil
	})

	if !executed {
		t.Error("Function should be executed in half-open state")
	}

	// State might be closed or half-open depending on timing
	state := cb.State()
	if state != StateHalfOpen && state != StateClosed {
		t.Errorf("Circuit should be half-open or closed, got %v", state)
	}
}

func TestCircuitBreaker_RecoveryFlow(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	}

	cb := NewCircuitBreaker("test", config)

	// 1. Open the circuit with failures
	for i := 0; i < 2; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return errors.New("failure")
		})
	}

	if cb.State() != StateOpen {
		t.Errorf("Circuit should be open, got %v", cb.State())
	}

	// 2. Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// 3. Send successful requests to close circuit
	for i := 0; i < 2; i++ {
		err := cb.Execute(context.Background(), func(ctx context.Context) error {
			return nil
		})
		if err != nil {
			t.Errorf("Execute() returned error: %v", err)
		}
	}

	// 4. Circuit should be closed
	if cb.State() != StateClosed {
		t.Errorf("Circuit should be closed after recovery, got %v", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	}

	cb := NewCircuitBreaker("test", config)

	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return errors.New("failure")
		})
	}

	// Wait for timeout to allow half-open
	time.Sleep(60 * time.Millisecond)

	// Fail in half-open state (should reopen circuit)
	_ = cb.Execute(context.Background(), func(ctx context.Context) error {
		return errors.New("failure")
	})

	// Circuit should be open again
	if cb.State() != StateOpen {
		t.Errorf("Circuit should be open after half-open failure, got %v", cb.State())
	}
}

func TestCircuitBreaker_Metrics(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	}

	cb := NewCircuitBreaker("test-metrics", config)

	// Execute some requests
	_ = cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	_ = cb.Execute(context.Background(), func(ctx context.Context) error {
		return errors.New("failure")
	})

	metrics := cb.Metrics()

	if metrics.Name != "test-metrics" {
		t.Errorf("Metrics.Name = %q, want %q", metrics.Name, "test-metrics")
	}

	if metrics.State != StateClosed {
		t.Errorf("Metrics.State = %v, want %v", metrics.State, StateClosed)
	}

	if metrics.FailureCount != 1 {
		t.Errorf("Metrics.FailureCount = %d, want 1", metrics.FailureCount)
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	}

	cb := NewCircuitBreaker("test", config)

	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return errors.New("failure")
		})
	}

	if cb.State() != StateOpen {
		t.Errorf("Circuit should be open, got %v", cb.State())
	}

	// Reset
	cb.Reset()

	if cb.State() != StateClosed {
		t.Errorf("Circuit should be closed after reset, got %v", cb.State())
	}

	metrics := cb.Metrics()
	if metrics.FailureCount != 0 {
		t.Errorf("FailureCount should be 0 after reset, got %d", metrics.FailureCount)
	}
}

func TestCircuitBreaker_ExecuteFunc(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	t.Run("success", func(t *testing.T) {
		result, err := ExecuteFunc(cb, context.Background(), func(ctx context.Context) (int, error) {
			return 42, nil
		})

		if err != nil {
			t.Errorf("ExecuteFunc() returned error: %v", err)
		}

		if result != 42 {
			t.Errorf("ExecuteFunc() result = %d, want 42", result)
		}
	})

	t.Run("failure", func(t *testing.T) {
		result, err := ExecuteFunc(cb, context.Background(), func(ctx context.Context) (string, error) {
			return "", errors.New("failure")
		})

		if err == nil {
			t.Error("ExecuteFunc() should return error")
		}

		if result != "" {
			t.Errorf("ExecuteFunc() result = %q, want empty string", result)
		}
	})
}

func TestCircuitBreaker_Concurrent(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 10,
		SuccessThreshold: 5,
		Timeout:          100 * time.Millisecond,
	}

	cb := NewCircuitBreaker("concurrent", config)

	var wg sync.WaitGroup
	concurrency := 50
	iterations := 100

	// Run concurrent requests
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = cb.Execute(context.Background(), func(ctx context.Context) error {
					// Randomly succeed or fail
					if (id+j)%3 == 0 {
						return errors.New("failure")
					}
					return nil
				})
			}
		}(i)
	}

	wg.Wait()

	// Verify circuit breaker is still functional
	metrics := cb.Metrics()
	t.Logf("Final state: %v, failures: %d", metrics.State, metrics.FailureCount)
}

func TestCircuitBreaker_StateChangeCallback(t *testing.T) {
	stateChanges := make([]string, 0)
	var mu sync.Mutex

	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
		OnStateChange: func(from, to CircuitState, name string) {
			mu.Lock()
			stateChanges = append(stateChanges, fmt.Sprintf("%s->%s", from, to))
			mu.Unlock()
		},
	}

	cb := NewCircuitBreaker("test", config)

	// Open circuit
	for i := 0; i < 2; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return errors.New("failure")
		})
	}

	// Wait for timeout and transition to half-open
	time.Sleep(60 * time.Millisecond)
	_ = cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})

	// Give callback time to execute
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(stateChanges) == 0 {
		t.Error("OnStateChange callback was not called")
	}

	// Should have at least closed->open transition
	foundClosedToOpen := false
	for _, change := range stateChanges {
		if change == "closed->open" {
			foundClosedToOpen = true
			break
		}
	}

	if !foundClosedToOpen {
		t.Errorf("Expected closed->open transition, got: %v", stateChanges)
	}
}

func TestCircuitBreakerManager(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	manager := NewCircuitBreakerManager(config)

	t.Run("get creates circuit breaker", func(t *testing.T) {
		cb1 := manager.Get("service1")
		if cb1 == nil {
			t.Error("Get() should return circuit breaker")
		}

		// Getting again should return same instance
		cb2 := manager.Get("service1")
		if cb1 != cb2 {
			t.Error("Get() should return same instance for same name")
		}
	})

	t.Run("get different names", func(t *testing.T) {
		cb1 := manager.Get("service1")
		cb2 := manager.Get("service2")

		if cb1 == cb2 {
			t.Error("Get() should return different instances for different names")
		}
	})

	t.Run("metrics", func(t *testing.T) {
		_ = manager.Get("service1")
		_ = manager.Get("service2")

		metrics := manager.GetMetrics()
		if len(metrics) < 2 {
			t.Errorf("GetMetrics() returned %d metrics, want at least 2", len(metrics))
		}
	})

	t.Run("reset all", func(t *testing.T) {
		cb1 := manager.Get("service1")
		cb2 := manager.Get("service2")

		// Open both circuits
		for i := 0; i < 5; i++ {
			_ = cb1.Execute(context.Background(), func(ctx context.Context) error {
				return errors.New("failure")
			})
			_ = cb2.Execute(context.Background(), func(ctx context.Context) error {
				return errors.New("failure")
			})
		}

		// Reset all
		manager.ResetAll()

		// Both should be closed
		if cb1.State() != StateClosed {
			t.Errorf("service1 should be closed after ResetAll, got %v", cb1.State())
		}
		if cb2.State() != StateClosed {
			t.Errorf("service2 should be closed after ResetAll, got %v", cb2.State())
		}
	})

	t.Run("remove", func(t *testing.T) {
		_ = manager.Get("temp-service")
		manager.Remove("temp-service")

		// Getting again should create new instance
		cb := manager.Get("temp-service")
		if cb == nil {
			t.Error("Get() should create new instance after removal")
		}
	})
}

func TestDefaultCircuitBreakerConfig(t *testing.T) {
	config := DefaultCircuitBreakerConfig()

	if config.FailureThreshold != 5 {
		t.Errorf("DefaultCircuitBreakerConfig().FailureThreshold = %d, want 5", config.FailureThreshold)
	}

	if config.SuccessThreshold != 2 {
		t.Errorf("DefaultCircuitBreakerConfig().SuccessThreshold = %d, want 2", config.SuccessThreshold)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("DefaultCircuitBreakerConfig().Timeout = %v, want 30s", config.Timeout)
	}
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.state.String()
			if result != tt.expected {
				t.Errorf("CircuitState.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// Benchmark tests

func BenchmarkCircuitBreaker_Success(b *testing.B) {
	config := DefaultCircuitBreakerConfig()
	cb := NewCircuitBreaker("bench", config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return nil
		})
	}
}

func BenchmarkCircuitBreaker_Open(b *testing.B) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          1 * time.Hour, // Keep open
	}
	cb := NewCircuitBreaker("bench", config)

	// Open the circuit
	for i := 0; i < 3; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return errors.New("failure")
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return nil
		})
	}
}
