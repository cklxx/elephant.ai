package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	alexerrors "alex/internal/shared/errors"
	"alex/internal/shared/logging"
)

// retryClient wraps an LLM client with retry logic and circuit breaker
type retryClient struct {
	underlying     portsllm.LLMClient
	retryConfig    alexerrors.RetryConfig
	circuitBreaker *alexerrors.CircuitBreaker
	logger         logging.Logger
	healthRegistry *HealthRegistry
	provider       string
	model          string
}

var _ portsllm.StreamingLLMClient = (*retryClient)(nil)

// NewRetryClient wraps an LLM client with retry and circuit breaker logic
func NewRetryClient(client portsllm.LLMClient, retryConfig alexerrors.RetryConfig, circuitBreaker *alexerrors.CircuitBreaker) portsllm.LLMClient {
	return &retryClient{
		underlying:     client,
		retryConfig:    retryConfig,
		circuitBreaker: circuitBreaker,
		logger:         logging.NewComponentLogger("llm-retry"),
	}
}

// newRetryClientWithHealth wraps an LLM client with retry, circuit breaker, and health recording.
func newRetryClientWithHealth(client portsllm.LLMClient, retryConfig alexerrors.RetryConfig, circuitBreaker *alexerrors.CircuitBreaker, hr *HealthRegistry, provider, model string) portsllm.LLMClient {
	return &retryClient{
		underlying:     client,
		retryConfig:    retryConfig,
		circuitBreaker: circuitBreaker,
		logger:         logging.NewComponentLogger("llm-retry"),
		healthRegistry: hr,
		provider:       provider,
		model:          model,
	}
}

// Complete executes LLM completion with retry logic
func (c *retryClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	startTime := time.Now()

	// Execute with circuit breaker and retry
	resp, err := alexerrors.RetryWithResultAndLog(ctx, c.retryConfig, func(ctx context.Context) (*ports.CompletionResponse, error) {
		// Use circuit breaker to protect against cascading failures
		return alexerrors.ExecuteFunc(c.circuitBreaker, ctx, func(ctx context.Context) (*ports.CompletionResponse, error) {
			response, err := c.underlying.Complete(ctx, req)
			if err != nil {
				// Classify and wrap error for better retry decisions
				return nil, c.classifyLLMError(err)
			}
			return response, nil
		})
	}, c.logger)

	duration := time.Since(startTime)

	if err != nil {
		c.recordHealthError(err)
		c.logger.Warn("LLM request failed after retries (took %v): %v", duration, err)

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

	if duration > 5*time.Second {
		c.logger.Debug("LLM request succeeded after %v", duration)
	}

	return resp, nil
}

// Model returns the underlying model name
func (c *retryClient) Model() string {
	return c.underlying.Model()
}

// SetUsageCallback sets the usage callback for cost tracking
func (c *retryClient) SetUsageCallback(callback func(usage ports.TokenUsage, model string, provider string)) {
	if trackingClient, ok := c.underlying.(portsllm.UsageTrackingClient); ok {
		trackingClient.SetUsageCallback(callback)
	}
}

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

	// Retry loop for pre-stream errors (e.g., rate limits).
	// These errors occur before any streaming data is sent, so they're safe to retry.
	maxAttempts := c.retryConfig.MaxAttempts + 1
	var resp *ports.CompletionResponse
	var err error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		resp, err = alexerrors.ExecuteFunc(c.circuitBreaker, ctx, func(ctx context.Context) (*ports.CompletionResponse, error) {
			response, streamErr := streamingClient.StreamComplete(ctx, req, callbacks)
			if streamErr != nil {
				return nil, c.classifyLLMError(streamErr)
			}
			return response, nil
		})

		if err == nil {
			break
		}

		// Only retry on rate limit errors (pre-stream errors).
		if !c.isRateLimitError(err) {
			break
		}

		if attempt < maxAttempts-1 {
			delay := c.calculateBackoff(attempt)
			c.logger.Debug("Rate limited, retrying in %v (attempt %d/%d)", delay, attempt+1, maxAttempts)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	duration := time.Since(startTime)

	if err != nil {
		c.recordHealthError(err)
		if alexerrors.IsDegraded(err) {
			return nil, fmt.Errorf("%s", alexerrors.FormatForLLM(err))
		}
		formattedErr := c.formatStreamingError(err, duration)
		return nil, fmt.Errorf("%s", formattedErr)
	}

	c.recordHealthLatency(duration)

	if duration > 5*time.Second {
		c.logger.Debug("LLM streaming request succeeded after %v", duration)
	}

	return resp, nil
}

// isRateLimitError checks if the error is a rate limit (429) error.
func (c *retryClient) isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit")
}

// calculateBackoff returns the delay for the given retry attempt using exponential backoff with jitter.
func (c *retryClient) calculateBackoff(attempt int) time.Duration {
	baseDelay := c.retryConfig.BaseDelay
	if baseDelay == 0 {
		baseDelay = time.Second
	}
	maxDelay := c.retryConfig.MaxDelay
	if maxDelay == 0 {
		maxDelay = 30 * time.Second
	}
	jitter := c.retryConfig.JitterFactor
	if jitter == 0 {
		jitter = 0.25
	}

	// Exponential backoff: baseDelay * 2^attempt
	delay := float64(baseDelay) * float64(int(1)<<attempt)
	if delay > float64(maxDelay) {
		delay = float64(maxDelay)
	}

	// Add jitter: ±jitter%
	jitterRange := delay * jitter
	delay = delay - jitterRange + (2 * jitterRange * float64(time.Now().UnixNano()%1000) / 1000)

	return time.Duration(delay)
}

func (c *retryClient) streamingClient() portsllm.StreamingLLMClient {
	if streaming, ok := c.underlying.(portsllm.StreamingLLMClient); ok {
		return streaming
	}
	if adapted, ok := EnsureStreamingClient(c.underlying).(portsllm.StreamingLLMClient); ok {
		return adapted
	}
	return nil
}

// classifyLLMError detects transient errors from LLM API
func (c *retryClient) classifyLLMError(err error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()
	lowerErr := strings.ToLower(errStr)

	// Rate limit errors (429)
	if strings.Contains(lowerErr, "429") || strings.Contains(lowerErr, "rate limit") {
		return alexerrors.NewTransientError(err,
			"API rate limit reached. Retrying with exponential backoff.")
	}

	// Server errors (500, 502, 503, 504)
	if strings.Contains(lowerErr, "500") || strings.Contains(lowerErr, "internal server error") {
		return alexerrors.NewTransientError(err,
			"Server error (500). Retrying request.")
	}

	if strings.Contains(lowerErr, "502") || strings.Contains(lowerErr, "bad gateway") {
		return alexerrors.NewTransientError(err,
			"Bad gateway (502). Retrying request.")
	}

	if strings.Contains(lowerErr, "503") || strings.Contains(lowerErr, "service unavailable") {
		return alexerrors.NewTransientError(err,
			"Service unavailable (503). Retrying request.")
	}

	if strings.Contains(lowerErr, "504") || strings.Contains(lowerErr, "gateway timeout") {
		return alexerrors.NewTransientError(err,
			"Gateway timeout (504). Retrying request.")
	}

	// Network errors
	if strings.Contains(lowerErr, "connection refused") {
		return alexerrors.NewTransientError(err,
			alexerrors.FormatForLLM(err))
	}

	if strings.Contains(lowerErr, "timeout") || strings.Contains(lowerErr, "deadline exceeded") {
		return alexerrors.NewTransientError(err,
			"Request timed out. Retrying with backoff.")
	}

	if strings.Contains(lowerErr, "network") || strings.Contains(lowerErr, "dns") {
		return alexerrors.NewTransientError(err,
			"Network connectivity issue. Retrying request.")
	}

	// Connection reset, broken pipe
	if strings.Contains(lowerErr, "connection reset") || strings.Contains(lowerErr, "broken pipe") {
		return alexerrors.NewTransientError(err,
			"Connection reset. Retrying request.")
	}

	// Permanent errors
	if strings.Contains(lowerErr, "401") || strings.Contains(lowerErr, "unauthorized") {
		return alexerrors.NewPermanentError(err,
			"Authentication failed. Please check your API key configuration.")
	}

	if strings.Contains(lowerErr, "403") || strings.Contains(lowerErr, "forbidden") {
		return alexerrors.NewPermanentError(err,
			"Permission denied. You don't have access to this model or resource.")
	}

	if strings.Contains(lowerErr, "404") || strings.Contains(lowerErr, "not found") {
		return alexerrors.NewPermanentError(err,
			"Model or endpoint not found. Please verify the model name.")
	}

	if strings.Contains(lowerErr, "400") || strings.Contains(lowerErr, "bad request") {
		return alexerrors.NewPermanentError(err,
			"Invalid request. Please check the parameters.")
	}

	// Default: return as-is (will be classified by IsTransient)
	return err
}

// formatRetryError formats error message with retry context
func (c *retryClient) formatRetryError(err error, duration time.Duration) string {
	// Get formatted LLM message
	llmMessage := alexerrors.FormatForLLM(err)

	// Add retry context
	attempts := c.retryConfig.MaxAttempts + 1
	return fmt.Sprintf("%s Retried %d times over %v.",
		llmMessage, attempts, duration.Round(time.Second))
}

func (c *retryClient) formatStreamingError(err error, duration time.Duration) string {
	llmMessage := alexerrors.FormatForLLM(err)
	return fmt.Sprintf("%s Streaming request failed after %v.", llmMessage, duration.Round(time.Second))
}

// recordHealthLatency records a successful call latency if a HealthRegistry is attached.
func (c *retryClient) recordHealthLatency(d time.Duration) {
	if c.healthRegistry != nil {
		c.healthRegistry.RecordLatency(c.provider, c.model, d)
	}
}

// recordHealthError records a failed call if a HealthRegistry is attached.
func (c *retryClient) recordHealthError(err error) {
	if c.healthRegistry != nil {
		c.healthRegistry.RecordError(c.provider, c.model, err)
	}
}

// WrapWithRetry wraps an existing LLM client with retry logic using provided configuration
func WrapWithRetry(client portsllm.LLMClient, retryConfig alexerrors.RetryConfig, circuitBreakerConfig alexerrors.CircuitBreakerConfig) portsllm.LLMClient {
	// Create circuit breaker for this client
	circuitBreaker := alexerrors.NewCircuitBreaker(
		fmt.Sprintf("llm-%s", client.Model()),
		circuitBreakerConfig,
	)

	return NewRetryClient(client, retryConfig, circuitBreaker)
}

// WrapWithRetryAndHealth wraps an LLM client with retry, circuit breaker, and health tracking.
// The circuit breaker is automatically registered with the HealthRegistry.
func WrapWithRetryAndHealth(client portsllm.LLMClient, retryConfig alexerrors.RetryConfig, circuitBreakerConfig alexerrors.CircuitBreakerConfig, hr *HealthRegistry, provider, model string) portsllm.LLMClient {
	circuitBreaker := alexerrors.NewCircuitBreaker(
		fmt.Sprintf("llm-%s", client.Model()),
		circuitBreakerConfig,
	)

	if hr != nil {
		hr.Register(provider, model, circuitBreaker)
	}

	return newRetryClientWithHealth(client, retryConfig, circuitBreaker, hr, provider, model)
}

// HTTPStatusError represents an HTTP error with status code
type HTTPStatusError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Status)
}

// NewHTTPStatusError creates an HTTP status error
func NewHTTPStatusError(statusCode int, status, body string) error {
	return &HTTPStatusError{
		StatusCode: statusCode,
		Status:     status,
		Body:       body,
	}
}

// IsHTTPStatusError checks if error is an HTTP status error
func IsHTTPStatusError(err error, statusCode int) bool {
	var httpErr *HTTPStatusError
	if !strings.Contains(err.Error(), fmt.Sprintf("%d", statusCode)) {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "http") ||
		strings.Contains(strings.ToLower(err.Error()), "api error") ||
		httpErr != nil
}
