package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	"alex/internal/infra/backoff"
	alexerrors "alex/internal/shared/errors"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
	id "alex/internal/shared/utils/id"
)

// tokenRefresher is called when a 401 is received for an OAuth token to
// obtain a fresh access token. Implementations should handle refresh-token
// flows and credential persistence.
type tokenRefresher func() (newToken string, err error)

// retryClient wraps an LLM client with retry logic and circuit breaker
type retryClient struct {
	underlying     portsllm.LLMClient
	retryConfig    alexerrors.RetryConfig
	circuitBreaker *alexerrors.CircuitBreaker
	logger         logging.Logger
	llmLogger      logging.Logger // writes to alex-llm.log
	healthRegistry *healthRegistry
	provider       string
	model          string
	sleepFn        func(context.Context, time.Duration) error
	authRefresher  tokenRefresher
	authRefreshMu  sync.Mutex
	lastAuthRefresh time.Time
}

var _ portsllm.StreamingLLMClient = (*retryClient)(nil)

// NewRetryClient wraps an LLM client with retry and circuit breaker logic
func NewRetryClient(client portsllm.LLMClient, retryConfig alexerrors.RetryConfig, circuitBreaker *alexerrors.CircuitBreaker) portsllm.LLMClient {
	return &retryClient{
		underlying:     client,
		retryConfig:    retryConfig,
		circuitBreaker: circuitBreaker,
		logger:         logging.NewComponentLogger("llm-retry"),
		llmLogger:      logging.NewLLMLogger("llm-retry"),
	}
}

// newRetryClientWithHealth wraps an LLM client with retry, circuit breaker, and health recording.
func newRetryClientWithHealth(client portsllm.LLMClient, retryConfig alexerrors.RetryConfig, circuitBreaker *alexerrors.CircuitBreaker, hr *healthRegistry, provider, model string) portsllm.LLMClient {
	return &retryClient{
		underlying:     client,
		retryConfig:    retryConfig,
		circuitBreaker: circuitBreaker,
		logger:         logging.NewComponentLogger("llm-retry"),
		llmLogger:      logging.NewLLMLogger("llm-retry"),
		healthRegistry: hr,
		provider:       provider,
		model:          model,
	}
}

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

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
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

	// Retry loop for transient stream failures before output is emitted.
	// These errors are only safe to retry before any streamed content has been emitted.
	maxAttempts := c.retryConfig.MaxAttempts + 1
	var resp *ports.CompletionResponse
	var err error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		observedStreamOutput := false
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

	delay := backoff.ExponentialClamp(baseDelay, maxDelay, attempt)
	return backoff.ScaleJitter(delay, jitter, float64(time.Now().UnixNano()%1000)/1000)
}

func (c *retryClient) retryDelay(attempt int, err error) time.Duration {
	if retryAfter := retryAfterDuration(err); retryAfter > 0 {
		maxDelay := c.retryConfig.MaxDelay
		if maxDelay > 0 && retryAfter > maxDelay {
			return maxDelay
		}
		return retryAfter
	}
	return c.calculateBackoff(attempt)
}

func (c *retryClient) waitForRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	if c.sleepFn != nil {
		if err := c.sleepFn(ctx, delay); err != nil {
			return err
		}
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		c.logger.Debug("Context cancelled during backoff")
		return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
	}
}

func retryAfterDuration(err error) time.Duration {
	var transientErr *alexerrors.TransientError
	if errors.As(err, &transientErr) && transientErr.RetryAfter > 0 {
		return time.Duration(transientErr.RetryAfter) * time.Second
	}
	return 0
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

type errorClassificationRule struct {
	patterns  []string
	permanent bool
	message   string
}

var llmErrorClassificationRules = []errorClassificationRule{
	// Rate limit
	{patterns: []string{"429", "rate limit"}, message: "API rate limit reached. Retrying with exponential backoff."},
	// Server errors
	{patterns: []string{"500", "internal server error"}, message: "Server error (500). Retrying request."},
	{patterns: []string{"502", "bad gateway"}, message: "Bad gateway (502). Retrying request."},
	{patterns: []string{"503", "service unavailable"}, message: "Service unavailable (503). Retrying request."},
	{patterns: []string{"504", "gateway timeout"}, message: "Gateway timeout (504). Retrying request."},
	// Network / transport errors
	{patterns: []string{"stream error", "received from peer", "internal_error", "read stream"}, message: "Streaming transport was interrupted. Retrying request."},
	{patterns: []string{"connection refused"}, message: "Connection refused. Retrying request."},
	{patterns: []string{"timeout", "deadline exceeded"}, message: "Request timed out. Retrying with backoff."},
	{patterns: []string{"network", "dns"}, message: "Network connectivity issue. Retrying request."},
	{patterns: []string{"connection reset", "broken pipe"}, message: "Connection reset. Retrying request."},
	// Permanent errors
	{patterns: []string{"401", "unauthorized"}, permanent: true, message: "Authentication failed. Please check your API key configuration."},
	{patterns: []string{"403", "forbidden"}, permanent: true, message: "Permission denied. You don't have access to this model or resource."},
	{patterns: []string{"404", "not found"}, permanent: true, message: "Model or endpoint not found. Please verify the model name."},
	{patterns: []string{"400", "bad request"}, permanent: true, message: "Invalid request. Please check the parameters."},
}

// classifyLLMError detects transient errors from LLM API
func (c *retryClient) classifyLLMError(err error) error {
	if err == nil {
		return nil
	}

	var transientErr *alexerrors.TransientError
	if errors.As(err, &transientErr) {
		return err
	}

	var permanentErr *alexerrors.PermanentError
	if errors.As(err, &permanentErr) {
		return err
	}

	var degradedErr *alexerrors.DegradedError
	if errors.As(err, &degradedErr) {
		return err
	}

	lowerErr := strings.ToLower(err.Error())

	for _, rule := range llmErrorClassificationRules {
		for _, pattern := range rule.patterns {
			if strings.Contains(lowerErr, pattern) {
				// On 401/unauthorized with an auth refresher, attempt token refresh
				// before classifying as permanent.
				if rule.permanent && (pattern == "401" || pattern == "unauthorized") {
					if refreshed := c.tryAuthRefresh(); refreshed {
						return alexerrors.NewTransientError(err, "Authentication refreshed. Retrying request.")
					}
				}
				if rule.permanent {
					return alexerrors.NewPermanentError(err, rule.message)
				}
				return alexerrors.NewTransientError(err, rule.message)
			}
		}
	}

	// Default: return as-is (will be classified by IsTransient)
	return err
}

// tryAuthRefresh attempts to refresh the OAuth token when a 401 is received.
// Returns true if the token was successfully refreshed and the underlying
// client's API key was updated. Uses a mutex and cooldown to prevent
// thundering-herd refreshes from concurrent requests.
func (c *retryClient) tryAuthRefresh() bool {
	if c.authRefresher == nil {
		return false
	}

	c.authRefreshMu.Lock()
	defer c.authRefreshMu.Unlock()

	// Cooldown: skip if refreshed within the last 30 seconds.
	if !c.lastAuthRefresh.IsZero() && time.Since(c.lastAuthRefresh) < 30*time.Second {
		// Another goroutine already refreshed recently; the underlying client's
		// API key is already updated — return true so the caller retries.
		return true
	}

	newToken, err := c.authRefresher()
	if err != nil {
		c.logger.Warn("Auth token refresh failed: %v", err)
		return false
	}
	if newToken == "" {
		return false
	}

	// Update the underlying client's API key.
	if updatable, ok := c.underlying.(apiKeyUpdatable); ok {
		updatable.SetAPIKey(newToken)
		c.lastAuthRefresh = time.Now()
		c.logger.Info("Auth token refreshed successfully, updated API key")
		return true
	}

	c.logger.Warn("Auth token refreshed but underlying client does not support SetAPIKey")
	return false
}

// formatRetryError formats error message with retry context
func (c *retryClient) formatRetryError(err error, duration time.Duration) string {
	// Get formatted LLM message
	llmMessage := alexerrors.FormatForLLM(err)

	// Add retry context
	attempts := c.retryConfig.MaxAttempts + 1
	return fmt.Sprintf("[%s/%s] %s Retried %d times over %v.",
		c.provider, c.underlying.Model(), llmMessage, attempts, duration.Round(time.Second))
}

func (c *retryClient) formatStreamingError(err error, duration time.Duration) string {
	llmMessage := alexerrors.FormatForLLM(err)
	return fmt.Sprintf("[%s/%s] %s Streaming request failed after %v.", c.provider, c.underlying.Model(), llmMessage, duration.Round(time.Second))
}

// logLLMCallSummary writes a structured summary to the LLM log (alex-llm.log) for both
// successful and failed calls. This ensures failures are always visible in the LLM log
// alongside the debug-level request/response details logged by the base client.
func (c *retryClient) logLLMCallSummary(ctx context.Context, mode string, req ports.CompletionRequest, latency time.Duration, resp *ports.CompletionResponse, err error) {
	model := normalizeLogField(c.underlying.Model())
	provider := normalizeLogField(c.provider)
	requestID := c.resolveRequestID(ctx, req, resp)
	intent := extractRequestIntent(req.Metadata)

	if err != nil {
		c.llmLogger.Warn("=== LLM %s FAILED === provider=%s model=%s request_id=%s intent=%s latency=%v error_class=%s error=%v",
			strings.ToUpper(mode),
			provider,
			model,
			normalizeLogField(requestID),
			normalizeLogField(intent),
			latency.Round(time.Millisecond),
			classifyFailureError(err),
			err,
		)
		c.logFailureRequestEntry(requestID, mode, provider, model, intent, latency, err)
		return
	}
	if resp == nil {
		return
	}
	c.llmLogger.Info("=== LLM %s OK === provider=%s model=%s request_id=%s intent=%s latency=%v tokens=%d+%d=%d stop=%s",
		strings.ToUpper(mode),
		provider,
		model,
		normalizeLogField(requestID),
		normalizeLogField(intent),
		latency.Round(time.Millisecond),
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens,
		resp.StopReason)
}

func (c *retryClient) resolveRequestID(ctx context.Context, req ports.CompletionRequest, resp *ports.CompletionResponse) string {
	requestID := strings.TrimSpace(extractRequestID(req.Metadata))
	if requestID == "" && resp != nil {
		requestID = strings.TrimSpace(extractMetadataString(resp.Metadata, "request_id"))
	}
	if requestID != "" {
		return requestID
	}
	return id.NewRequestIDWithLogID(id.LogIDFromContext(ctx))
}

func (c *retryClient) logFailureRequestEntry(
	requestID string,
	mode string,
	provider string,
	model string,
	intent string,
	latency time.Duration,
	err error,
) {
	if strings.TrimSpace(requestID) == "" {
		return
	}
	utils.LogStreamingErrorPayload(requestID, utils.LLMErrorLogDetails{
		Mode:       utils.TrimLower(mode),
		Provider:   provider,
		Model:      model,
		Intent:     sanitizeLogValue(intent, 128),
		Stage:      "retry_client",
		ErrorClass: classifyFailureError(err),
		Error:      sanitizeLogValue(err.Error(), llmFailurePreviewLimit),
		LatencyMS:  latency.Milliseconds(),
	})
}

// recordHealthLatency records a successful call latency if a healthRegistry is attached.
func (c *retryClient) recordHealthLatency(d time.Duration) {
	if c.healthRegistry != nil {
		c.healthRegistry.recordLatency(c.provider, c.model, d)
	}
}

// recordHealthError records a failed call if a healthRegistry is attached.
func (c *retryClient) recordHealthError(err error) {
	if c.healthRegistry != nil {
		c.healthRegistry.recordError(c.provider, c.model, err)
	}
}

// WrapWithRetryWithMeta wraps an existing LLM client with retry logic and
// optional provider/model metadata for log enrichment.
func WrapWithRetryWithMeta(
	client portsllm.LLMClient,
	retryConfig alexerrors.RetryConfig,
	circuitBreakerConfig alexerrors.CircuitBreakerConfig,
	provider string,
	model string,
) portsllm.LLMClient {
	// Create circuit breaker for this client
	circuitBreaker := alexerrors.NewCircuitBreaker(
		fmt.Sprintf("llm-%s", client.Model()),
		circuitBreakerConfig,
	)
	retry := NewRetryClient(client, retryConfig, circuitBreaker)
	rc, ok := retry.(*retryClient)
	if !ok {
		return retry
	}
	rc.provider = strings.TrimSpace(provider)
	if rc.model == "" {
		rc.model = strings.TrimSpace(model)
	}
	return rc
}

// WrapWithRetryAndHealth wraps an LLM client with retry, circuit breaker, and health tracking.
// The circuit breaker is automatically registered with the healthRegistry.
func WrapWithRetryAndHealth(client portsllm.LLMClient, retryConfig alexerrors.RetryConfig, circuitBreakerConfig alexerrors.CircuitBreakerConfig, hr *healthRegistry, provider, model string) portsllm.LLMClient {
	circuitBreaker := alexerrors.NewCircuitBreaker(
		fmt.Sprintf("llm-%s", client.Model()),
		circuitBreakerConfig,
	)

	if hr != nil {
		hr.register(provider, model, circuitBreaker)
	}

	return newRetryClientWithHealth(client, retryConfig, circuitBreaker, hr, provider, model)
}

