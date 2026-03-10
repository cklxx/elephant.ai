package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/delivery/channels"
	"alex/internal/delivery/channels/lark"
	"alex/internal/domain/agent/presets"
	"alex/internal/infra/attachments"
	runtimeconfig "alex/internal/shared/config"
	configadmin "alex/internal/shared/config/admin"
	"alex/internal/shared/utils"
)

func LoadConfig() (ConfigResult, error) { //nolint:cyclop // sequential config loading pipeline
	envLookup := runtimeconfig.DefaultEnvLookup

	storePath := configadmin.ResolveStorePath(envLookup)
	cacheTTL := 30 * time.Second
	if ttlValue, ok := envLookup("CONFIG_ADMIN_CACHE_TTL"); ok && utils.HasContent(ttlValue) {
		if parsed, err := time.ParseDuration(strings.TrimSpace(ttlValue)); err == nil && parsed > 0 {
			cacheTTL = parsed
		}
	}
	ctx := context.Background()
	store := configadmin.NewFileStore(storePath)
	managedOverrides, err := store.LoadOverrides(ctx)
	if err != nil {
		return ConfigResult{}, err
	}
	manager := configadmin.NewManager(store, managedOverrides, configadmin.WithCacheTTL(cacheTTL))

	loader := func(ctx context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error) {
		if ctx == nil {
			ctx = context.Background()
		}
		overrides, err := manager.CurrentOverrides(ctx)
		if err != nil {
			return runtimeconfig.RuntimeConfig{}, runtimeconfig.Metadata{}, err
		}
		return runtimeconfig.Load(
			runtimeconfig.WithEnv(envLookup),
			runtimeconfig.WithOverrides(overrides),
		)
	}
	runtimeCache, err := runtimeconfig.NewRuntimeConfigCache(loader)
	if err != nil {
		return ConfigResult{}, err
	}
	runtimeCfg, runtimeMeta, err := runtimeCache.Resolve(context.Background())
	if err != nil {
		return ConfigResult{}, err
	}

	cfg := Config{
		Runtime:        runtimeCfg,
		RuntimeMeta:    runtimeMeta,
		Port:           "8080",
		DebugPort:      "9090",
		DebugBindHost:  "127.0.0.1",
		LogDir:         "logs",
		AllowedOrigins: append([]string(nil), defaultAllowedOrigins...),
		StreamGuard: StreamGuardConfig{
			MaxDuration:   2 * time.Hour,
			MaxBytes:      64 * 1024 * 1024,
			MaxConcurrent: 128,
		},
		RateLimit: RateLimitConfig{
			RequestsPerMinute: 600,
			Burst:             120,
		},
		NonStreamTimeout: 30 * time.Second,
		TaskExecution: TaskExecutionConfig{
			LeaseTTL:             45 * time.Second,
			LeaseRenewInterval:   15 * time.Second,
			MaxInFlight:          64,
			ResumeClaimBatchSize: 128,
		},
		EventHistory: EventHistoryConfig{
			Retention:   30 * 24 * time.Hour,
			MaxSessions: 100,
			SessionTTL:  1 * time.Hour,
			MaxEvents:   1000,
		},
		Session: runtimeconfig.SessionConfig{
			Dir: "~/.alex/sessions",
		},
		Channels: ChannelsConfig{
			Registry: NewChannelRegistry(),
		},
		Attachment: attachments.StoreConfig{
			Provider: attachments.ProviderLocal,
			Dir:      "~/.alex/attachments",
		},
	}

	// Register default channel configs in the registry.
	cfg.Channels.SetLarkConfig(defaultLarkGatewayConfig())
	cfg.Channels.SetTelegramConfig(defaultTelegramGatewayConfig())

	fileCfg, _, err := runtimeconfig.LoadFileConfig(runtimeconfig.WithEnv(envLookup))
	if err != nil {
		return ConfigResult{}, err
	}
	applyServerFileConfig(&cfg, fileCfg)
	applyLarkEnvFallback(&cfg, envLookup)
	applyTelegramEnvFallback(&cfg, envLookup)
	if err := validateLarkPersistenceConfig(&cfg); err != nil {
		return ConfigResult{}, err
	}
	if err := validateLarkDeliveryConfig(&cfg); err != nil {
		return ConfigResult{}, err
	}
	if err := validateTelegramPersistenceConfig(&cfg); err != nil {
		return ConfigResult{}, err
	}

	report := runtimeconfig.ValidateRuntimeConfig(cfg.Runtime)
	if report.HasErrors() {
		var ids []string
		for _, item := range report.Errors {
			ids = append(ids, item.ID)
		}
		return ConfigResult{}, fmt.Errorf("runtime config validation failed: %s", strings.Join(ids, ","))
	}

	return ConfigResult{
		Config:        cfg,
		ConfigManager: manager,
		Resolver:      runtimeCache.Resolve,
		RuntimeCache:  runtimeCache,
	}, nil
}

func defaultLarkGatewayConfig() LarkGatewayConfig {
	return LarkGatewayConfig{
		BaseConfig: channels.BaseConfig{
			SessionPrefix: "lark",
			AllowGroups:   true,
			AllowDirect:   true,
			AgentPreset:   string(presets.PresetDefault),
			ToolPreset:    string(presets.ToolPresetFull),
			ReplyTimeout:  3 * time.Minute,
		},
		BaseDomain:         "https://open.larkoffice.com",
		ToolMode:           "cli",
		AutoUploadFiles:    true,
		AutoUploadMaxBytes: 2 * 1024 * 1024,
		AutoUploadAllowExt: []string{".txt", ".md", ".json", ".yaml", ".yml", ".csv", ".log", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".pdf", ".docx", ".xlsx", ".pptx"},
		Browser: lark.BrowserConfig{
			Headless: true,
			Timeout:  60 * time.Second,
		},
		SlowProgressSummaryEnabled:  true,
		SlowProgressSummaryDelay:    30 * time.Second,
		ToolFailureAbortThreshold:   6,
		AutoChatContextSize:         20,
		ActiveSlotTTL:               6 * time.Hour,
		ActiveSlotMaxEntries:        2048,
		PendingInputRelayTTL:        30 * time.Minute,
		PendingInputRelayMaxChats:   2048,
		PendingInputRelayMaxPerChat: 64,
		AIChatSessionTTL:            45 * time.Minute,
		StateCleanupInterval:        5 * time.Minute,
		PersistenceMode:             larkPersistenceModeFile,
		PersistenceDir:              "~/.alex/lark",
		PersistenceRetention:        7 * 24 * time.Hour,
		PersistenceMaxTasksPerChat:  200,
		DeliveryMode:                "shadow",
		DeliveryWorker: lark.DeliveryWorkerConfig{
			Enabled:      true,
			PollInterval: 500 * time.Millisecond,
			BatchSize:    50,
			MaxAttempts:  8,
			BaseBackoff:  500 * time.Millisecond,
			MaxBackoff:   60 * time.Second,
			JitterRatio:  0.2,
		},
	}
}

func defaultTelegramGatewayConfig() TelegramGatewayConfig {
	return TelegramGatewayConfig{
		BaseConfig: channels.BaseConfig{
			SessionPrefix: "tg",
			AllowGroups:   true,
			AllowDirect:   true,
			AgentPreset:   string(presets.PresetDefault),
			ToolPreset:    string(presets.ToolPresetFull),
			ReplyTimeout:  3 * time.Minute,
			MemoryEnabled: true,
		},
		SlowProgressSummaryEnabled: true,
		SlowProgressSummaryDelay:   30 * time.Second,
		ActiveSlotTTL:              6 * time.Hour,
		ActiveSlotMaxEntries:       2048,
		StateCleanupInterval:       5 * time.Minute,
		PersistenceMode:            telegramPersistenceModeFile,
		PersistenceDir:             "~/.alex/telegram",
		PersistenceRetention:       7 * 24 * time.Hour,
		PersistenceMaxTasksPerChat: 200,
	}
}

func applyServerFileConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if cfg == nil {
		return
	}
	applyLarkConfig(cfg, file)
	applyTelegramConfig(cfg, file)
	applyServerHTTPConfig(cfg, file)
	applySessionConfig(cfg, file)
	applyAnalyticsConfig(cfg, file)
	applyAttachmentConfig(cfg, file)
}
