package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	alexerrors "alex/internal/shared/errors"
	"alex/internal/shared/utils"
	id "alex/internal/shared/utils/id"
)

// isRateLimitError returns true if the error represents an HTTP 429 rate limit.
func isRateLimitError(err error) bool {
	var transientErr *alexerrors.TransientError
	if errors.As(err, &transientErr) && transientErr.StatusCode == 429 {
		return true
	}
	return false
}

// checkRateLimitCircuit returns an error if the rate-limit circuit is open
// (too many consecutive 429s). Callers should check this before each attempt.
func (c *retryClient) checkRateLimitCircuit() error {
	c.rlMu.Lock()
	defer c.rlMu.Unlock()

	if c.rlConsecutive429 < rateLimitCircuitThreshold {
		return nil
	}
	// Circuit is open — check if cooldown has elapsed.
	if time.Since(c.rlCircuitOpenedAt) < rateLimitCircuitCooldown {
		remaining := rateLimitCircuitCooldown - time.Since(c.rlCircuitOpenedAt)
		return alexerrors.NewDegradedError(
			fmt.Errorf("rate limit circuit open for %s (429 ×%d)", c.underlying.Model(), c.rlConsecutive429),
			fmt.Sprintf("Rate limit circuit open for '%s'. Cooling down for %v after %d consecutive 429 errors.",
				c.underlying.Model(), remaining.Round(time.Second), c.rlConsecutive429),
			"",
		)
	}
	// Cooldown elapsed — half-open: reset counter and let one request through.
	c.rlConsecutive429 = 0
	c.logger.Info("Rate-limit circuit half-open for %s, allowing probe request", c.underlying.Model())
	return nil
}

// recordRateLimitOutcome updates the consecutive-429 counter.
func (c *retryClient) recordRateLimitOutcome(err error) {
	c.rlMu.Lock()
	defer c.rlMu.Unlock()

	if isRateLimitError(err) {
		c.rlConsecutive429++
		if c.rlConsecutive429 == rateLimitCircuitThreshold {
			c.rlCircuitOpenedAt = time.Now()
			c.logger.Warn("Rate-limit circuit OPEN for %s after %d consecutive 429s, cooldown %v",
				c.underlying.Model(), c.rlConsecutive429, rateLimitCircuitCooldown)
		}
	} else {
		c.rlConsecutive429 = 0
	}
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
	{patterns: []string{"529", "overloaded"}, message: "Server overloaded (529). Retrying request."},
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
				terr := alexerrors.NewTransientError(err, rule.message)
				// Tag rate-limit errors with StatusCode so retryDelay and the
				// rate-limit circuit can identify them.
				if pattern == "429" || pattern == "rate limit" {
					terr.StatusCode = 429
				}
				return terr
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
	llmMessage := alexerrors.FormatForLLM(err)
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
