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
)

// tokenRefresher is called when a 401 is received for an OAuth token to
// obtain a fresh access token. Implementations should handle refresh-token
// flows and credential persistence.
type tokenRefresher func() (newToken string, err error)

// Rate-limit-specific backoff and circuit breaker constants.
const (
	// rateLimitBaseDelay is the minimum backoff for 429 errors (vs 1s default).
	rateLimitBaseDelay = 5 * time.Second

	// rateLimitCircuitThreshold is the number of consecutive 429s before the
	// rate-limit circuit opens. This is independent of the general circuit
	// breaker (which uses FailureThreshold, typically 5).
	rateLimitCircuitThreshold = 3

	// rateLimitCircuitCooldown is the cooldown period after the rate-limit
	// circuit opens. No requests are attempted during this window.
	rateLimitCircuitCooldown = 30 * time.Second
)

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

	// Rate-limit circuit: opens after rateLimitCircuitThreshold consecutive 429s.
	rlMu              sync.Mutex
	rlConsecutive429  int
	rlCircuitOpenedAt time.Time

	// fallback support: when all retries are exhausted on a transient error,
	// attempt a single call to the fallback client.
	fallbackClientFn func() (portsllm.LLMClient, error)
	fallbackProvider string
	fallbackModel    string
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

func (c *retryClient) streamingClient() portsllm.StreamingLLMClient {
	if streaming, ok := c.underlying.(portsllm.StreamingLLMClient); ok {
		return streaming
	}
	if adapted, ok := EnsureStreamingClient(c.underlying).(portsllm.StreamingLLMClient); ok {
		return adapted
	}
	return nil
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
	is429 := isRateLimitError(err)

	// Honour upstream Retry-After as a floor. For 429 errors the server's
	// value is authoritative and must NOT be capped by MaxDelay.
	if retryAfter := retryAfterDuration(err); retryAfter > 0 {
		if is429 {
			// Use Retry-After as-is (uncapped) for rate limits.
			return retryAfter
		}
		maxDelay := c.retryConfig.MaxDelay
		if maxDelay > 0 && retryAfter > maxDelay {
			return maxDelay
		}
		return retryAfter
	}

	// For 429 without Retry-After, use a more aggressive base delay.
	if is429 {
		delay := backoff.ExponentialClamp(rateLimitBaseDelay, 0, attempt)
		return backoff.ScaleJitter(delay, 0.25, float64(time.Now().UnixNano()%1000)/1000)
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
