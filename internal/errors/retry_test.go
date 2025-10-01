package errors

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestRetry_Success(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  3,
		BaseDelay:    10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		JitterFactor: 0,
	}

	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		return nil // Success immediately
	}

	err := Retry(context.Background(), config, fn)
	if err != nil {
		t.Errorf("Retry() returned error: %v", err)
	}

	if attempts != 1 {
		t.Errorf("Retry() made %d attempts, want 1", attempts)
	}
}

func TestRetry_SuccessAfterRetries(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  3,
		BaseDelay:    10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		JitterFactor: 0,
	}

	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return NewTransientError(errors.New("temporary failure"), "retry me")
		}
		return nil // Success on 3rd attempt
	}

	err := Retry(context.Background(), config, fn)
	if err != nil {
		t.Errorf("Retry() returned error: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Retry() made %d attempts, want 3", attempts)
	}
}

func TestRetry_PermanentError(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  3,
		BaseDelay:    10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		JitterFactor: 0,
	}

	attempts := 0
	permanentErr := NewPermanentError(errors.New("permanent"), "don't retry")

	fn := func(ctx context.Context) error {
		attempts++
		return permanentErr
	}

	err := Retry(context.Background(), config, fn)
	if err == nil {
		t.Error("Retry() should have returned error")
	}

	if attempts != 1 {
		t.Errorf("Retry() made %d attempts, want 1 (should not retry permanent errors)", attempts)
	}

	if !errors.Is(err, permanentErr) {
		t.Errorf("Retry() error = %v, want %v", err, permanentErr)
	}
}

func TestRetry_MaxRetriesExceeded(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  3,
		BaseDelay:    10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		JitterFactor: 0,
	}

	attempts := 0
	transientErr := NewTransientError(errors.New("always fails"), "transient")

	fn := func(ctx context.Context) error {
		attempts++
		return transientErr
	}

	err := Retry(context.Background(), config, fn)
	if err == nil {
		t.Error("Retry() should have returned error")
	}

	expectedAttempts := config.MaxAttempts + 1 // Initial attempt + retries
	if attempts != expectedAttempts {
		t.Errorf("Retry() made %d attempts, want %d", attempts, expectedAttempts)
	}
}

func TestRetry_ContextCancellation(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  10,
		BaseDelay:    100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		JitterFactor: 0,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		if attempts == 2 {
			cancel() // Cancel after second attempt
		}
		return NewTransientError(errors.New("transient"), "keep trying")
	}

	err := Retry(ctx, config, fn)
	if err == nil {
		t.Error("Retry() should have returned error")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Retry() error should wrap context.Canceled, got: %v", err)
	}

	// Should stop immediately when context is cancelled
	if attempts > 3 {
		t.Errorf("Retry() made %d attempts after cancellation, should stop quickly", attempts)
	}
}

func TestRetryWithResult_Success(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  3,
		BaseDelay:    10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		JitterFactor: 0,
	}

	attempts := 0
	fn := func(ctx context.Context) (int, error) {
		attempts++
		if attempts < 3 {
			return 0, NewTransientError(errors.New("transient"), "retry")
		}
		return 42, nil
	}

	result, err := RetryWithResult(context.Background(), config, fn)
	if err != nil {
		t.Errorf("RetryWithResult() returned error: %v", err)
	}

	if result != 42 {
		t.Errorf("RetryWithResult() result = %d, want 42", result)
	}

	if attempts != 3 {
		t.Errorf("RetryWithResult() made %d attempts, want 3", attempts)
	}
}

func TestRetryWithResult_Failure(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  2,
		BaseDelay:    10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		JitterFactor: 0,
	}

	attempts := 0
	fn := func(ctx context.Context) (string, error) {
		attempts++
		return "", NewTransientError(errors.New("always fails"), "transient")
	}

	result, err := RetryWithResult(context.Background(), config, fn)
	if err == nil {
		t.Error("RetryWithResult() should have returned error")
	}

	if result != "" {
		t.Errorf("RetryWithResult() result = %q, want empty string", result)
	}

	expectedAttempts := config.MaxAttempts + 1
	if attempts != expectedAttempts {
		t.Errorf("RetryWithResult() made %d attempts, want %d", attempts, expectedAttempts)
	}
}

func TestCalculateBackoff(t *testing.T) {
	config := RetryConfig{
		BaseDelay:    1 * time.Second,
		MaxDelay:     30 * time.Second,
		JitterFactor: 0, // No jitter for deterministic testing
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{attempt: 0, expected: 1 * time.Second},   // 1s * 2^0 = 1s
		{attempt: 1, expected: 2 * time.Second},   // 1s * 2^1 = 2s
		{attempt: 2, expected: 4 * time.Second},   // 1s * 2^2 = 4s
		{attempt: 3, expected: 8 * time.Second},   // 1s * 2^3 = 8s
		{attempt: 4, expected: 16 * time.Second},  // 1s * 2^4 = 16s
		{attempt: 5, expected: 30 * time.Second},  // 1s * 2^5 = 32s, capped at 30s
		{attempt: 10, expected: 30 * time.Second}, // Always capped at max
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			delay := calculateBackoff(tt.attempt, config)
			if delay != tt.expected {
				t.Errorf("calculateBackoff(%d) = %v, want %v", tt.attempt, delay, tt.expected)
			}
		})
	}
}

func TestCalculateBackoff_WithJitter(t *testing.T) {
	config := RetryConfig{
		BaseDelay:    1 * time.Second,
		MaxDelay:     30 * time.Second,
		JitterFactor: 0.25, // ±25%
	}

	// Test that jitter keeps delay within acceptable range
	for attempt := 0; attempt < 5; attempt++ {
		delay := calculateBackoff(attempt, config)

		// Calculate expected base with proper type conversion
		multiplier := float64(int(1) << attempt)
		expectedBase := time.Duration(float64(config.BaseDelay) * multiplier)
		if expectedBase > config.MaxDelay {
			expectedBase = config.MaxDelay
		}

		// With jitter, delay should be within reasonable bounds
		if delay < 0 {
			t.Errorf("calculateBackoff(%d) with jitter = %v, should be positive", attempt, delay)
		}

		if delay > config.MaxDelay {
			t.Errorf("calculateBackoff(%d) with jitter = %v, exceeds MaxDelay %v", attempt, delay, config.MaxDelay)
		}

		// Delay should be within a reasonable range of expected (with jitter)
		// We can't test exact values with jitter, but we can test it's not zero or negative
		if delay == 0 {
			t.Errorf("calculateBackoff(%d) with jitter = 0, should have some delay", attempt)
		}
	}
}

func TestRetryWithStats(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  3,
		BaseDelay:    10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		JitterFactor: 0,
	}

	t.Run("success after retries", func(t *testing.T) {
		attempts := 0
		fn := func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return NewTransientError(errors.New("transient"), "retry")
			}
			return nil
		}

		stats, err := RetryWithStats(context.Background(), config, fn)
		if err != nil {
			t.Errorf("RetryWithStats() returned error: %v", err)
		}

		if stats.TotalAttempts != 3 {
			t.Errorf("stats.TotalAttempts = %d, want 3", stats.TotalAttempts)
		}

		if stats.SuccessfulRetries != 1 {
			t.Errorf("stats.SuccessfulRetries = %d, want 1", stats.SuccessfulRetries)
		}

		if stats.FailedRetries != 0 {
			t.Errorf("stats.FailedRetries = %d, want 0", stats.FailedRetries)
		}
	})

	t.Run("failure after retries", func(t *testing.T) {
		fn := func(ctx context.Context) error {
			return NewTransientError(errors.New("always fails"), "transient")
		}

		stats, err := RetryWithStats(context.Background(), config, fn)
		if err == nil {
			t.Error("RetryWithStats() should have returned error")
		}

		expectedAttempts := config.MaxAttempts + 1
		if stats.TotalAttempts != expectedAttempts {
			t.Errorf("stats.TotalAttempts = %d, want %d", stats.TotalAttempts, expectedAttempts)
		}

		if stats.FailedRetries != 1 {
			t.Errorf("stats.FailedRetries = %d, want 1", stats.FailedRetries)
		}
	})
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		attemptNumber int
		maxAttempts   int
		expected      bool
	}{
		{
			name:          "nil error",
			err:           nil,
			attemptNumber: 0,
			maxAttempts:   3,
			expected:      false,
		},
		{
			name:          "transient error, within limit",
			err:           NewTransientError(errors.New("test"), "transient"),
			attemptNumber: 1,
			maxAttempts:   3,
			expected:      true,
		},
		{
			name:          "transient error, at limit",
			err:           NewTransientError(errors.New("test"), "transient"),
			attemptNumber: 3,
			maxAttempts:   3,
			expected:      false,
		},
		{
			name:          "permanent error",
			err:           NewPermanentError(errors.New("test"), "permanent"),
			attemptNumber: 0,
			maxAttempts:   3,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldRetry(tt.err, tt.attemptNumber, tt.maxAttempts)
			if result != tt.expected {
				t.Errorf("ShouldRetry(%v, %d, %d) = %v, want %v",
					tt.err, tt.attemptNumber, tt.maxAttempts, result, tt.expected)
			}
		})
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxAttempts != 3 {
		t.Errorf("DefaultRetryConfig().MaxAttempts = %d, want 3", config.MaxAttempts)
	}

	if config.BaseDelay != 1*time.Second {
		t.Errorf("DefaultRetryConfig().BaseDelay = %v, want 1s", config.BaseDelay)
	}

	if config.MaxDelay != 30*time.Second {
		t.Errorf("DefaultRetryConfig().MaxDelay = %v, want 30s", config.MaxDelay)
	}

	if config.JitterFactor != 0.25 {
		t.Errorf("DefaultRetryConfig().JitterFactor = %f, want 0.25", config.JitterFactor)
	}
}

// Benchmark tests

func BenchmarkRetry_ImmediateSuccess(b *testing.B) {
	config := DefaultRetryConfig()
	fn := func(ctx context.Context) error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Retry(context.Background(), config, fn)
	}
}

func BenchmarkRetry_WithRetries(b *testing.B) {
	config := RetryConfig{
		MaxAttempts:  3,
		BaseDelay:    1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		JitterFactor: 0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attempts := 0
		fn := func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return NewTransientError(errors.New("transient"), "retry")
			}
			return nil
		}
		_ = Retry(context.Background(), config, fn)
	}
}
