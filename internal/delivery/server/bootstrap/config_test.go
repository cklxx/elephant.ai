package bootstrap

import (
	"os"
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

	cfg, _, _, _, err := LoadConfig()
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

func TestLoadConfig_LarkCardCallbackTokenFromEnvFallback(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := []byte(`
runtime:
  llm_provider: mock
channels:
  lark:
    enabled: true
`)
	if err := os.WriteFile(configPath, configContent, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")
	t.Setenv("LARK_VERIFICATION_TOKEN", "verify_token_from_env")

	cfg, _, _, _, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if got := cfg.Channels.Lark.CardCallbackVerificationToken; got != "verify_token_from_env" {
		t.Fatalf("expected env fallback token, got %q", got)
	}
}

func TestLoadConfig_LarkCardCallbackTokenKeepsYamlValue(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := []byte(`
runtime:
  llm_provider: mock
channels:
  lark:
    enabled: true
    card_callback_verification_token: "verify_token_from_yaml"
`)
	if err := os.WriteFile(configPath, configContent, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")
	t.Setenv("LARK_VERIFICATION_TOKEN", "verify_token_from_env")

	cfg, _, _, _, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if got := cfg.Channels.Lark.CardCallbackVerificationToken; got != "verify_token_from_yaml" {
		t.Fatalf("expected yaml token to take precedence, got %q", got)
	}
}
