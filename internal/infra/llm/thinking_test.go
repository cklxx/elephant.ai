package llm

import (
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestIsArkEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    bool
	}{
		{"ark cn beijing", "https://ark-cn-beijing.bytedance.net/api/v3", true},
		{"ark uppercase", "https://ARK-cn-beijing.bytedance.net/api/v3", true},
		{"ark mixed case", "https://Ark.example.com/api/v1", true},
		{"openai", "https://api.openai.com/v1", false},
		{"deepseek", "https://api.deepseek.com/v1", false},
		{"empty", "", false},
		{"whitespace only", "  ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isArkEndpoint(tt.baseURL)
			if got != tt.want {
				t.Fatalf("isArkEndpoint(%q) = %v, want %v", tt.baseURL, got, tt.want)
			}
		})
	}
}

func TestShouldSendArkReasoning(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		cfg     ports.ThinkingConfig
		want    bool
	}{
		{
			name:    "ark endpoint with thinking enabled",
			baseURL: "https://ark-cn-beijing.bytedance.net/api/v3",
			cfg:     ports.ThinkingConfig{Enabled: true, Effort: "medium"},
			want:    true,
		},
		{
			name:    "ark endpoint with thinking disabled",
			baseURL: "https://ark-cn-beijing.bytedance.net/api/v3",
			cfg:     ports.ThinkingConfig{Enabled: false},
			want:    false,
		},
		{
			name:    "non-ark endpoint with thinking enabled",
			baseURL: "https://api.openai.com/v1",
			cfg:     ports.ThinkingConfig{Enabled: true, Effort: "high"},
			want:    false,
		},
		{
			name:    "empty base URL with thinking enabled",
			baseURL: "",
			cfg:     ports.ThinkingConfig{Enabled: true},
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSendArkReasoning(tt.baseURL, tt.cfg)
			if got != tt.want {
				t.Fatalf("shouldSendArkReasoning(%q, %+v) = %v, want %v", tt.baseURL, tt.cfg, got, tt.want)
			}
		})
	}
}

func TestShouldSendOpenAIReasoning(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		model   string
		cfg     ports.ThinkingConfig
		want    bool
	}{
		{
			name:    "codex endpoint with thinking enabled",
			baseURL: "https://chatgpt.com/backend-api/codex",
			model:   "gpt-5.3-codex-spark",
			cfg:     ports.ThinkingConfig{Enabled: true},
			want:    true,
		},
		{
			name:    "codex endpoint with thinking disabled",
			baseURL: "https://chatgpt.com/backend-api/codex",
			model:   "gpt-5.3-codex-spark",
			cfg:     ports.ThinkingConfig{Enabled: false},
			want:    false,
		},
		{
			name:    "openai reasoning model enabled",
			baseURL: "https://api.openai.com/v1",
			model:   "o3",
			cfg:     ports.ThinkingConfig{Enabled: true},
			want:    true,
		},
		{
			name:    "openai non-reasoning model enabled",
			baseURL: "https://api.openai.com/v1",
			model:   "gpt-4o",
			cfg:     ports.ThinkingConfig{Enabled: true},
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSendOpenAIReasoning(tt.baseURL, tt.model, tt.cfg)
			if got != tt.want {
				t.Fatalf("shouldSendOpenAIReasoning(%q, %q, %+v) = %v, want %v", tt.baseURL, tt.model, tt.cfg, got, tt.want)
			}
		})
	}
}

func TestApplyCodexReasoningDefaults(t *testing.T) {
	t.Run("adds summary auto when missing", func(t *testing.T) {
		reasoning := map[string]any{"effort": "medium"}
		got := applyCodexReasoningDefaults(reasoning)
		if got["summary"] != "auto" {
			t.Fatalf("expected summary auto, got %#v", got["summary"])
		}
	})

	t.Run("preserves explicit summary", func(t *testing.T) {
		reasoning := map[string]any{
			"effort":  "high",
			"summary": "detailed",
		}
		got := applyCodexReasoningDefaults(reasoning)
		if got["summary"] != "detailed" {
			t.Fatalf("expected summary detailed, got %#v", got["summary"])
		}
	})

	t.Run("supports env override", func(t *testing.T) {
		t.Setenv("ALEX_CODEX_REASONING_SUMMARY", "detailed")
		reasoning := map[string]any{"effort": "medium"}
		got := applyCodexReasoningDefaults(reasoning)
		if got["summary"] != "detailed" {
			t.Fatalf("expected summary detailed from env, got %#v", got["summary"])
		}
	})
}
