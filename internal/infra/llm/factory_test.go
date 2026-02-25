package llm

import (
	"testing"
	"time"

	portsllm "alex/internal/domain/agent/ports/llm"
	alexerrors "alex/internal/shared/errors"
)

// --- adaptConfig ---

func TestAdaptConfig_AllFields(t *testing.T) {
	input := portsllm.LLMConfig{
		APIKey:     "sk-test",
		BaseURL:    "https://api.example.com",
		Timeout:    30,
		MaxRetries: 3,
		Headers:    map[string]string{"X-Custom": "value"},
	}
	got := adaptConfig(input)
	if got.APIKey != "sk-test" {
		t.Fatalf("expected APIKey, got %q", got.APIKey)
	}
	if got.BaseURL != "https://api.example.com" {
		t.Fatalf("expected BaseURL, got %q", got.BaseURL)
	}
	if got.Timeout != 30 {
		t.Fatalf("expected Timeout 30, got %d", got.Timeout)
	}
	if got.MaxRetries != 3 {
		t.Fatalf("expected MaxRetries 3, got %d", got.MaxRetries)
	}
	if got.Headers["X-Custom"] != "value" {
		t.Fatalf("expected header, got %v", got.Headers)
	}
}

func TestAdaptConfig_Empty(t *testing.T) {
	got := adaptConfig(portsllm.LLMConfig{})
	if got.APIKey != "" || got.BaseURL != "" || got.Timeout != 0 || got.MaxRetries != 0 {
		t.Fatalf("expected zero values, got %+v", got)
	}
}

// --- newLLMCache ---

func TestNewLLMCache_ZeroDisables(t *testing.T) {
	if got := newLLMCache(0); got != nil {
		t.Fatal("expected nil cache for size 0")
	}
}

func TestNewLLMCache_NegativeDisables(t *testing.T) {
	if got := newLLMCache(-1); got != nil {
		t.Fatal("expected nil cache for negative size")
	}
}

func TestNewLLMCache_PositiveCreates(t *testing.T) {
	got := newLLMCache(10)
	if got == nil {
		t.Fatal("expected non-nil cache for positive size")
	}
}

// --- Factory.GetClient unknown provider ---

func TestFactory_GetClient_UnknownProvider(t *testing.T) {
	factory := NewFactory()
	factory.DisableRetry()

	_, err := factory.GetClient("nonexistent", "model", portsllm.LLMConfig{})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if err.Error() != "unknown provider: nonexistent" {
		t.Fatalf("expected unknown provider error, got %q", err.Error())
	}
}

// --- Factory.GetIsolatedClient ---

func TestFactory_GetIsolatedClient_NeverCaches(t *testing.T) {
	factory := NewFactory()
	factory.SetCacheOptions(10, time.Hour)

	cfg := portsllm.LLMConfig{}
	client1, err := factory.GetIsolatedClient("mock", "model-a", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	client2, err := factory.GetIsolatedClient("mock", "model-a", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client1 == client2 {
		t.Fatal("expected isolated clients to be different instances")
	}
}

func TestFactory_GetIsolatedClient_DoesNotPopulateCache(t *testing.T) {
	factory := NewFactory()
	factory.SetCacheOptions(10, time.Hour)

	cfg := portsllm.LLMConfig{}
	_, err := factory.GetIsolatedClient("mock", "model-a", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// A subsequent GetClient should create a new instance (not find cached)
	client1, _ := factory.GetClient("mock", "model-a", cfg)
	client2, _ := factory.GetClient("mock", "model-a", cfg)
	if client1 != client2 {
		t.Fatal("expected GetClient to cache after first call")
	}
}

// --- Factory.GetProviderHealth ---

func TestFactory_GetProviderHealth_NilRegistry(t *testing.T) {
	factory := NewFactory()
	if got := factory.GetProviderHealth(); got != nil {
		t.Fatalf("expected nil when no HealthRegistry, got %v", got)
	}
}

// --- Factory.SetCacheOptions ---

func TestFactory_SetCacheOptions_DisablesCache(t *testing.T) {
	factory := NewFactory()
	factory.SetCacheOptions(0, time.Hour)

	cfg := portsllm.LLMConfig{}
	client1, _ := factory.GetClient("mock", "model-a", cfg)
	client2, _ := factory.GetClient("mock", "model-a", cfg)
	if client1 == client2 {
		t.Fatal("expected no caching when size=0")
	}
}

// --- Factory.EnableUserRateLimit ---

func TestFactory_EnableUserRateLimit_MinBurst(t *testing.T) {
	factory := NewFactory()
	factory.EnableUserRateLimit(1.0, 0) // burst < 1 should be clamped to 1
	factory.mu.RLock()
	burst := factory.userRateBurst
	factory.mu.RUnlock()
	if burst != 1 {
		t.Fatalf("expected burst clamped to 1, got %d", burst)
	}
}

// --- Factory.DisableRetry ---

func TestFactory_DisableRetry(t *testing.T) {
	factory := NewFactory()
	if !factory.enableRetry {
		t.Fatal("expected retry enabled by default")
	}
	factory.DisableRetry()
	factory.mu.RLock()
	enabled := factory.enableRetry
	factory.mu.RUnlock()
	if enabled {
		t.Fatal("expected retry disabled after DisableRetry")
	}
}

// --- NewFactoryWithRetryConfig ---

func TestNewFactoryWithRetryConfig_SetsValues(t *testing.T) {
	factory := NewFactoryWithRetryConfig(
		alexerrors.DefaultRetryConfig(),
		alexerrors.DefaultCircuitBreakerConfig(),
	)
	if factory == nil {
		t.Fatal("expected non-nil factory")
	}
	if !factory.enableRetry {
		t.Fatal("expected retry enabled")
	}
	if factory.cache == nil {
		t.Fatal("expected cache initialized")
	}
}

// --- Factory provider aliases ---

func TestFactory_ResponsesProviderAliases(t *testing.T) {
	factory := NewFactory()
	factory.DisableRetry()

	cfg := portsllm.LLMConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.openai.com/v1",
	}
	for _, provider := range []string{"openai-responses", "responses", "codex"} {
		_, err := factory.GetClient(provider, "gpt-4o", cfg)
		if err != nil {
			t.Fatalf("GetClient(%q) unexpected error: %v", provider, err)
		}
	}
}

func TestFactory_AnthropicProviderAliases(t *testing.T) {
	factory := NewFactory()
	factory.DisableRetry()

	cfg := portsllm.LLMConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.anthropic.com",
	}
	for _, provider := range []string{"anthropic", "claude"} {
		_, err := factory.GetClient(provider, "claude-3", cfg)
		if err != nil {
			t.Fatalf("GetClient(%q) unexpected error: %v", provider, err)
		}
	}
}

func TestFactory_LlamaCppProviderAliases(t *testing.T) {
	factory := NewFactory()
	factory.DisableRetry()

	cfg := portsllm.LLMConfig{BaseURL: "http://localhost:8080"}
	for _, provider := range []string{"llama.cpp", "llama-cpp", "llamacpp"} {
		_, err := factory.GetClient(provider, "local", cfg)
		if err != nil {
			t.Fatalf("GetClient(%q) unexpected error: %v", provider, err)
		}
	}
}

func TestFactory_MockProvider(t *testing.T) {
	factory := NewFactory()
	factory.DisableRetry()

	client, err := factory.GetClient("mock", "any", portsllm.LLMConfig{})
	if err != nil {
		t.Fatalf("expected mock client, got error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil mock client")
	}
}
