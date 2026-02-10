package bootstrap

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func clearLoadConfigValidationEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ALEX_PROFILE", "")
	t.Setenv("LLM_PROVIDER", "")
	t.Setenv("LLM_MODEL", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_API_KEY", "")
}

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

func TestLoadConfig_EventHistoryAsyncDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")

	cr, err := LoadConfig()
	cfg := cr.Config
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.EventHistory.AsyncBatchSize != 200 {
		t.Fatalf("expected default async batch size 200, got %d", cfg.EventHistory.AsyncBatchSize)
	}
	if cfg.EventHistory.AsyncFlushInterval != 250*time.Millisecond {
		t.Fatalf("expected default async flush interval 250ms, got %s", cfg.EventHistory.AsyncFlushInterval)
	}
	if cfg.EventHistory.AsyncAppendTimeout != 50*time.Millisecond {
		t.Fatalf("expected default async append timeout 50ms, got %s", cfg.EventHistory.AsyncAppendTimeout)
	}
	if cfg.EventHistory.AsyncQueueCapacity != 8192 {
		t.Fatalf("expected default async queue size 8192, got %d", cfg.EventHistory.AsyncQueueCapacity)
	}
	if cfg.EventHistory.AsyncFlushRequestCoalesceWindow != 8*time.Millisecond {
		t.Fatalf("expected default async flush request coalesce window 8ms, got %s", cfg.EventHistory.AsyncFlushRequestCoalesceWindow)
	}
	if cfg.EventHistory.AsyncBackpressureHighWatermark != (8192*80)/100 {
		t.Fatalf("expected default async backpressure high watermark %d, got %d", (8192*80)/100, cfg.EventHistory.AsyncBackpressureHighWatermark)
	}
	if !cfg.EventHistory.DegradeDebugEventsOnBackpressure {
		t.Fatalf("expected default degrade_debug_events_on_backpressure=true")
	}
}

func TestLoadConfig_AuthJWTSecretFromEnvFallback(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := []byte(`
runtime:
  llm_provider: mock
`)
	if err := os.WriteFile(configPath, configContent, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")
	t.Setenv("AUTH_JWT_SECRET", "env-auth-secret")

	cr, err := LoadConfig()
	cfg := cr.Config
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Auth.JWTSecret != "env-auth-secret" {
		t.Fatalf("expected auth jwt secret from env fallback, got %q", cfg.Auth.JWTSecret)
	}
}

func TestLoadConfig_AuthYAMLOverridesEnvFallback(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := []byte(`
runtime:
  llm_provider: mock
auth:
  jwt_secret: "yaml-auth-secret"
`)
	if err := os.WriteFile(configPath, configContent, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")
	t.Setenv("AUTH_JWT_SECRET", "env-auth-secret")

	cr, err := LoadConfig()
	cfg := cr.Config
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Auth.JWTSecret != "yaml-auth-secret" {
		t.Fatalf("expected yaml jwt secret to take precedence, got %q", cfg.Auth.JWTSecret)
	}
}

func TestLoadConfig_AuthDatabasePoolMaxConnsFromEnvFallback(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := []byte(`
runtime:
  llm_provider: mock
`)
	if err := os.WriteFile(configPath, configContent, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")
	t.Setenv("AUTH_DATABASE_POOL_MAX_CONNS", "6")

	cr, err := LoadConfig()
	cfg := cr.Config
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Auth.DatabasePoolMaxConns == nil || *cfg.Auth.DatabasePoolMaxConns != 6 {
		t.Fatalf("expected auth database pool max conns from env fallback, got %#v", cfg.Auth.DatabasePoolMaxConns)
	}
}

func TestLoadConfig_AuthDatabasePoolMaxConnsYAMLOverridesEnvFallback(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := []byte(`
runtime:
  llm_provider: mock
auth:
  database_pool_max_conns: 9
`)
	if err := os.WriteFile(configPath, configContent, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")
	t.Setenv("AUTH_DATABASE_POOL_MAX_CONNS", "6")

	cr, err := LoadConfig()
	cfg := cr.Config
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Auth.DatabasePoolMaxConns == nil || *cfg.Auth.DatabasePoolMaxConns != 9 {
		t.Fatalf("expected yaml auth database pool max conns to take precedence, got %#v", cfg.Auth.DatabasePoolMaxConns)
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
  event_history_async_flush_request_coalesce_window_ms: 25
  event_history_async_backpressure_high_watermark: 3072
  event_history_degrade_debug_events_on_backpressure: false
`)
	if err := os.WriteFile(configPath, configContent, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")

	cr, err := LoadConfig()
	cfg := cr.Config
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.EventHistory.AsyncBatchSize != 320 {
		t.Fatalf("expected async batch size 320, got %d", cfg.EventHistory.AsyncBatchSize)
	}
	if cfg.EventHistory.AsyncFlushInterval != 1200*time.Millisecond {
		t.Fatalf("expected async flush interval 1200ms, got %s", cfg.EventHistory.AsyncFlushInterval)
	}
	if cfg.EventHistory.AsyncAppendTimeout != 90*time.Millisecond {
		t.Fatalf("expected async append timeout 90ms, got %s", cfg.EventHistory.AsyncAppendTimeout)
	}
	if cfg.EventHistory.AsyncQueueCapacity != 4096 {
		t.Fatalf("expected async queue size 4096, got %d", cfg.EventHistory.AsyncQueueCapacity)
	}
	if cfg.EventHistory.AsyncFlushRequestCoalesceWindow != 25*time.Millisecond {
		t.Fatalf("expected async flush request coalesce window 25ms, got %s", cfg.EventHistory.AsyncFlushRequestCoalesceWindow)
	}
	if cfg.EventHistory.AsyncBackpressureHighWatermark != 3072 {
		t.Fatalf("expected async backpressure high watermark 3072, got %d", cfg.EventHistory.AsyncBackpressureHighWatermark)
	}
	if cfg.EventHistory.DegradeDebugEventsOnBackpressure {
		t.Fatalf("expected degrade_debug_events_on_backpressure=false, got true")
	}
}

func TestLoadConfig_ServerEventHistoryQueueCapacityRescalesDefaultBackpressureWatermark(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := []byte(`
runtime:
  llm_provider: mock
server:
  event_history_async_queue_capacity: 4096
`)
	if err := os.WriteFile(configPath, configContent, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")

	cr, err := LoadConfig()
	cfg := cr.Config
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.EventHistory.AsyncQueueCapacity != 4096 {
		t.Fatalf("expected async queue capacity 4096, got %d", cfg.EventHistory.AsyncQueueCapacity)
	}
	if cfg.EventHistory.AsyncBackpressureHighWatermark != (4096*80)/100 {
		t.Fatalf("expected async backpressure high watermark %d, got %d", (4096*80)/100, cfg.EventHistory.AsyncBackpressureHighWatermark)
	}
}

func TestLoadConfig_ProductionProfileRequiresAPIKey(t *testing.T) {
	clearLoadConfigValidationEnv(t)
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := []byte(`
runtime:
  profile: production
  llm_provider: openai
  llm_model: gpt-4o-mini
`)
	if err := os.WriteFile(configPath, configContent, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)

	_, err := LoadConfig()
	if err == nil {
		t.Fatalf("expected LoadConfig to fail in production profile without api key")
	}
	if got := err.Error(); !strings.Contains(got, "llm-api-key") {
		t.Fatalf("expected llm-api-key validation error, got %q", got)
	}
}

func TestLoadConfig_QuickstartProfileAllowsMissingAPIKey(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := []byte(`
runtime:
  profile: quickstart
  llm_provider: openai
  llm_model: gpt-4o-mini
`)
	if err := os.WriteFile(configPath, configContent, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)

	cr, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected quickstart profile to allow missing api key, got %v", err)
	}
	if cr.Config.Runtime.Profile != "quickstart" {
		t.Fatalf("expected runtime profile quickstart, got %q", cr.Config.Runtime.Profile)
	}
}
