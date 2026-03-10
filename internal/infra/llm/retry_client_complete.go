package llm

import (
	"context"
	"fmt"
	"time"

	"alex/internal/domain/agent/ports"
	alexerrors "alex/internal/shared/errors"
)

// Complete executes LLM completion with retry logic
func (c *retryClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	startTime := time.Now()

	resp, err := c.completeWithRetry(ctx, req)

	duration := time.Since(startTime)

	if err != nil {
		c.recordHealthError(err)
		c.logger.Warn("LLM request failed after retries (took %v): %v", duration, err)
		c.logLLMCallSummary(ctx, "complete", req, duration, nil, err)

		// Check if it's a degraded error (circuit breaker open)
		if alexerrors.IsDegraded(err) {
			// Return formatted error for LLM
			return nil, fmt.Errorf("%s", alexerrors.FormatForLLM(err))
		}

		// Format error for LLM with retry context
		formattedErr := c.formatRetryError(err, duration)
		return nil, fmt.Errorf("%s", formattedErr)
	}

	c.recordHealthLatency(duration)
	c.logLLMCallSummary(ctx, "complete", req, duration, resp, nil)

	if duration > 5*time.Second {
		c.logger.Debug("LLM request succeeded after %v", duration)
	}

	return resp, nil
}

func (c *retryClient) completeWithRetry(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	maxAttempts := c.retryConfig.MaxAttempts + 1
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			c.logger.Debug("Context cancelled, stopping retries")
			return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		// Check the rate-limit circuit before each attempt.
		if err := c.checkRateLimitCircuit(); err != nil {
			return nil, err
		}

		if attempt == 0 {
			c.logger.Debug("Executing (attempt 1/%d)", maxAttempts)
		} else {
			c.logger.Debug("Retrying (attempt %d/%d)", attempt+1, maxAttempts)
		}

		resp, err := alexerrors.ExecuteFunc(c.circuitBreaker, ctx, func(ctx context.Context) (*ports.CompletionResponse, error) {
			response, callErr := c.underlying.Complete(ctx, req)
			if callErr != nil {
				return nil, c.classifyLLMError(callErr)
			}
			return response, nil
		})

		c.recordRateLimitOutcome(err)

		if err == nil {
			if attempt > 0 {
				c.logger.Info("Retry succeeded after %d attempts", attempt+1)
			}
			return resp, nil
		}

		lastErr = err
		c.logger.Debug("Attempt %d failed: %v", attempt+1, err)

		if !alexerrors.IsTransient(err) {
			c.logger.Debug("Error is not transient, stopping retries")
			return nil, err
		}

		if attempt == maxAttempts-1 {
			c.logger.Warn("Max retries (%d) exhausted", maxAttempts)
			break
		}

		delay := c.retryDelay(attempt, err)
		c.logger.Debug("Waiting %v before next retry", delay)
		if err := c.waitForRetry(ctx, delay); err != nil {
			return nil, err
		}
	}

	// All retries exhausted — try fallback if available and error is transient.
	if c.fallbackClientFn != nil && alexerrors.IsTransient(lastErr) {
		return c.tryFallbackComplete(ctx, req, lastErr)
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// tryFallbackComplete attempts a single Complete call on the fallback client.
func (c *retryClient) tryFallbackComplete(ctx context.Context, req ports.CompletionRequest, originalErr error) (*ports.CompletionResponse, error) {
	c.logger.Warn("[FALLBACK] Primary %s/%s exhausted retries (transient); attempting fallback to %s/%s: %v",
		c.provider, c.model, c.fallbackProvider, c.fallbackModel, originalErr)

	fallbackClient, err := c.fallbackClientFn()
	if err != nil {
		c.logger.Warn("[FALLBACK] Failed to create fallback client %s/%s: %v", c.fallbackProvider, c.fallbackModel, err)
		return nil, fmt.Errorf("max retries exceeded (fallback unavailable): %w", originalErr)
	}

	resp, err := fallbackClient.Complete(ctx, req)
	if err != nil {
		c.logger.Warn("[FALLBACK] Fallback %s/%s also failed: %v", c.fallbackProvider, c.fallbackModel, err)
		return nil, fmt.Errorf("max retries exceeded (fallback %s/%s also failed): %w", c.fallbackProvider, c.fallbackModel, originalErr)
	}

	c.logger.Info("[FALLBACK] Successfully completed via fallback %s/%s (primary %s/%s was unavailable)",
		c.fallbackProvider, c.fallbackModel, c.provider, c.model)
	return resp, nil
}
