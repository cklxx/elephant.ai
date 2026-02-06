package llm

import (
	"testing"

	"alex/internal/agent/ports"
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
