package llm

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLLMInstanceCache tests the LLMInstanceCache structure
func TestLLMInstanceCache(t *testing.T) {
	cache := &LLMInstanceCache{
		clients: make(map[string]Client),
	}

	assert.NotNil(t, cache)
	assert.NotNil(t, cache.clients)
	assert.Empty(t, cache.clients)
}

// TestSetConfigProvider tests the global config provider setting
func TestSetConfigProvider(t *testing.T) {
	// Store original provider to restore later
	originalProvider := globalConfigProvider

	// Test setting a mock provider
	mockProvider := func() (*Config, error) {
		return &Config{
			Model:   "test-model",
			APIKey:  "test-key",
			BaseURL: "https://api.test.com",
		}, nil
	}

	SetConfigProvider(mockProvider)
	assert.NotNil(t, globalConfigProvider)

	// Test that the provider works
	config, err := globalConfigProvider()
	require.NoError(t, err)
	assert.Equal(t, "test-model", config.Model)
	assert.Equal(t, "test-key", config.APIKey)

	// Restore original provider
	globalConfigProvider = originalProvider
}

// TestGetLLMInstanceNoProvider tests getting LLM instance without config provider
func TestGetLLMInstanceNoProvider(t *testing.T) {
	// Store original provider to restore later
	originalProvider := globalConfigProvider

	// Clear the global config provider
	globalConfigProvider = nil

	client, err := GetLLMInstance(BasicModel)

	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "no config provider set")

	// Restore original provider
	globalConfigProvider = originalProvider
}

// TestGetLLMInstanceConfigError tests getting LLM instance with config provider error
func TestGetLLMInstanceConfigError(t *testing.T) {
	// Store original provider to restore later
	originalProvider := globalConfigProvider

	// Set a provider that returns an error
	SetConfigProvider(func() (*Config, error) {
		return nil, assert.AnError
	})

	client, err := GetLLMInstance(BasicModel)

	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "failed to get config")

	// Restore original provider
	globalConfigProvider = originalProvider
}

// Note: MockLLMClient is now defined in mock.go

// TestLLMInstanceCacheConcurrency tests concurrent access to the cache
func TestLLMInstanceCacheConcurrency(t *testing.T) {
	cache := &LLMInstanceCache{
		clients: make(map[string]Client),
	}

	var wg sync.WaitGroup
	numGoroutines := 10

	// Test concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Write operation
			cache.mu.Lock()
			cache.clients[fmt.Sprintf("client_%d", id)] = &MockLLMClient{
				model:   fmt.Sprintf("model_%d", id),
				baseURL: fmt.Sprintf("https://api%d.test.com", id),
			}
			cache.mu.Unlock()

			// Read operation
			cache.mu.RLock()
			_ = len(cache.clients)
			cache.mu.RUnlock()
		}(i)
	}

	wg.Wait()

	cache.mu.RLock()
	clientCount := len(cache.clients)
	cache.mu.RUnlock()

	assert.Equal(t, numGoroutines, clientCount)
}

// TestLLMInstanceCacheReadWrite tests basic cache operations
func TestLLMInstanceCacheReadWrite(t *testing.T) {
	cache := &LLMInstanceCache{
		clients: make(map[string]Client),
	}

	mockClient := &MockLLMClient{
		model:   "test-model",
		baseURL: "https://api.test.com",
	}

	// Test write
	cache.mu.Lock()
	cache.clients["test-key"] = mockClient
	cache.mu.Unlock()

	// Test read
	cache.mu.RLock()
	client, exists := cache.clients["test-key"]
	cache.mu.RUnlock()

	assert.True(t, exists)
	assert.Equal(t, mockClient, client)

	// Test read non-existent key
	cache.mu.RLock()
	_, exists = cache.clients["non-existent"]
	cache.mu.RUnlock()

	assert.False(t, exists)
}

// TestGlobalCacheInitialization tests that global cache is properly initialized
func TestGlobalCacheInitialization(t *testing.T) {
	assert.NotNil(t, globalCache)
	assert.NotNil(t, globalCache.clients)
}

// TestConfigProviderChaining tests multiple config provider changes
func TestConfigProviderChaining(t *testing.T) {
	// Store original provider
	originalProvider := globalConfigProvider

	// Test setting multiple providers
	provider1 := func() (*Config, error) {
		return &Config{Model: "model1"}, nil
	}
	provider2 := func() (*Config, error) {
		return &Config{Model: "model2"}, nil
	}

	SetConfigProvider(provider1)
	config, err := globalConfigProvider()
	require.NoError(t, err)
	assert.Equal(t, "model1", config.Model)

	SetConfigProvider(provider2)
	config, err = globalConfigProvider()
	require.NoError(t, err)
	assert.Equal(t, "model2", config.Model)

	// Restore original provider
	globalConfigProvider = originalProvider
}

// TestModelTypeHandling tests model type parameter handling
func TestModelTypeHandling(t *testing.T) {
	// Test with different model types
	basicType := BasicModel
	reasoningType := ReasoningModel

	assert.Equal(t, ModelType("basic"), basicType)
	assert.Equal(t, ModelType("reasoning"), reasoningType)
	assert.NotEqual(t, basicType, reasoningType)
}

// TestConfigMultiModelSupport tests multi-model configuration handling
func TestConfigMultiModelSupport(t *testing.T) {
	config := &Config{
		DefaultModelType: BasicModel,
		Models: map[ModelType]*ModelConfig{
			BasicModel: {
				Model:   "gpt-3.5-turbo",
				BaseURL: "https://api.openai.com/v1",
				APIKey:  "basic-key",
			},
			ReasoningModel: {
				Model:   "gpt-4",
				BaseURL: "https://api.openai.com/v1",
				APIKey:  "reasoning-key",
			},
		},
	}

	assert.Equal(t, BasicModel, config.DefaultModelType)
	assert.Len(t, config.Models, 2)

	basicConfig := config.Models[BasicModel]
	require.NotNil(t, basicConfig)
	assert.Equal(t, "gpt-3.5-turbo", basicConfig.Model)

	reasoningConfig := config.Models[ReasoningModel]
	require.NotNil(t, reasoningConfig)
	assert.Equal(t, "gpt-4", reasoningConfig.Model)
}

// TestCacheKeyGeneration tests cache key generation logic
func TestCacheKeyGeneration(t *testing.T) {
	// Test cache key generation for different model types and configs
	config1 := &ModelConfig{
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-4",
	}

	config2 := &ModelConfig{
		BaseURL: "https://api.anthropic.com/v1",
		Model:   "claude-3",
	}

	// Test that different configs generate different keys
	key1 := fmt.Sprintf("%s_%s_%s", BasicModel, config1.BaseURL, config1.Model)
	key2 := fmt.Sprintf("%s_%s_%s", BasicModel, config2.BaseURL, config2.Model)
	key3 := fmt.Sprintf("%s_%s_%s", ReasoningModel, config1.BaseURL, config1.Model)

	assert.NotEqual(t, key1, key2)
	assert.NotEqual(t, key1, key3)
	assert.NotEqual(t, key2, key3)

	// Test same configs generate same keys
	key1_duplicate := fmt.Sprintf("%s_%s_%s", BasicModel, config1.BaseURL, config1.Model)
	assert.Equal(t, key1, key1_duplicate)
}

// Benchmark tests for performance
func BenchmarkCacheRead(b *testing.B) {
	cache := &LLMInstanceCache{
		clients: map[string]Client{
			"test-key": &MockLLMClient{model: "test-model"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.mu.RLock()
		_ = cache.clients["test-key"]
		cache.mu.RUnlock()
	}
}

func BenchmarkCacheWrite(b *testing.B) {
	cache := &LLMInstanceCache{
		clients: make(map[string]Client),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.mu.Lock()
		cache.clients[fmt.Sprintf("key_%d", i)] = &MockLLMClient{model: "test"}
		cache.mu.Unlock()
	}
}
