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

func TestLoadConfig_LarkPersistenceDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")

	cr, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cr.Config.Channels.Lark.PersistenceMode != "file" {
		t.Fatalf("expected default persistence mode file, got %q", cr.Config.Channels.Lark.PersistenceMode)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("resolve home dir: %v", err)
	}
	wantDir := filepath.Join(home, ".alex", "lark")
	if cr.Config.Channels.Lark.PersistenceDir != wantDir {
		t.Fatalf("expected default persistence dir %q, got %q", wantDir, cr.Config.Channels.Lark.PersistenceDir)
	}
	if cr.Config.Channels.Lark.PersistenceRetention != 7*24*time.Hour {
		t.Fatalf("expected default persistence retention 168h, got %s", cr.Config.Channels.Lark.PersistenceRetention)
	}
	if cr.Config.Channels.Lark.PersistenceMaxTasksPerChat != 200 {
		t.Fatalf("expected default max tasks per chat 200, got %d", cr.Config.Channels.Lark.PersistenceMaxTasksPerChat)
	}
}

func TestLoadConfig_InvalidLarkPersistenceMode(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := []byte(`
runtime:
  llm_provider: mock
channels:
  lark:
    persistence:
      mode: invalid
`)
	if err := os.WriteFile(configPath, configContent, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")

	_, err := LoadConfig()
	if err == nil {
		t.Fatalf("expected invalid persistence mode error")
	}
	if !strings.Contains(err.Error(), "channels.lark.persistence.mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_ExpandsLarkPersistenceDir(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := []byte(`
runtime:
  llm_provider: mock
channels:
  lark:
    persistence:
      mode: file
      dir: "~/.alex/lark-custom"
`)
	if err := os.WriteFile(configPath, configContent, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")

	cr, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("resolve home dir: %v", err)
	}
	wantDir := filepath.Join(home, ".alex", "lark-custom")
	if cr.Config.Channels.Lark.PersistenceDir != wantDir {
		t.Fatalf("expected expanded persistence dir %q, got %q", wantDir, cr.Config.Channels.Lark.PersistenceDir)
	}
}

func TestLoadConfig_LarkRuntimeStateLimits(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := []byte(`
runtime:
  llm_provider: mock
channels:
  lark:
    active_slot_ttl_minutes: 90
    active_slot_max_entries: 1200
    pending_input_relay_ttl_minutes: 25
    pending_input_relay_max_chats: 600
    pending_input_relay_max_per_chat: 20
    ai_chat_session_ttl_minutes: 30
    state_cleanup_interval_seconds: 70
`)
	if err := os.WriteFile(configPath, configContent, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")

	cr, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	lark := cr.Config.Channels.Lark
	if lark.ActiveSlotTTL != 90*time.Minute {
		t.Fatalf("expected active slot ttl 90m, got %s", lark.ActiveSlotTTL)
	}
	if lark.ActiveSlotMaxEntries != 1200 {
		t.Fatalf("expected active slot max 1200, got %d", lark.ActiveSlotMaxEntries)
	}
	if lark.PendingInputRelayTTL != 25*time.Minute {
		t.Fatalf("expected pending relay ttl 25m, got %s", lark.PendingInputRelayTTL)
	}
	if lark.PendingInputRelayMaxChats != 600 {
		t.Fatalf("expected pending relay max chats 600, got %d", lark.PendingInputRelayMaxChats)
	}
	if lark.PendingInputRelayMaxPerChat != 20 {
		t.Fatalf("expected pending relay max per chat 20, got %d", lark.PendingInputRelayMaxPerChat)
	}
	if lark.AIChatSessionTTL != 30*time.Minute {
		t.Fatalf("expected ai chat session ttl 30m, got %s", lark.AIChatSessionTTL)
	}
	if lark.StateCleanupInterval != 70*time.Second {
		t.Fatalf("expected state cleanup interval 70s, got %s", lark.StateCleanupInterval)
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

func TestLoadConfig_ServerTaskExecutionOverrides(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := []byte(`
runtime:
  llm_provider: mock
server:
  task_execution_owner_id: worker-a
  task_execution_lease_ttl_seconds: 90
  task_execution_lease_renew_interval_seconds: 30
  task_execution_max_in_flight: 33
  task_execution_resume_claim_batch_size: 77
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

	if cfg.TaskExecution.OwnerID != "worker-a" {
		t.Fatalf("expected task execution owner worker-a, got %q", cfg.TaskExecution.OwnerID)
	}
	if cfg.TaskExecution.LeaseTTL != 90*time.Second {
		t.Fatalf("expected task execution lease ttl 90s, got %s", cfg.TaskExecution.LeaseTTL)
	}
	if cfg.TaskExecution.LeaseRenewInterval != 30*time.Second {
		t.Fatalf("expected task execution renew interval 30s, got %s", cfg.TaskExecution.LeaseRenewInterval)
	}
	if cfg.TaskExecution.MaxInFlight != 33 {
		t.Fatalf("expected task execution max in flight 33, got %d", cfg.TaskExecution.MaxInFlight)
	}
	if cfg.TaskExecution.ResumeClaimBatchSize != 77 {
		t.Fatalf("expected task execution resume claim batch size 77, got %d", cfg.TaskExecution.ResumeClaimBatchSize)
	}
}

func TestLoadConfig_ServerTaskExecutionAllowsAdmissionDisable(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := []byte(`
runtime:
  llm_provider: mock
server:
  task_execution_max_in_flight: 0
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
	if cfg.TaskExecution.MaxInFlight != 0 {
		t.Fatalf("expected task execution max in flight to be disabled (0), got %d", cfg.TaskExecution.MaxInFlight)
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
