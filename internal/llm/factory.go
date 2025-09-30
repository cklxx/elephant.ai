package llm

import (
	"alex/internal/agent/ports"
	"fmt"
	"sync"
)

type Factory struct {
	cache map[string]ports.LLMClient
	mu    sync.RWMutex
}

func NewFactory() *Factory {
	return &Factory{
		cache: make(map[string]ports.LLMClient),
	}
}

func (f *Factory) GetClient(provider, model string, config Config) (ports.LLMClient, error) {
	cacheKey := fmt.Sprintf("%s:%s", provider, model)

	f.mu.RLock()
	if client, ok := f.cache[cacheKey]; ok {
		f.mu.RUnlock()
		return client, nil
	}
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

	f.mu.Lock()
	f.cache[cacheKey] = client
	f.mu.Unlock()

	return client, nil
}

type Config struct {
	APIKey     string
	BaseURL    string
	Timeout    int
	MaxRetries int
	Headers    map[string]string
}
