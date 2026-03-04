package llm

import (
	"fmt"
	"strings"
	"sync"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	portsllm "alex/internal/domain/agent/ports/llm"
	alexerrors "alex/internal/shared/errors"
	"alex/internal/shared/utils"

	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/time/rate"
)

// Ensure Factory implements portsllm.LLMClientFactory interface
var _ portsllm.LLMClientFactory = (*Factory)(nil)

type Factory struct {
	cache                *lru.Cache[string, cacheEntry]
	cacheTTL             time.Duration
	mu                   sync.RWMutex
	enableRetry          bool
	retryConfig          alexerrors.RetryConfig
	circuitBreakerConfig alexerrors.CircuitBreakerConfig
	userRateLimit        rate.Limit
	userRateBurst        int
	kimiRateLimit        rate.Limit
	kimiRateBurst        int
	kimiLimiter          *rate.Limiter
	toolCallParser       agent.FunctionCallParser
	HealthRegistry       *HealthRegistry
	registry             *Registry
}

type cacheEntry struct {
	client    portsllm.LLMClient
	expiresAt time.Time
}

const (
	defaultLLMCacheSize = 64
	defaultLLMCacheTTL  = 30 * time.Minute
)

func NewFactory() *Factory {
	return &Factory{
		cache:                newLLMCache(defaultLLMCacheSize),
		cacheTTL:             defaultLLMCacheTTL,
		enableRetry:          true, // Enabled by default
		retryConfig:          alexerrors.DefaultRetryConfig(),
		circuitBreakerConfig: alexerrors.DefaultCircuitBreakerConfig(),
		userRateBurst:        1,
		kimiRateBurst:        1,
		registry:             NewDefaultRegistry(),
	}
}

// NewFactoryWithRetryConfig creates a factory with custom retry configuration
func NewFactoryWithRetryConfig(retryConfig alexerrors.RetryConfig, circuitBreakerConfig alexerrors.CircuitBreakerConfig) *Factory {
	return &Factory{
		cache:                newLLMCache(defaultLLMCacheSize),
		cacheTTL:             defaultLLMCacheTTL,
		enableRetry:          true,
		retryConfig:          retryConfig,
		circuitBreakerConfig: circuitBreakerConfig,
		userRateBurst:        1,
		kimiRateBurst:        1,
		registry:             NewDefaultRegistry(),
	}
}

// SetCacheOptions configures the LLM client cache.
// A size <= 0 disables caching. A TTL <= 0 disables expiration.
func (f *Factory) SetCacheOptions(size int, ttl time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cache = newLLMCache(size)
	f.cacheTTL = ttl
}

func newLLMCache(size int) *lru.Cache[string, cacheEntry] {
	if size <= 0 {
		return nil
	}
	cache, err := lru.New[string, cacheEntry](size)
	if err != nil {
		return nil
	}
	return cache
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

// EnableKimiRateLimit enforces a global limiter for Kimi/Moonshot traffic.
// The limiter is shared across all clients created by this factory.
func (f *Factory) EnableKimiRateLimit(limit rate.Limit, burst int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.kimiRateLimit = limit
	if burst < 1 {
		burst = 1
	}
	f.kimiRateBurst = burst
	if limit > 0 {
		f.kimiLimiter = rate.NewLimiter(limit, burst)
	} else {
		f.kimiLimiter = nil
	}
}

// DisableRetry disables retry logic for all clients created by this factory
func (f *Factory) DisableRetry() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enableRetry = false
}

// EnableToolCallParsing enables automatic parsing of <tool_call>...</tool_call>
// fallbacks when upstream providers do not return native tool calls.
func (f *Factory) EnableToolCallParsing(parser agent.FunctionCallParser) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.toolCallParser = parser
}

// GetClient implements portsllm.LLMClientFactory interface
// Creates or retrieves a cached LLM client
func (f *Factory) GetClient(provider, model string, config portsllm.LLMConfig) (portsllm.LLMClient, error) {
	return f.getClient(provider, model, adaptConfig(config), true)
}

// GetIsolatedClient implements portsllm.LLMClientFactory interface
// Creates a new non-cached client instance for session isolation
// This is useful when per-session state (like cost tracking callbacks) needs to be isolated
func (f *Factory) GetIsolatedClient(provider, model string, config portsllm.LLMConfig) (portsllm.LLMClient, error) {
	return f.getClient(provider, model, adaptConfig(config), false)
}

// adaptConfig converts portsllm.LLMConfig to internal Config
func adaptConfig(config portsllm.LLMConfig) Config {
	return Config{
		APIKey:     config.APIKey,
		BaseURL:    config.BaseURL,
		Timeout:    config.Timeout,
		MaxRetries: config.MaxRetries,
		Headers:    config.Headers,
	}
}

func (f *Factory) getClient(provider, model string, config Config, useCache bool) (portsllm.LLMClient, error) {
	cacheKey := fmt.Sprintf("%s:%s", provider, model)
	now := time.Now()

	f.mu.RLock()
	enableRetry := f.enableRetry
	retryConfig := f.retryConfig
	circuitBreakerConfig := f.circuitBreakerConfig
	toolCallParser := f.toolCallParser
	cache := f.cache
	cacheTTL := f.cacheTTL
	userRateLimit := f.userRateLimit
	userRateBurst := f.userRateBurst
	kimiLimiter := f.kimiLimiter
	healthRegistry := f.HealthRegistry
	registry := f.registry
	f.mu.RUnlock()

	// Check cache if enabled
	if useCache {
		if cache != nil {
			if entry, ok := cache.Get(cacheKey); ok {
				if entry.client != nil && (entry.expiresAt.IsZero() || now.Before(entry.expiresAt)) {
					return entry.client, nil
				}
				cache.Remove(cacheKey)
			}
		}
	}

	desc, ok := registry.Get(provider)
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}

	// Apply provider-specific config mutations.
	if desc.ConfigMutator != nil {
		desc.ConfigMutator(&config)
	}

	// Kimi-compat behaviors for any Kimi target (URL/model detection preserves existing behavior).
	if isKimiTarget(provider, model, config.BaseURL) {
		config.KimiCompat = true
		if config.Headers == nil {
			config.Headers = make(map[string]string)
		}
		if _, hasUA := config.Headers["User-Agent"]; !hasUA {
			config.Headers["User-Agent"] = "KimiCLI/1.3"
		}
	}

	// Codex endpoint behaviors.
	if strings.Contains(strings.ToLower(strings.TrimSpace(config.BaseURL)), "/backend-api/codex") {
		config.CodexEndpoint = true
	}

	client, err := desc.ClientFactory(model, config)
	if err != nil {
		return nil, err
	}

	client = EnsureStreamingClient(client)

	if kimiLimiter != nil && isKimiTarget(provider, model, config.BaseURL) {
		client = WrapWithSharedRateLimit(client, kimiLimiter)
	}

	// Wrap with retry logic if enabled
	if enableRetry {
		if healthRegistry != nil {
			client = WrapWithRetryAndHealth(client, retryConfig, circuitBreakerConfig, healthRegistry, provider, model)
		} else {
			client = WrapWithRetryWithMeta(client, retryConfig, circuitBreakerConfig, provider, model)
		}
	}

	if userRateLimit > 0 {
		client = WrapWithUserRateLimit(client, userRateLimit, userRateBurst)
	}

	if toolCallParser != nil {
		client = WrapWithToolCallParsing(client, toolCallParser)
	}

	// Cache only if requested
	if useCache {
		if cache != nil {
			var expiresAt time.Time
			if cacheTTL > 0 {
				expiresAt = now.Add(cacheTTL)
			}
			cache.Add(cacheKey, cacheEntry{client: client, expiresAt: expiresAt})
		}
	}

	return client, nil
}

func isKimiTarget(provider, model, baseURL string) bool {
	provider = utils.TrimLower(provider)
	model = utils.TrimLower(model)
	baseURL = utils.TrimLower(baseURL)

	if provider == "kimi" {
		return true
	}
	if strings.Contains(model, "kimi") {
		return true
	}
	return strings.Contains(baseURL, "kimi.com") || strings.Contains(baseURL, "moonshot")
}

// GetProviderHealth returns health snapshots for all registered providers.
// Returns nil if no HealthRegistry is configured.
func (f *Factory) GetProviderHealth() []ProviderHealth {
	f.mu.RLock()
	hr := f.HealthRegistry
	f.mu.RUnlock()

	if hr == nil {
		return nil
	}
	return hr.GetAllHealth()
}

type Config struct {
	APIKey        string
	BaseURL       string
	Timeout       int
	MaxRetries    int
	Headers       map[string]string
	KimiCompat    bool // Kimi-specific protocol quirks (drop empty messages, inject reasoning_content)
	CodexEndpoint bool // Codex endpoint behaviors (stream-only, instructions field, omit temperature)
}
