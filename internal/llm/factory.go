package llm

import (
	"alex/internal/agent/ports"
	alexerrors "alex/internal/errors"
	"fmt"
	"sync"
)

type Factory struct {
	cache                map[string]ports.LLMClient
	mu                   sync.RWMutex
	enableRetry          bool
	retryConfig          alexerrors.RetryConfig
	circuitBreakerConfig alexerrors.CircuitBreakerConfig
}

func NewFactory() *Factory {
	return &Factory{
		cache:                make(map[string]ports.LLMClient),
		enableRetry:          true, // Enabled by default
		retryConfig:          alexerrors.DefaultRetryConfig(),
		circuitBreakerConfig: alexerrors.DefaultCircuitBreakerConfig(),
	}
}

// NewFactoryWithRetryConfig creates a factory with custom retry configuration
func NewFactoryWithRetryConfig(retryConfig alexerrors.RetryConfig, circuitBreakerConfig alexerrors.CircuitBreakerConfig) *Factory {
	return &Factory{
		cache:                make(map[string]ports.LLMClient),
		enableRetry:          true,
		retryConfig:          retryConfig,
		circuitBreakerConfig: circuitBreakerConfig,
	}
}

// DisableRetry disables retry logic for all clients created by this factory
func (f *Factory) DisableRetry() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enableRetry = false
}

func (f *Factory) GetClient(provider, model string, config Config) (ports.LLMClient, error) {
	return f.getClient(provider, model, config, true)
}

// GetIsolatedClient creates a new non-cached client instance for session isolation
// This is useful when per-session state (like cost tracking callbacks) needs to be isolated
func (f *Factory) GetIsolatedClient(provider, model string, config Config) (ports.LLMClient, error) {
	return f.getClient(provider, model, config, false)
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

	// Wrap with retry logic if enabled
	if enableRetry {
		client = WrapWithRetry(client, retryConfig, circuitBreakerConfig)
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
