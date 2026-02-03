package bootstrap

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestNormalizeAllowedOrigins(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "empty",
			input: []string{},
			want:  []string{},
		},
		{
			name:  "mixed separators and duplicates",
			input: []string{" https://a.example.com", "https://b.example.com", "https://a.example.com", "http://c.example.com\t"},
			want:  []string{"https://a.example.com", "https://b.example.com", "http://c.example.com"},
		},
		{
			name:  "trims whitespace",
			input: []string{"  http://localhost:3000  ", "   http://localhost:3001 "},
			want:  []string{"http://localhost:3000", "http://localhost:3001"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeAllowedOrigins(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("normalizeAllowedOrigins(%v) = %#v; want %#v", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadConfig_DefaultLarkCardsErrorsOnly(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")

	cfg, _, _, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if !cfg.Channels.Lark.CardsEnabled {
		t.Fatalf("expected CardsEnabled true by default")
	}
	if cfg.Channels.Lark.CardsPlanReview {
		t.Errorf("expected CardsPlanReview false by default")
	}
	if cfg.Channels.Lark.CardsResults {
		t.Errorf("expected CardsResults false by default")
	}
	if !cfg.Channels.Lark.CardsErrors {
		t.Errorf("expected CardsErrors true by default")
	}
}
