package llm

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// LLMInstanceCache holds cached LLM clients with synchronization
type LLMInstanceCache struct {
	mu      sync.RWMutex
	clients map[string]Client
}

var (
	globalCache = &LLMInstanceCache{
		clients: make(map[string]Client),
	}
	// Global config provider function - can be set by the application
	globalConfigProvider func() (*Config, error)
)

// SetConfigProvider sets the global config provider function
func SetConfigProvider(provider func() (*Config, error)) {
	globalConfigProvider = provider
}

// IsMockURL checks if the URL is a mock URL for testing
func IsMockURL(baseURL string) bool {
	return strings.HasPrefix(baseURL, "mock://") ||
		strings.Contains(baseURL, "mock.") ||
		strings.Contains(baseURL, "test.local") ||
		baseURL == ""
}

// GetLLMInstance returns a cached LLM client instance based on model type
// The function takes modelType (basic/reasoning) and creates/returns cached instances
func GetLLMInstance(modelType ModelType) (Client, error) {
	if globalConfigProvider == nil {
		return nil, fmt.Errorf("no config provider set - call SetConfigProvider first")
	}

	config, err := globalConfigProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	// Generate cache key based on model type
	effectiveConfig := getEffectiveConfigForModelType(modelType, config)
	cacheKey := fmt.Sprintf("%s_%s_%s", modelType, effectiveConfig.BaseURL, effectiveConfig.Model)

	globalCache.mu.RLock()
	if client, exists := globalCache.clients[cacheKey]; exists {
		globalCache.mu.RUnlock()
		return client, nil
	}
	globalCache.mu.RUnlock()

	// Create new client if not cached
	globalCache.mu.Lock()
	defer globalCache.mu.Unlock()

	// Double-check pattern
	if client, exists := globalCache.clients[cacheKey]; exists {
		return client, nil
	}

	var client Client

	// Check if this is a mock URL for testing
	if IsMockURL(effectiveConfig.BaseURL) {
		// Create mock client for testing
		client = &MockLLMClient{
			model:   effectiveConfig.Model,
			baseURL: effectiveConfig.BaseURL,
		}
		log.Printf("Created mock client for %s model (URL: %s)", modelType, effectiveConfig.BaseURL)
	} else if IsOllamaAPI(effectiveConfig.BaseURL) {
		// Create Ollama client with ultra think support for reasoning model
		enableUltraThink := (modelType == ReasoningModel)
		client, err = NewOllamaClient(effectiveConfig.BaseURL, enableUltraThink)
		if err != nil {
			return nil, fmt.Errorf("failed to create Ollama client for %s: %w", modelType, err)
		}
		log.Printf("Created Ollama client for %s model (ultra think: %v)", modelType, enableUltraThink)
	} else {
		// Create HTTP client for other APIs
		client, err = NewHTTPClient()
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP LLM client for %s: %w", modelType, err)
		}
	}

	// Cache the client
	globalCache.clients[cacheKey] = client

	return client, nil
}

// ClearInstanceCache clears all cached LLM instances
func ClearInstanceCache() {
	globalCache.mu.Lock()
	defer globalCache.mu.Unlock()

	// Close all cached clients
	for _, client := range globalCache.clients {
		if closer, ok := client.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				log.Printf("Error closing LLM client: %v", err)
			}
		}
	}

	globalCache.clients = make(map[string]Client)
}

// DefaultClientFactory implements the ClientFactory interface
type DefaultClientFactory struct {
	supportedProviders []string
}

// NewDefaultClientFactory creates a new default client factory
func NewDefaultClientFactory() *DefaultClientFactory {
	return &DefaultClientFactory{
		supportedProviders: []string{"openai", "anthropic", "azure", "custom"},
	}
}

// CreateHTTPClient creates an HTTP-mode client
func (f *DefaultClientFactory) CreateHTTPClient(config *Config) (HTTPClient, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	client, err := NewHTTPClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	return client, nil
}

// CreateStreamingClient creates a streaming-mode client
func (f *DefaultClientFactory) CreateStreamingClient(config *Config) (StreamingClient, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	client, err := NewStreamingClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create streaming client: %w", err)
	}

	return client, nil
}

// GetSupportedProviders returns list of supported providers
func (f *DefaultClientFactory) GetSupportedProviders() []string {
	return f.supportedProviders
}

// ConfigBuilder helps build LLM configurations
type ConfigBuilder struct {
	config *Config
}

// NewConfigBuilder creates a new configuration builder
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: &Config{
			Temperature: 0.7,
			MaxTokens:   1000,
			Timeout:     30 * time.Second,
		},
	}
}

// WithAPIKey sets the API key
func (b *ConfigBuilder) WithAPIKey(apiKey string) *ConfigBuilder {
	b.config.APIKey = apiKey
	return b
}

// WithBaseURL sets the base URL
func (b *ConfigBuilder) WithBaseURL(baseURL string) *ConfigBuilder {
	b.config.BaseURL = baseURL
	return b
}

// WithModel sets the model
func (b *ConfigBuilder) WithModel(model string) *ConfigBuilder {
	b.config.Model = model
	return b
}

// WithTemperature sets the temperature
func (b *ConfigBuilder) WithTemperature(temperature float64) *ConfigBuilder {
	b.config.Temperature = temperature
	return b
}

// WithMaxTokens sets the max tokens
func (b *ConfigBuilder) WithMaxTokens(maxTokens int) *ConfigBuilder {
	b.config.MaxTokens = maxTokens
	return b
}

// WithTimeout sets the timeout
func (b *ConfigBuilder) WithTimeout(timeout time.Duration) *ConfigBuilder {
	b.config.Timeout = timeout
	return b
}

// Build builds the configuration
func (b *ConfigBuilder) Build() *Config {
	return b.config
}

// ProviderConfig represents provider-specific configuration
type ProviderConfig struct {
	Name    string            `json:"name"`
	BaseURL string            `json:"base_url"`
	Models  []string          `json:"models"`
	Headers map[string]string `json:"headers,omitempty"`
}

// GetOpenAIConfig returns OpenAI provider configuration
func GetOpenAIConfig() *ProviderConfig {
	return &ProviderConfig{
		Name:    "openai",
		BaseURL: "https://api.openai.com/v1",
		Models:  []string{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo"},
	}
}

// GetAnthropicConfig returns Anthropic provider configuration
func GetAnthropicConfig() *ProviderConfig {
	return &ProviderConfig{
		Name:    "anthropic",
		BaseURL: "https://api.anthropic.com/v1",
		Models:  []string{"claude-3-sonnet", "claude-3-haiku", "claude-3-opus"},
		Headers: map[string]string{
			"anthropic-version": "2023-06-01",
		},
	}
}

// GetAzureConfig returns Azure OpenAI provider configuration
func GetAzureConfig(endpoint string, apiVersion string) *ProviderConfig {
	return &ProviderConfig{
		Name:    "azure",
		BaseURL: fmt.Sprintf("%s/openai/deployments", endpoint),
		Models:  []string{"gpt-4", "gpt-35-turbo"},
		Headers: map[string]string{
			"api-version": apiVersion,
		},
	}
}

// CreateClientFromProvider creates a client from provider configuration
func CreateClientFromProvider(provider *ProviderConfig, apiKey string, model string, clientType string) (Client, error) {
	if provider == nil {
		return nil, fmt.Errorf("provider config cannot be nil")
	}

	config := NewConfigBuilder().
		WithAPIKey(apiKey).
		WithBaseURL(provider.BaseURL).
		WithModel(model).
		Build()

	factory := NewDefaultClientFactory()

	switch clientType {
	case "http":
		return factory.CreateHTTPClient(config)
	case "streaming":
		return factory.CreateStreamingClient(config)
	default:
		return nil, fmt.Errorf("unsupported client type: %s", clientType)
	}
}

// getEffectiveConfigForModelType returns the effective configuration for a specific model type
func getEffectiveConfigForModelType(modelType ModelType, config *Config) *Config {
	// If multi-model configurations exist, use them
	if config.Models != nil {
		if modelConfig, exists := config.Models[modelType]; exists {
			return &Config{
				APIKey:      modelConfig.APIKey,
				BaseURL:     modelConfig.BaseURL,
				Model:       modelConfig.Model,
				Temperature: modelConfig.Temperature,
				MaxTokens:   modelConfig.MaxTokens,
				Timeout:     config.Timeout,
			}
		}
	}

	// Fallback to default single model config
	return config
}
