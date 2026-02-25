package llmclient

import (
	"testing"

	portsllm "alex/internal/domain/agent/ports/llm"
	runtimeconfig "alex/internal/shared/config"
)

type stubFactory struct {
	provider string
	model    string
	cfg      portsllm.LLMConfig
}

func (s *stubFactory) GetClient(provider, model string, config portsllm.LLMConfig) (portsllm.LLMClient, error) {
	s.provider = provider
	s.model = model
	s.cfg = config
	return nil, nil
}

func (s *stubFactory) GetIsolatedClient(provider, model string, config portsllm.LLMConfig) (portsllm.LLMClient, error) {
	s.provider = provider
	s.model = model
	s.cfg = config
	return nil, nil
}

func (s *stubFactory) DisableRetry() {}

func TestBuildConfigWithRefresh(t *testing.T) {
	profile := runtimeconfig.LLMProfile{
		Provider: "openai",
		Model:    "gpt-4o-mini",
		APIKey:   "base-key",
		BaseURL:  "https://api.openai.com/v1",
		Headers: map[string]string{
			" Authorization ": "Bearer token",
		},
	}
	cfg := BuildConfigWithRefresh(profile, func(provider string) (string, string, bool) {
		if provider != "openai" {
			t.Fatalf("unexpected provider %q", provider)
		}
		return "refreshed-key", "https://refresh.local/v1", true
	}, true)

	if cfg.APIKey != "refreshed-key" || cfg.BaseURL != "https://refresh.local/v1" {
		t.Fatalf("unexpected refreshed config: %+v", cfg)
	}
	if cfg.Headers["Authorization"] != "Bearer token" {
		t.Fatalf("expected sanitized headers, got %+v", cfg.Headers)
	}
}

func TestGetIsolatedClientFromProfile(t *testing.T) {
	factory := &stubFactory{}
	profile := runtimeconfig.LLMProfile{
		Provider: "openai",
		Model:    "gpt-4o-mini",
		APIKey:   "base-key",
		BaseURL:  "https://api.openai.com/v1",
	}
	_, cfg, err := GetIsolatedClientFromProfile(factory, profile, nil, false)
	if err != nil {
		t.Fatalf("GetIsolatedClientFromProfile error: %v", err)
	}
	if factory.provider != "openai" || factory.model != "gpt-4o-mini" {
		t.Fatalf("unexpected provider/model: %s/%s", factory.provider, factory.model)
	}
	if cfg.APIKey != "base-key" {
		t.Fatalf("expected base key in cfg, got %+v", cfg)
	}
}

func TestGetClientFromProfileRequiresProviderAndModel(t *testing.T) {
	factory := &stubFactory{}
	_, _, err := GetClientFromProfile(factory, runtimeconfig.LLMProfile{}, nil, false)
	if err == nil {
		t.Fatal("expected missing profile fields to fail")
	}
}
