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

// Config holds server configuration.
type Config struct {
	Runtime            runtimeconfig.RuntimeConfig
	RuntimeMeta        runtimeconfig.Metadata
	Port               string
	DebugPort          string // Debug HTTP port for Lark standalone mode (default "9090")
	EnableMCP          bool
	EnvironmentSummary string
	Session            runtimeconfig.SessionConfig
	Analytics          runtimeconfig.AnalyticsConfig
	Channels           ChannelsConfig
	HooksBridge        HooksBridgeConfig
	AllowedOrigins     []string
	MaxTaskBodyBytes   int64
	StreamGuard        StreamGuardConfig
	RateLimit          RateLimitConfig
	NonStreamTimeout   time.Duration
	TaskExecution      TaskExecutionConfig
	EventHistory       EventHistoryConfig
	Attachment         attachments.StoreConfig
}

// EventHistoryConfig captures event history storage tuning.
type EventHistoryConfig struct {
	Retention   time.Duration
	MaxSessions int
	SessionTTL  time.Duration
	MaxEvents   int
}

// StreamGuardConfig captures SSE stream guard limits.
type StreamGuardConfig struct {
	MaxDuration   time.Duration
	MaxBytes      int64
	MaxConcurrent int
}

// RateLimitConfig captures HTTP rate limiting parameters.
type RateLimitConfig struct {
	RequestsPerMinute int
	Burst             int
}

// TaskExecutionConfig captures task admission and lease settings.
type TaskExecutionConfig struct {
	OwnerID              string
	LeaseTTL             time.Duration
	LeaseRenewInterval   time.Duration
	MaxInFlight          int
	ResumeClaimBatchSize int
}

// ChannelsConfig captures server-side channel gateways.
type ChannelsConfig struct {
	Lark     LarkGatewayConfig
	Telegram TelegramGatewayConfig
}

// TelegramGatewayConfig captures the resolved Telegram gateway configuration.
type TelegramGatewayConfig struct {
	channels.BaseConfig
	Enabled                       bool
	BotToken                      string
	AllowedGroups                 []int64
	ShowToolProgress              bool
	SlowProgressSummaryEnabled    bool
	SlowProgressSummaryDelay      time.Duration
	PlanReviewEnabled             bool
	PlanReviewRequireConfirmation bool
	PlanReviewPendingTTL          time.Duration
	ActiveSlotTTL                 time.Duration
	ActiveSlotMaxEntries          int
	StateCleanupInterval          time.Duration
	PersistenceMode               string
	PersistenceDir                string
	PersistenceRetention          time.Duration
	PersistenceMaxTasksPerChat    int
	MaxConcurrentTasks            int
}

// LarkGatewayConfig captures the resolved Lark gateway configuration.
type LarkGatewayConfig struct {
	channels.BaseConfig
	Enabled                       bool
	AppID                         string
	AppSecret                     string
	TenantCalendarID              string
	BaseDomain                    string
	WorkspaceDir                  string
	AutoUploadFiles               bool
	AutoUploadMaxBytes            int
	AutoUploadAllowExt            []string
	Browser                       lark.BrowserConfig
	ToolMode                      string
	ReactEmoji                    string
	InjectionAckReactEmoji        string
	ShowToolProgress              bool
	SlowProgressSummaryEnabled    bool
	SlowProgressSummaryDelay      time.Duration
	ShowPlanClarifyMessages       bool
	ToolFailureAbortThreshold     int
	AutoChatContextSize           int
	PlanReviewEnabled             bool
	PlanReviewRequireConfirmation bool
	PlanReviewPendingTTL          time.Duration
	ActiveSlotTTL                 time.Duration
	ActiveSlotMaxEntries          int
	PendingInputRelayTTL          time.Duration
	PendingInputRelayMaxChats     int
	PendingInputRelayMaxPerChat   int
	AIChatSessionTTL              time.Duration
	StateCleanupInterval          time.Duration
	PersistenceMode               string
	PersistenceDir                string
	PersistenceRetention          time.Duration
	PersistenceMaxTasksPerChat    int
	MaxConcurrentTasks            int
	DefaultPlanMode               string
	DeliveryMode                  string
	DeliveryWorker                lark.DeliveryWorkerConfig
}

// HooksBridgeConfig controls the Claude Code hooks → Lark bridge endpoint.
// Note: always enabled when Lark is enabled; token is optional.
type HooksBridgeConfig struct {
	Token         string `yaml:"token"`
	DefaultChatID string `yaml:"default_chat_id"`
}

// ConfigResult bundles all outputs from LoadConfig into a single return value.
type ConfigResult struct {
	Config        Config
	ConfigManager *configadmin.Manager
	Resolver      func(context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error)
	RuntimeCache  *runtimeconfig.RuntimeConfigCache
}

var defaultAllowedOrigins = []string{
	"http://localhost:3000",
	"http://localhost:3001",
	"https://alex.yourdomain.com",
}

const (
	larkPersistenceModeFile   = "file"
	larkPersistenceModeMemory = "memory"

	telegramPersistenceModeFile   = "file"
	telegramPersistenceModeMemory = "memory"
)

func LoadConfig() (ConfigResult, error) {
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
		EnableMCP:      true, // Default: enabled
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
			Lark: LarkGatewayConfig{
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
				ReactEmoji:                  "WAVE, Get, THINKING, MUSCLE, THUMBSUP, OK, THANKS, APPLAUSE, LGTM",
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
			},
			Telegram: TelegramGatewayConfig{
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
			},
		},
		Attachment: attachments.StoreConfig{
			Provider: attachments.ProviderLocal,
			Dir:      "~/.alex/attachments",
		},
	}

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

func applyLarkEnvFallback(cfg *Config, lookup runtimeconfig.EnvLookup) {
	if debugPort := lookupFirstNonEmptyEnv(lookup, "ALEX_DEBUG_PORT"); debugPort != "" {
		cfg.DebugPort = debugPort
	}
}

func lookupFirstNonEmptyEnv(lookup runtimeconfig.EnvLookup, keys ...string) string {
	if lookup == nil {
		return ""
	}
	for _, key := range keys {
		if value, ok := lookup(key); ok {
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func applyLarkConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Channels == nil || file.Channels.Lark == nil {
		return
	}
	larkCfg := file.Channels.Lark
	target := &cfg.Channels.Lark
	applyOptionalBool(&target.Enabled, larkCfg.Enabled)
	applyTrimmedString(&target.AppID, larkCfg.AppID)
	applyTrimmedString(&target.AppSecret, larkCfg.AppSecret)
	applyTrimmedString(&target.TenantCalendarID, larkCfg.TenantCalendarID)
	applyTrimmedString(&target.BaseDomain, larkCfg.BaseDomain)
	applyTrimmedString(&target.WorkspaceDir, larkCfg.WorkspaceDir)
	applyOptionalBool(&target.AutoUploadFiles, larkCfg.AutoUploadFiles)
	applyPositiveInt(&target.AutoUploadMaxBytes, larkCfg.AutoUploadMaxBytes)
	if len(larkCfg.AutoUploadAllowExt) > 0 {
		target.AutoUploadAllowExt = append([]string(nil), larkCfg.AutoUploadAllowExt...)
	}
	applyBrowserConfig(&target.Browser, larkCfg.Browser)
	applyTrimmedString(&target.SessionPrefix, larkCfg.SessionPrefix)
	applyTrimmedString(&target.ReplyPrefix, larkCfg.ReplyPrefix)
	applyOptionalBool(&target.AllowGroups, larkCfg.AllowGroups)
	applyOptionalBool(&target.AllowDirect, larkCfg.AllowDirect)
	applyTrimmedString(&target.AgentPreset, larkCfg.AgentPreset)
	applyTrimmedString(&target.ToolPreset, larkCfg.ToolPreset)
	applyTrimmedString(&target.ToolMode, larkCfg.ToolMode)
	applyPositiveDurationSeconds(&target.ReplyTimeout, larkCfg.ReplyTimeoutSeconds)
	applyTrimmedString(&target.ReactEmoji, larkCfg.ReactEmoji)
	applyTrimmedString(&target.InjectionAckReactEmoji, larkCfg.InjectionAckReactEmoji)
	applyOptionalBool(&target.MemoryEnabled, larkCfg.MemoryEnabled)
	applyOptionalBool(&target.ShowToolProgress, larkCfg.ShowToolProgress)
	applyOptionalBool(&target.SlowProgressSummaryEnabled, larkCfg.SlowProgressSummaryEnabled)
	applyPositiveDurationSeconds(&target.SlowProgressSummaryDelay, larkCfg.SlowProgressSummaryDelaySecs)
	applyOptionalBool(&target.ShowPlanClarifyMessages, larkCfg.ShowPlanClarifyMessages)
	applyPositiveInt(&target.ToolFailureAbortThreshold, larkCfg.ToolFailureAbortThreshold)
	applyPositiveInt(&target.AutoChatContextSize, larkCfg.AutoChatContextSize)
	applyOptionalBool(&target.PlanReviewEnabled, larkCfg.PlanReviewEnabled)
	applyOptionalBool(&target.PlanReviewRequireConfirmation, larkCfg.PlanReviewRequireConfirmation)
	applyPositiveDurationMinutes(&target.PlanReviewPendingTTL, larkCfg.PlanReviewPendingTTLMinutes)
	applyPositiveDurationMinutes(&target.ActiveSlotTTL, larkCfg.ActiveSlotTTLMinutes)
	applyPositiveInt(&target.ActiveSlotMaxEntries, larkCfg.ActiveSlotMaxEntries)
	applyPositiveDurationMinutes(&target.PendingInputRelayTTL, larkCfg.PendingInputRelayTTLMinutes)
	applyPositiveInt(&target.PendingInputRelayMaxChats, larkCfg.PendingInputRelayMaxChats)
	applyPositiveInt(&target.PendingInputRelayMaxPerChat, larkCfg.PendingInputRelayMaxPerChat)
	applyPositiveDurationMinutes(&target.AIChatSessionTTL, larkCfg.AIChatSessionTTLMinutes)
	applyPositiveDurationSeconds(&target.StateCleanupInterval, larkCfg.StateCleanupIntervalSeconds)
	applyLarkPersistenceConfig(target, larkCfg.Persistence)
	applyLarkDeliveryConfig(target, larkCfg.Delivery)
	applyPositiveInt(&target.MaxConcurrentTasks, larkCfg.MaxConcurrentTasks)
	applyOptionalTrimmedString(&target.DefaultPlanMode, larkCfg.DefaultPlanMode)
}

func applyBrowserConfig(dst *lark.BrowserConfig, browser *runtimeconfig.LarkBrowserConfig) {
	if dst == nil || browser == nil {
		return
	}
	applyTrimmedString(&dst.CDPURL, browser.CDPURL)
	applyTrimmedString(&dst.ChromePath, browser.ChromePath)
	applyOptionalBool(&dst.Headless, browser.Headless)
	applyTrimmedString(&dst.UserDataDir, browser.UserDataDir)
	applyPositiveDurationSeconds(&dst.Timeout, browser.TimeoutSeconds)
}

func applyLarkPersistenceConfig(dst *LarkGatewayConfig, persistence *runtimeconfig.LarkPersistenceConfig) {
	if dst == nil || persistence == nil {
		return
	}
	applyTrimmedLowerString(&dst.PersistenceMode, persistence.Mode)
	applyTrimmedString(&dst.PersistenceDir, persistence.Dir)
	applyPositiveDurationHours(&dst.PersistenceRetention, persistence.RetentionHours)
	applyPositiveInt(&dst.PersistenceMaxTasksPerChat, persistence.MaxTasksPerChat)
}

func applyLarkDeliveryConfig(dst *LarkGatewayConfig, delivery *runtimeconfig.LarkDeliveryConfig) {
	if dst == nil || delivery == nil {
		return
	}
	applyTrimmedLowerString(&dst.DeliveryMode, delivery.Mode)
	if delivery.Worker == nil {
		return
	}
	worker := delivery.Worker
	applyOptionalBool(&dst.DeliveryWorker.Enabled, worker.Enabled)
	applyPositiveDurationMilliseconds(&dst.DeliveryWorker.PollInterval, worker.PollIntervalMs)
	applyPositiveInt(&dst.DeliveryWorker.BatchSize, worker.BatchSize)
	applyPositiveInt(&dst.DeliveryWorker.MaxAttempts, worker.MaxAttempts)
	applyPositiveDurationMilliseconds(&dst.DeliveryWorker.BaseBackoff, worker.BaseBackoffMs)
	applyPositiveDurationMilliseconds(&dst.DeliveryWorker.MaxBackoff, worker.MaxBackoffMs)
	if worker.JitterRatio != nil && *worker.JitterRatio > 0 {
		dst.DeliveryWorker.JitterRatio = *worker.JitterRatio
	}
}

func applyTrimmedString(dst *string, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed != "" {
		*dst = trimmed
	}
}

func applyOptionalTrimmedString(dst *string, value *string) {
	if value == nil {
		return
	}
	*dst = strings.TrimSpace(*value)
}

func applyTrimmedLowerString(dst *string, value string) {
	trimmed := utils.TrimLower(value)
	if trimmed != "" {
		*dst = trimmed
	}
}

func applyOptionalBool(dst *bool, value *bool) {
	if value != nil {
		*dst = *value
	}
}

func applyPositiveInt(dst *int, value *int) {
	if value != nil && *value > 0 {
		*dst = *value
	}
}

func applyPositiveDurationSeconds(dst *time.Duration, seconds *int) {
	if seconds != nil && *seconds > 0 {
		*dst = time.Duration(*seconds) * time.Second
	}
}

func applyPositiveDurationMinutes(dst *time.Duration, minutes *int) {
	if minutes != nil && *minutes > 0 {
		*dst = time.Duration(*minutes) * time.Minute
	}
}

func applyPositiveDurationHours(dst *time.Duration, hours *int) {
	if hours != nil && *hours > 0 {
		*dst = time.Duration(*hours) * time.Hour
	}
}

func applyPositiveDurationMilliseconds(dst *time.Duration, ms *int) {
	if ms != nil && *ms > 0 {
		*dst = time.Duration(*ms) * time.Millisecond
	}
}

func validateLarkPersistenceConfig(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	mode := utils.TrimLower(cfg.Channels.Lark.PersistenceMode)
	if mode == "" {
		mode = larkPersistenceModeFile
	}
	switch mode {
	case larkPersistenceModeFile, larkPersistenceModeMemory:
	default:
		return fmt.Errorf("channels.lark.persistence.mode must be one of [file,memory], got %q", mode)
	}
	cfg.Channels.Lark.PersistenceMode = mode

	if mode == larkPersistenceModeFile {
		dir := strings.TrimSpace(cfg.Channels.Lark.PersistenceDir)
		if dir == "" {
			return fmt.Errorf("channels.lark.persistence.dir is required when persistence.mode=file")
		}
		cfg.Channels.Lark.PersistenceDir = expandHome(dir)
	}
	if cfg.Channels.Lark.PersistenceRetention <= 0 {
		cfg.Channels.Lark.PersistenceRetention = 7 * 24 * time.Hour
	}
	if cfg.Channels.Lark.PersistenceMaxTasksPerChat <= 0 {
		cfg.Channels.Lark.PersistenceMaxTasksPerChat = 200
	}
	return nil
}

func validateLarkDeliveryConfig(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	mode := utils.TrimLower(cfg.Channels.Lark.DeliveryMode)
	if mode == "" {
		mode = "shadow"
	}
	switch mode {
	case "direct", "shadow", "outbox":
	default:
		return fmt.Errorf("channels.lark.delivery.mode must be one of [direct,shadow,outbox], got %q", mode)
	}
	cfg.Channels.Lark.DeliveryMode = mode

	worker := &cfg.Channels.Lark.DeliveryWorker
	if worker.PollInterval <= 0 {
		worker.PollInterval = 500 * time.Millisecond
	}
	if worker.BatchSize <= 0 {
		worker.BatchSize = 50
	}
	if worker.MaxAttempts <= 0 {
		worker.MaxAttempts = 8
	}
	if worker.BaseBackoff <= 0 {
		worker.BaseBackoff = 500 * time.Millisecond
	}
	if worker.MaxBackoff <= 0 {
		worker.MaxBackoff = 60 * time.Second
	}
	if worker.MaxBackoff < worker.BaseBackoff {
		worker.MaxBackoff = worker.BaseBackoff
	}
	if worker.JitterRatio <= 0 {
		worker.JitterRatio = 0.2
	}
	if worker.JitterRatio > 1 {
		return fmt.Errorf("channels.lark.delivery.worker.jitter_ratio must be <= 1, got %v", worker.JitterRatio)
	}
	return nil
}

func applyTelegramConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Channels == nil || file.Channels.Telegram == nil {
		return
	}
	tgCfg := file.Channels.Telegram
	target := &cfg.Channels.Telegram
	applyOptionalBool(&target.Enabled, tgCfg.Enabled)
	applyTrimmedString(&target.BotToken, tgCfg.BotToken)
	applyTrimmedString(&target.SessionPrefix, tgCfg.SessionPrefix)
	applyTrimmedString(&target.ReplyPrefix, tgCfg.ReplyPrefix)
	applyOptionalBool(&target.AllowGroups, tgCfg.AllowGroups)
	applyOptionalBool(&target.AllowDirect, tgCfg.AllowDirect)
	applyTrimmedString(&target.AgentPreset, tgCfg.AgentPreset)
	applyTrimmedString(&target.ToolPreset, tgCfg.ToolPreset)
	applyPositiveDurationSeconds(&target.ReplyTimeout, tgCfg.ReplyTimeoutSeconds)
	applyOptionalBool(&target.MemoryEnabled, tgCfg.MemoryEnabled)
	if len(tgCfg.AllowedGroups) > 0 {
		target.AllowedGroups = append([]int64(nil), tgCfg.AllowedGroups...)
	}
	applyOptionalBool(&target.ShowToolProgress, tgCfg.ShowToolProgress)
	applyOptionalBool(&target.SlowProgressSummaryEnabled, tgCfg.SlowProgressSummaryEnabled)
	applyPositiveDurationSeconds(&target.SlowProgressSummaryDelay, tgCfg.SlowProgressSummaryDelaySecs)
	applyOptionalBool(&target.PlanReviewEnabled, tgCfg.PlanReviewEnabled)
	applyOptionalBool(&target.PlanReviewRequireConfirmation, tgCfg.PlanReviewRequireConfirmation)
	applyPositiveDurationMinutes(&target.PlanReviewPendingTTL, tgCfg.PlanReviewPendingTTLMinutes)
	applyPositiveDurationMinutes(&target.ActiveSlotTTL, tgCfg.ActiveSlotTTLMinutes)
	applyPositiveInt(&target.ActiveSlotMaxEntries, tgCfg.ActiveSlotMaxEntries)
	applyPositiveDurationSeconds(&target.StateCleanupInterval, tgCfg.StateCleanupIntervalSeconds)
	applyTelegramPersistenceConfig(target, tgCfg.Persistence)
	applyPositiveInt(&target.MaxConcurrentTasks, tgCfg.MaxConcurrentTasks)
}

func applyTelegramPersistenceConfig(dst *TelegramGatewayConfig, persistence *runtimeconfig.TelegramPersistenceConfig) {
	if dst == nil || persistence == nil {
		return
	}
	applyTrimmedLowerString(&dst.PersistenceMode, persistence.Mode)
	applyTrimmedString(&dst.PersistenceDir, persistence.Dir)
	applyPositiveDurationHours(&dst.PersistenceRetention, persistence.RetentionHours)
	applyPositiveInt(&dst.PersistenceMaxTasksPerChat, persistence.MaxTasksPerChat)
}

func applyTelegramEnvFallback(cfg *Config, lookup runtimeconfig.EnvLookup) {
	if token := lookupFirstNonEmptyEnv(lookup, "TELEGRAM_BOT_TOKEN"); token != "" {
		if cfg.Channels.Telegram.BotToken == "" {
			cfg.Channels.Telegram.BotToken = token
		}
	}
}

func validateTelegramPersistenceConfig(cfg *Config) error {
	if cfg == nil || !cfg.Channels.Telegram.Enabled {
		return nil
	}
	mode := utils.TrimLower(cfg.Channels.Telegram.PersistenceMode)
	if mode == "" {
		mode = telegramPersistenceModeFile
	}
	switch mode {
	case telegramPersistenceModeFile, telegramPersistenceModeMemory:
	default:
		return fmt.Errorf("channels.telegram.persistence.mode must be one of [file,memory], got %q", mode)
	}
	cfg.Channels.Telegram.PersistenceMode = mode

	if mode == telegramPersistenceModeFile {
		dir := strings.TrimSpace(cfg.Channels.Telegram.PersistenceDir)
		if dir == "" {
			return fmt.Errorf("channels.telegram.persistence.dir is required when persistence.mode=file")
		}
		cfg.Channels.Telegram.PersistenceDir = expandHome(dir)
	}
	if cfg.Channels.Telegram.PersistenceRetention <= 0 {
		cfg.Channels.Telegram.PersistenceRetention = 7 * 24 * time.Hour
	}
	if cfg.Channels.Telegram.PersistenceMaxTasksPerChat <= 0 {
		cfg.Channels.Telegram.PersistenceMaxTasksPerChat = 200
	}
	return nil
}

func applyServerHTTPConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Server == nil {
		return
	}
	if port := strings.TrimSpace(file.Server.Port); port != "" {
		cfg.Port = port
	}
	if debugPort := strings.TrimSpace(file.Server.DebugPort); debugPort != "" {
		cfg.DebugPort = debugPort
	}
	if file.Server.EnableMCP != nil {
		cfg.EnableMCP = *file.Server.EnableMCP
	}
	if file.Server.MaxTaskBodyBytes != nil && *file.Server.MaxTaskBodyBytes > 0 {
		cfg.MaxTaskBodyBytes = *file.Server.MaxTaskBodyBytes
	}
	if file.Server.StreamMaxDurationSeconds != nil && *file.Server.StreamMaxDurationSeconds > 0 {
		cfg.StreamGuard.MaxDuration = time.Duration(*file.Server.StreamMaxDurationSeconds) * time.Second
	}
	if file.Server.StreamMaxBytes != nil && *file.Server.StreamMaxBytes > 0 {
		cfg.StreamGuard.MaxBytes = *file.Server.StreamMaxBytes
	}
	if file.Server.StreamMaxConcurrent != nil && *file.Server.StreamMaxConcurrent > 0 {
		cfg.StreamGuard.MaxConcurrent = *file.Server.StreamMaxConcurrent
	}
	if file.Server.RateLimitRequestsPerMinute != nil && *file.Server.RateLimitRequestsPerMinute > 0 {
		cfg.RateLimit.RequestsPerMinute = *file.Server.RateLimitRequestsPerMinute
	}
	if file.Server.RateLimitBurst != nil && *file.Server.RateLimitBurst > 0 {
		cfg.RateLimit.Burst = *file.Server.RateLimitBurst
	}
	if file.Server.NonStreamTimeoutSeconds != nil && *file.Server.NonStreamTimeoutSeconds > 0 {
		cfg.NonStreamTimeout = time.Duration(*file.Server.NonStreamTimeoutSeconds) * time.Second
	}
	if ownerID := strings.TrimSpace(file.Server.TaskExecutionOwnerID); ownerID != "" {
		cfg.TaskExecution.OwnerID = ownerID
	}
	if file.Server.TaskExecutionLeaseTTLSeconds != nil && *file.Server.TaskExecutionLeaseTTLSeconds > 0 {
		cfg.TaskExecution.LeaseTTL = time.Duration(*file.Server.TaskExecutionLeaseTTLSeconds) * time.Second
	}
	if file.Server.TaskExecutionLeaseRenewIntervalSeconds != nil && *file.Server.TaskExecutionLeaseRenewIntervalSeconds > 0 {
		cfg.TaskExecution.LeaseRenewInterval = time.Duration(*file.Server.TaskExecutionLeaseRenewIntervalSeconds) * time.Second
	}
	if file.Server.TaskExecutionMaxInFlight != nil {
		if *file.Server.TaskExecutionMaxInFlight <= 0 {
			cfg.TaskExecution.MaxInFlight = 0
		} else {
			cfg.TaskExecution.MaxInFlight = *file.Server.TaskExecutionMaxInFlight
		}
	}
	if file.Server.TaskExecutionResumeClaimBatchSize != nil && *file.Server.TaskExecutionResumeClaimBatchSize > 0 {
		cfg.TaskExecution.ResumeClaimBatchSize = *file.Server.TaskExecutionResumeClaimBatchSize
	}
	if file.Server.EventHistoryRetentionDays != nil {
		days := *file.Server.EventHistoryRetentionDays
		if days <= 0 {
			cfg.EventHistory.Retention = 0
		} else {
			cfg.EventHistory.Retention = time.Duration(days) * 24 * time.Hour
		}
	}
	if file.Server.EventHistoryMaxSessions != nil {
		if *file.Server.EventHistoryMaxSessions <= 0 {
			cfg.EventHistory.MaxSessions = 0
		} else {
			cfg.EventHistory.MaxSessions = *file.Server.EventHistoryMaxSessions
		}
	}
	if file.Server.EventHistorySessionTTL != nil {
		if *file.Server.EventHistorySessionTTL <= 0 {
			cfg.EventHistory.SessionTTL = 0
		} else {
			cfg.EventHistory.SessionTTL = time.Duration(*file.Server.EventHistorySessionTTL) * time.Second
		}
	}
	if file.Server.EventHistoryMaxEvents != nil {
		if *file.Server.EventHistoryMaxEvents <= 0 {
			cfg.EventHistory.MaxEvents = 0
		} else {
			cfg.EventHistory.MaxEvents = *file.Server.EventHistoryMaxEvents
		}
	}
	if file.Server.AllowedOrigins != nil {
		cfg.AllowedOrigins = normalizeAllowedOrigins(file.Server.AllowedOrigins)
	}
}

func applySessionConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Session == nil {
		return
	}
	if dir := strings.TrimSpace(file.Session.Dir); dir != "" {
		cfg.Session.Dir = dir
	}
}

func applyAnalyticsConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Analytics == nil {
		return
	}
	cfg.Analytics = runtimeconfig.AnalyticsConfig{
		PostHogAPIKey: strings.TrimSpace(file.Analytics.PostHogAPIKey),
		PostHogHost:   strings.TrimSpace(file.Analytics.PostHogHost),
	}
}

func applyAttachmentConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Attachments == nil {
		return
	}
	if provider := strings.TrimSpace(file.Attachments.Provider); provider != "" {
		cfg.Attachment.Provider = provider
	}
	if dir := strings.TrimSpace(file.Attachments.Dir); dir != "" {
		cfg.Attachment.Dir = dir
	}
	if accountID := strings.TrimSpace(file.Attachments.CloudflareAccountID); accountID != "" {
		cfg.Attachment.CloudflareAccountID = accountID
	}
	if accessKey := strings.TrimSpace(file.Attachments.CloudflareAccessKeyID); accessKey != "" {
		cfg.Attachment.CloudflareAccessKeyID = accessKey
	}
	if secret := strings.TrimSpace(file.Attachments.CloudflareSecretAccessKey); secret != "" {
		cfg.Attachment.CloudflareSecretAccessKey = secret
	}
	if bucket := strings.TrimSpace(file.Attachments.CloudflareBucket); bucket != "" {
		cfg.Attachment.CloudflareBucket = bucket
	}
	if base := strings.TrimSpace(file.Attachments.CloudflarePublicBaseURL); base != "" {
		cfg.Attachment.CloudflarePublicBaseURL = base
	}
	if prefix := strings.TrimSpace(file.Attachments.CloudflareKeyPrefix); prefix != "" {
		cfg.Attachment.CloudflareKeyPrefix = prefix
	}
	if ttlRaw := strings.TrimSpace(file.Attachments.PresignTTL); ttlRaw != "" {
		if parsed, err := time.ParseDuration(ttlRaw); err == nil && parsed > 0 {
			cfg.Attachment.PresignTTL = parsed
		}
	}
}

func normalizeAllowedOrigins(values []string) []string {
	return utils.TrimDedupeStrings(values)
}
