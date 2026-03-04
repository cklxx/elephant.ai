package subscription

import "testing"

func TestLookupProviderPresetReturnsCopiedRecommendations(t *testing.T) {
	first, ok := LookupProviderPreset(" OpenAI ")
	if !ok {
		t.Fatal("expected openai preset")
	}
	if len(first.RecommendedModels) == 0 {
		t.Fatal("expected recommended models")
	}

	first.RecommendedModels[0].ID = "mutated-model"

	second, ok := LookupProviderPreset("openai")
	if !ok {
		t.Fatal("expected openai preset")
	}
	if len(second.RecommendedModels) == 0 {
		t.Fatal("expected recommended models")
	}
	if second.RecommendedModels[0].ID == "mutated-model" {
		t.Fatal("expected lookup to return a defensive copy")
	}
}

func TestListProviderPresetsReturnsSortedCopies(t *testing.T) {
	first := ListProviderPresets()
	if len(first) == 0 {
		t.Fatal("expected non-empty presets")
	}
	for i := 1; i < len(first); i++ {
		if first[i-1].Provider > first[i].Provider {
			t.Fatalf("expected sorted provider IDs, got %q before %q", first[i-1].Provider, first[i].Provider)
		}
	}

	openAIFirst := findProviderPreset(first, "openai")
	if openAIFirst == nil || len(openAIFirst.RecommendedModels) == 0 {
		t.Fatal("expected openai recommendation list")
	}
	openAIFirst.RecommendedModels[0].ID = "mutated-model"

	second := ListProviderPresets()
	openAISecond := findProviderPreset(second, "openai")
	if openAISecond == nil || len(openAISecond.RecommendedModels) == 0 {
		t.Fatal("expected openai recommendation list")
	}
	if openAISecond.RecommendedModels[0].ID == "mutated-model" {
		t.Fatal("expected list call to return defensive copies")
	}
}

func TestLookupEnvCredential(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		provider   string
		env        map[string]string
		wantKey    string
		wantURL    string
		wantSource string
		wantOK     bool
	}{
		{
			name:       "kimi with KIMI_API_KEY",
			provider:   "kimi",
			env:        map[string]string{"KIMI_API_KEY": "sk-kimi-123"},
			wantKey:    "sk-kimi-123",
			wantURL:    "https://api.kimi.com/coding/v1",
			wantSource: "KIMI_API_KEY",
			wantOK:     true,
		},
		{
			name:       "kimi falls back to OPENAI_API_KEY",
			provider:   "kimi",
			env:        map[string]string{"OPENAI_API_KEY": "sk-openai-for-kimi"},
			wantKey:    "sk-openai-for-kimi",
			wantURL:    "https://api.kimi.com/coding/v1",
			wantSource: "OPENAI_API_KEY",
			wantOK:     true,
		},
		{
			name:       "kimi with base url override",
			provider:   "kimi",
			env:        map[string]string{"KIMI_API_KEY": "sk-k", "KIMI_BASE_URL": "https://custom.kimi/v1"},
			wantKey:    "sk-k",
			wantURL:    "https://custom.kimi/v1",
			wantSource: "KIMI_API_KEY",
			wantOK:     true,
		},
		{
			name:     "kimi no env vars",
			provider: "kimi",
			env:      map[string]string{},
			wantOK:   false,
		},
		{
			name:       "openai with OPENAI_API_KEY",
			provider:   "openai",
			env:        map[string]string{"OPENAI_API_KEY": "sk-oai"},
			wantKey:    "sk-oai",
			wantURL:    "https://api.openai.com/v1",
			wantSource: "OPENAI_API_KEY",
			wantOK:     true,
		},
		{
			name:       "universal fallback LLM_API_KEY for known provider",
			provider:   "glm",
			env:        map[string]string{"LLM_API_KEY": "sk-universal"},
			wantKey:    "sk-universal",
			wantURL:    "https://open.bigmodel.cn/api/paas/v4",
			wantSource: "LLM_API_KEY",
			wantOK:     true,
		},
		{
			name:       "universal fallback LLM_API_KEY for unknown provider",
			provider:   "unknown_provider",
			env:        map[string]string{"LLM_API_KEY": "sk-fallback"},
			wantKey:    "sk-fallback",
			wantURL:    "",
			wantSource: "LLM_API_KEY",
			wantOK:     true,
		},
		{
			name:     "unknown provider no env",
			provider: "unknown_provider",
			env:      map[string]string{},
			wantOK:   false,
		},
		{
			name:       "provider-specific takes priority over LLM_API_KEY",
			provider:   "minimax",
			env:        map[string]string{"MINIMAX_API_KEY": "sk-mm", "LLM_API_KEY": "sk-universal"},
			wantKey:    "sk-mm",
			wantURL:    "https://api.minimax.chat/v1",
			wantSource: "MINIMAX_API_KEY",
			wantOK:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lookup := func(key string) (string, bool) {
				v, ok := tt.env[key]
				return v, ok
			}
			gotKey, gotURL, gotSource, gotOK := LookupEnvCredential(tt.provider, lookup)
			if gotOK != tt.wantOK {
				t.Fatalf("ok: got %v, want %v", gotOK, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if gotKey != tt.wantKey {
				t.Fatalf("apiKey: got %q, want %q", gotKey, tt.wantKey)
			}
			if gotURL != tt.wantURL {
				t.Fatalf("baseURL: got %q, want %q", gotURL, tt.wantURL)
			}
			if gotSource != tt.wantSource {
				t.Fatalf("source: got %q, want %q", gotSource, tt.wantSource)
			}
		})
	}
}

func findProviderPreset(items []ProviderPreset, provider string) *ProviderPreset {
	for i := range items {
		if items[i].Provider == provider {
			return &items[i]
		}
	}
	return nil
}
