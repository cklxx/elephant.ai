package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/ports"
	portsllm "alex/internal/agent/ports/llm"
	alexerrors "alex/internal/errors"
	"alex/internal/logging"
)

// retryClient wraps an LLM client with retry logic and circuit breaker
type retryClient struct {
	underlying     portsllm.LLMClient
	retryConfig    alexerrors.RetryConfig
	circuitBreaker *alexerrors.CircuitBreaker
	logger         logging.Logger
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
// Unlike Complete, streaming requests are not retried to avoid duplicating partial
// responses when an upstream error occurs mid-stream. We still leverage the circuit
// breaker for protection and fall back to non-streaming completion when the
// underlying client lacks native streaming support.
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

	resp, err := alexerrors.ExecuteFunc(c.circuitBreaker, ctx, func(ctx context.Context) (*ports.CompletionResponse, error) {
		response, streamErr := streamingClient.StreamComplete(ctx, req, callbacks)
		if streamErr != nil {
			return nil, c.classifyLLMError(streamErr)
		}
		return response, nil
	})

	duration := time.Since(startTime)

	if err != nil {
		if alexerrors.IsDegraded(err) {
			return nil, fmt.Errorf("%s", alexerrors.FormatForLLM(err))
		}
		formattedErr := c.formatStreamingError(err, duration)
		return nil, fmt.Errorf("%s", formattedErr)
	}

	if duration > 5*time.Second {
		c.logger.Debug("LLM streaming request succeeded after %v", duration)
	}

	return resp, nil
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

// WrapWithRetry wraps an existing LLM client with retry logic using provided configuration
func WrapWithRetry(client portsllm.LLMClient, retryConfig alexerrors.RetryConfig, circuitBreakerConfig alexerrors.CircuitBreakerConfig) portsllm.LLMClient {
	// Create circuit breaker for this client
	circuitBreaker := alexerrors.NewCircuitBreaker(
		fmt.Sprintf("llm-%s", client.Model()),
		circuitBreakerConfig,
	)

	return NewRetryClient(client, retryConfig, circuitBreaker)
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
