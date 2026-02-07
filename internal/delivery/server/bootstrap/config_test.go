package bootstrap

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
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

func TestLoadConfig_LarkCardCallbackPortFromYAML(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := []byte(`
runtime:
  llm_provider: mock
channels:
  lark:
    enabled: true
    card_callback_port: "9393"
`)
	if err := os.WriteFile(configPath, configContent, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")

	cfg, _, _, _, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if got := cfg.Channels.Lark.CardCallbackPort; got != "9393" {
		t.Fatalf("expected card_callback_port from YAML, got %q", got)
	}
}

func TestLoadConfig_LarkCardCallbackPortFromEnvFallback(t *testing.T) {
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
	t.Setenv("LARK_CARD_CALLBACK_PORT", "9494")

	cfg, _, _, _, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if got := cfg.Channels.Lark.CardCallbackPort; got != "9494" {
		t.Fatalf("expected card_callback_port from env fallback, got %q", got)
	}
}

func TestLoadConfig_LarkCardCallbackPortYAMLOverridesEnv(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := []byte(`
runtime:
  llm_provider: mock
channels:
  lark:
    enabled: true
    card_callback_port: "9595"
`)
	if err := os.WriteFile(configPath, configContent, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")
	t.Setenv("LARK_CARD_CALLBACK_PORT", "9494")

	cfg, _, _, _, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if got := cfg.Channels.Lark.CardCallbackPort; got != "9595" {
		t.Fatalf("expected YAML port to take precedence, got %q", got)
	}
}

func TestLoadConfig_EventHistoryAsyncDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")

	cfg, _, _, _, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.EventHistoryAsyncBatchSize != 200 {
		t.Fatalf("expected default async batch size 200, got %d", cfg.EventHistoryAsyncBatchSize)
	}
	if cfg.EventHistoryAsyncFlushInterval != 250*time.Millisecond {
		t.Fatalf("expected default async flush interval 250ms, got %s", cfg.EventHistoryAsyncFlushInterval)
	}
	if cfg.EventHistoryAsyncAppendTimeout != 50*time.Millisecond {
		t.Fatalf("expected default async append timeout 50ms, got %s", cfg.EventHistoryAsyncAppendTimeout)
	}
	if cfg.EventHistoryAsyncQueueCapacity != 8192 {
		t.Fatalf("expected default async queue size 8192, got %d", cfg.EventHistoryAsyncQueueCapacity)
	}
}

func TestLoadConfig_ServerEventHistoryAsyncOverride(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := []byte(`
runtime:
  llm_provider: mock
server:
  event_history_async_batch_size: 320
  event_history_async_flush_interval_ms: 1200
  event_history_async_append_timeout_ms: 90
  event_history_async_queue_capacity: 4096
`)
	if err := os.WriteFile(configPath, configContent, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")

	cfg, _, _, _, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.EventHistoryAsyncBatchSize != 320 {
		t.Fatalf("expected async batch size 320, got %d", cfg.EventHistoryAsyncBatchSize)
	}
	if cfg.EventHistoryAsyncFlushInterval != 1200*time.Millisecond {
		t.Fatalf("expected async flush interval 1200ms, got %s", cfg.EventHistoryAsyncFlushInterval)
	}
	if cfg.EventHistoryAsyncAppendTimeout != 90*time.Millisecond {
		t.Fatalf("expected async append timeout 90ms, got %s", cfg.EventHistoryAsyncAppendTimeout)
	}
	if cfg.EventHistoryAsyncQueueCapacity != 4096 {
		t.Fatalf("expected async queue size 4096, got %d", cfg.EventHistoryAsyncQueueCapacity)
	}
}
