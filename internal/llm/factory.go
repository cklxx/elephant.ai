package llm

import (
	"alex/internal/agent/ports"
	alexerrors "alex/internal/errors"
	"fmt"
	"sync"

	"golang.org/x/time/rate"
)

// Ensure Factory implements ports.LLMClientFactory interface
var _ ports.LLMClientFactory = (*Factory)(nil)

type Factory struct {
	cache                map[string]ports.LLMClient
	mu                   sync.RWMutex
	enableRetry          bool
	retryConfig          alexerrors.RetryConfig
	circuitBreakerConfig alexerrors.CircuitBreakerConfig
	userRateLimit        rate.Limit
	userRateBurst        int
}

func NewFactory() *Factory {
	return &Factory{
		cache:                make(map[string]ports.LLMClient),
		enableRetry:          true, // Enabled by default
		retryConfig:          alexerrors.DefaultRetryConfig(),
		circuitBreakerConfig: alexerrors.DefaultCircuitBreakerConfig(),
		userRateBurst:        1,
	}
}

// NewFactoryWithRetryConfig creates a factory with custom retry configuration
func NewFactoryWithRetryConfig(retryConfig alexerrors.RetryConfig, circuitBreakerConfig alexerrors.CircuitBreakerConfig) *Factory {
	return &Factory{
		cache:                make(map[string]ports.LLMClient),
		enableRetry:          true,
		retryConfig:          retryConfig,
		circuitBreakerConfig: circuitBreakerConfig,
		userRateBurst:        1,
	}
}

// EnableUserRateLimit enforces a per-user limiter around LLM calls.
func (f *Factory) EnableUserRateLimit(limit rate.Limit, burst int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.userRateLimit = limit
	if burst < 1 {
		burst = 1
	}
	f.userRateBurst = burst
}

// DisableRetry disables retry logic for all clients created by this factory
func (f *Factory) DisableRetry() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enableRetry = false
}

// GetClient implements ports.LLMClientFactory interface
// Creates or retrieves a cached LLM client
func (f *Factory) GetClient(provider, model string, config ports.LLMConfig) (ports.LLMClient, error) {
	return f.getClient(provider, model, adaptConfig(config), true)
}

// GetIsolatedClient implements ports.LLMClientFactory interface
// Creates a new non-cached client instance for session isolation
// This is useful when per-session state (like cost tracking callbacks) needs to be isolated
func (f *Factory) GetIsolatedClient(provider, model string, config ports.LLMConfig) (ports.LLMClient, error) {
	return f.getClient(provider, model, adaptConfig(config), false)
}

// adaptConfig converts ports.LLMConfig to internal Config
func adaptConfig(config ports.LLMConfig) Config {
	return Config{
		APIKey:     config.APIKey,
		BaseURL:    config.BaseURL,
		Timeout:    config.Timeout,
		MaxRetries: config.MaxRetries,
		Headers:    config.Headers,
	}
}

func (f *Factory) getClient(provider, model string, config Config, useCache bool) (ports.LLMClient, error) {
	cacheKey := fmt.Sprintf("%s:%s", provider, model)

	// Check cache if enabled
	if useCache {
		f.mu.RLock()
		if client, ok := f.cache[cacheKey]; ok {
			f.mu.RUnlock()
			return client, nil
		}
		f.mu.RUnlock()
	}

	// Get retry configuration
	f.mu.RLock()
	enableRetry := f.enableRetry
	retryConfig := f.retryConfig
	circuitBreakerConfig := f.circuitBreakerConfig
	f.mu.RUnlock()

	var client ports.LLMClient
	var err error

	switch provider {
	case "openai", "openrouter", "deepseek":
		client, err = NewOpenAIClient(model, config)
	case "ollama":
		client, err = NewOllamaClient(model, config)
	case "mock":
		client = NewMockClient()
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}

	if err != nil {
		return nil, err
	}

	client = EnsureStreamingClient(client)

	// Wrap with retry logic if enabled
	if enableRetry {
		client = WrapWithRetry(client, retryConfig, circuitBreakerConfig)
	}

	if f.userRateLimit > 0 {
		client = WrapWithUserRateLimit(client, f.userRateLimit, f.userRateBurst)
	}

	// Cache only if requested
	if useCache {
		f.mu.Lock()
		f.cache[cacheKey] = client
		f.mu.Unlock()
	}

	return client, nil
}

type Config struct {
	APIKey     string
	BaseURL    string
	Timeout    int
	MaxRetries int
	Headers    map[string]string
}
