package llm

import (
	"context"
	"fmt"
	"time"

	"alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	alexerrors "alex/internal/shared/errors"
)

// StreamComplete proxies streaming requests to the underlying client when supported.
// Unlike Complete, streaming requests are not fully retried to avoid duplicating partial
// responses when an upstream error occurs mid-stream. However, pre-stream errors like
// rate limits (429) are retried since they occur before any data is sent.
// We still leverage the circuit breaker for protection and fall back to non-streaming
// completion when the underlying client lacks native streaming support.
func (c *retryClient) StreamComplete(
	ctx context.Context,
	req ports.CompletionRequest,
	callbacks ports.CompletionStreamCallbacks,
) (*ports.CompletionResponse, error) {
	streamingClient := c.streamingClient()
	if streamingClient == nil {
		// Preserve legacy fallback but avoid logging noisy warnings.
		resp, err := c.Complete(ctx, req)
		if err != nil {
			return nil, err
		}
		if callbacks.OnContentDelta != nil {
			if resp.Content != "" {
				callbacks.OnContentDelta(ports.ContentDelta{Delta: resp.Content})
			}
			callbacks.OnContentDelta(ports.ContentDelta{Final: true})
		}
		return resp, nil
	}

	startTime := time.Now()

	// Retry loop for transient stream failures before output is emitted.
	// These errors are only safe to retry before any streamed content has been emitted.
	maxAttempts := c.retryConfig.MaxAttempts + 1
	var resp *ports.CompletionResponse
	var err error
	observedStreamOutput := false

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Check the rate-limit circuit before each streaming attempt.
		if rlErr := c.checkRateLimitCircuit(); rlErr != nil {
			err = rlErr
			break
		}

		observedStreamOutput = false
		attemptCallbacks := callbacks
		if callbacks.OnContentDelta != nil {
			original := callbacks.OnContentDelta
			attemptCallbacks.OnContentDelta = func(delta ports.ContentDelta) {
				if delta.Delta != "" || delta.Final {
					observedStreamOutput = true
				}
				original(delta)
			}
		}

		resp, err = alexerrors.ExecuteFunc(c.circuitBreaker, ctx, func(ctx context.Context) (*ports.CompletionResponse, error) {
			response, streamErr := streamingClient.StreamComplete(ctx, req, attemptCallbacks)
			if streamErr != nil {
				return nil, c.classifyLLMError(streamErr)
			}
			return response, nil
		})

		c.recordRateLimitOutcome(err)

		if err == nil {
			break
		}

		// Only retry transient transport issues that occur before output is sent.
		if !alexerrors.IsTransient(err) || observedStreamOutput {
			break
		}

		if attempt < maxAttempts-1 {
			delay := c.retryDelay(attempt, err)
			c.logger.Debug("Streaming request failed, retrying in %v (attempt %d/%d): %v", delay, attempt+1, maxAttempts, err)
			if err := c.waitForRetry(ctx, delay); err != nil {
				return nil, err
			}
		}
	}

	// All retries exhausted — try streaming fallback if available, transient, and no output was emitted.
	if err != nil && c.fallbackClientFn != nil && alexerrors.IsTransient(err) && !observedStreamOutput {
		fbResp, fbErr := c.tryFallbackStreamComplete(ctx, req, callbacks, err)
		if fbErr == nil {
			return fbResp, nil
		}
		// Fallback also failed — continue to normal error reporting with the original error.
	}

	duration := time.Since(startTime)

	if err != nil {
		c.recordHealthError(err)
		c.logLLMCallSummary(ctx, "stream", req, duration, nil, err)
		if alexerrors.IsDegraded(err) {
			return nil, fmt.Errorf("%s", alexerrors.FormatForLLM(err))
		}
		formattedErr := c.formatStreamingError(err, duration)
		if alexerrors.IsTransient(err) {
			return nil, alexerrors.NewTransientError(err, formattedErr)
		}
		return nil, fmt.Errorf("%s", formattedErr)
	}

	c.recordHealthLatency(duration)
	c.logLLMCallSummary(ctx, "stream", req, duration, resp, nil)

	if duration > 5*time.Second {
		c.logger.Debug("LLM streaming request succeeded after %v", duration)
	}

	return resp, nil
}

// tryFallbackStreamComplete attempts a single StreamComplete call on the fallback client.
func (c *retryClient) tryFallbackStreamComplete(
	ctx context.Context,
	req ports.CompletionRequest,
	callbacks ports.CompletionStreamCallbacks,
	originalErr error,
) (*ports.CompletionResponse, error) {
	c.logger.Warn("[FALLBACK] Primary %s/%s exhausted streaming retries (transient); attempting fallback to %s/%s: %v",
		c.provider, c.model, c.fallbackProvider, c.fallbackModel, originalErr)

	fallbackClient, err := c.fallbackClientFn()
	if err != nil {
		c.logger.Warn("[FALLBACK] Failed to create fallback client %s/%s: %v", c.fallbackProvider, c.fallbackModel, err)
		return nil, err
	}

	streamingFB, ok := EnsureStreamingClient(fallbackClient).(portsllm.StreamingLLMClient)
	if !ok {
		c.logger.Warn("[FALLBACK] Fallback client %s/%s does not support streaming", c.fallbackProvider, c.fallbackModel)
		return nil, fmt.Errorf("fallback client does not support streaming")
	}

	resp, err := streamingFB.StreamComplete(ctx, req, callbacks)
	if err != nil {
		c.logger.Warn("[FALLBACK] Fallback streaming %s/%s also failed: %v", c.fallbackProvider, c.fallbackModel, err)
		return nil, err
	}

	c.logger.Info("[FALLBACK] Successfully completed streaming via fallback %s/%s (primary %s/%s was unavailable)",
		c.fallbackProvider, c.fallbackModel, c.provider, c.model)
	return resp, nil
}
