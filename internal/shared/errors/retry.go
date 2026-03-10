package errors

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"alex/internal/shared/logging"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxAttempts  int           // Maximum number of retry attempts (default: 3)
	BaseDelay    time.Duration // Base delay for exponential backoff (default: 1s)
	MaxDelay     time.Duration // Maximum delay between retries (default: 30s)
	JitterFactor float64       // Jitter factor for randomization (default: 0.25 = ±25%)
}

// DefaultRetryConfig returns sensible defaults
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		BaseDelay:    1 * time.Second,
		MaxDelay:     30 * time.Second,
		JitterFactor: 0.25,
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc func(ctx context.Context) error

// Retry executes a function with exponential backoff retry logic
func Retry(ctx context.Context, config RetryConfig, fn RetryableFunc) error {
	return RetryWithLog(ctx, config, fn, nil)
}

// RetryWithLog executes a function with retry logic and custom logger
func RetryWithLog(ctx context.Context, config RetryConfig, fn RetryableFunc, logger logging.Logger) error {
	if logging.IsNil(logger) {
		logger = logging.NewComponentLogger("retry")
	}

	var lastErr error

	for attempt := 0; attempt <= config.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			logger.Debug("Context cancelled, stopping retries")
			return ctx.Err()
		default:
		}

		// Execute function
		if attempt == 0 {
			logger.Debug("Executing (attempt 1/%d)", config.MaxAttempts+1)
		} else {
			logger.Debug("Retrying (attempt %d/%d)", attempt+1, config.MaxAttempts+1)
		}

		err := fn(ctx)

		// Success
		if err == nil {
			if attempt > 0 {
				logger.Info("Retry succeeded after %d attempts", attempt+1)
			}
			return nil
		}

		lastErr = err
		logger.Debug("Attempt %d failed: %v", attempt+1, err)

		// Check if error is retryable
		if !IsTransient(err) {
			logger.Debug("Error is not transient, stopping retries")
			return err
		}

		// Don't sleep after last attempt
		if attempt == config.MaxAttempts {
			logger.Warn("Max retries (%d) exhausted", config.MaxAttempts+1)
			break
		}

		// Calculate backoff delay
		delay := calculateBackoff(attempt, config)
		logger.Debug("Waiting %v before next retry", delay)

		// Wait with context cancellation support
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			logger.Debug("Context cancelled during backoff")
			return ctx.Err()
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// RetryWithResult executes a function that returns a result with retry logic
func RetryWithResult[T any](ctx context.Context, config RetryConfig, fn func(ctx context.Context) (T, error)) (T, error) {
	return RetryWithResultAndLog[T](ctx, config, fn, nil)
}

// RetryWithResultAndLog executes a function that returns a result with retry logic and custom logger
func RetryWithResultAndLog[T any](ctx context.Context, config RetryConfig, fn func(ctx context.Context) (T, error), logger logging.Logger) (T, error) {
	if logging.IsNil(logger) {
		logger = logging.NewComponentLogger("retry")
	}

	var lastErr error
	var zeroValue T

	for attempt := 0; attempt <= config.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			logger.Debug("Context cancelled, stopping retries")
			return zeroValue, ctx.Err()
		default:
		}

		// Execute function
		if attempt == 0 {
			logger.Debug("Executing (attempt 1/%d)", config.MaxAttempts+1)
		} else {
			logger.Debug("Retrying (attempt %d/%d)", attempt+1, config.MaxAttempts+1)
		}

		result, err := fn(ctx)

		// Success
		if err == nil {
			if attempt > 0 {
				logger.Info("Retry succeeded after %d attempts", attempt+1)
			}
			return result, nil
		}

		lastErr = err
		logger.Debug("Attempt %d failed: %v", attempt+1, err)

		// Check if error is retryable
		if !IsTransient(err) {
			logger.Debug("Error is not transient, stopping retries")
			return zeroValue, err
		}

		// Don't sleep after last attempt
		if attempt == config.MaxAttempts {
			logger.Warn("Max retries (%d) exhausted", config.MaxAttempts+1)
			break
		}

		// Calculate backoff delay
		delay := calculateBackoff(attempt, config)
		logger.Debug("Waiting %v before next retry", delay)

		// Wait with context cancellation support
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			logger.Debug("Context cancelled during backoff")
			return zeroValue, ctx.Err()
		}
	}

	return zeroValue, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// calculateBackoff calculates exponential backoff with jitter
func calculateBackoff(attempt int, config RetryConfig) time.Duration {
	// Exponential backoff: baseDelay * 2^attempt
	// attempt 0 -> 1s (2^0 = 1)
	// attempt 1 -> 2s (2^1 = 2)
	// attempt 2 -> 4s (2^2 = 4)
	// attempt 3 -> 8s (2^3 = 8)
	multiplier := math.Pow(2, float64(attempt))
	delay := time.Duration(float64(config.BaseDelay) * multiplier)

	// Cap at max delay
	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	// Add jitter: ±25% randomization
	if config.JitterFactor > 0 {
		jitter := float64(delay) * config.JitterFactor
		// Random value in range [-jitter, +jitter]
		jitterAmount := (rand.Float64()*2 - 1) * jitter
		delay = time.Duration(float64(delay) + jitterAmount)

		// Ensure delay is positive and doesn't exceed max
		if delay < 0 {
			delay = config.BaseDelay
		}
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return delay
}
