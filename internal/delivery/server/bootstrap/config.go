package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/delivery/channels"
	"alex/internal/infra/attachments"
	"alex/internal/domain/agent/presets"
	runtimeconfig "alex/internal/shared/config"
	configadmin "alex/internal/shared/config/admin"
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
	Retention                        time.Duration
	MaxSessions                      int
	SessionTTL                       time.Duration
	MaxEvents                        int
	AsyncBatchSize                   int
	AsyncFlushInterval               time.Duration
	AsyncAppendTimeout               time.Duration
	AsyncQueueCapacity               int
	AsyncFlushRequestCoalesceWindow  time.Duration
	AsyncBackpressureHighWatermark   int
	DegradeDebugEventsOnBackpressure bool
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
	Lark LarkGatewayConfig
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
	Browser                       LarkBrowserConfig
	ToolMode                      string
	ReactEmoji                    string
	InjectionAckReactEmoji        string
	FinalAnswerReviewReactEmoji   string
	ShowToolProgress              bool
	ShowPlanClarifyMessages       bool
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
}

// LarkBrowserConfig captures local browser settings for Lark.
type LarkBrowserConfig struct {
	CDPURL      string
	ChromePath  string
	Headless    bool
	UserDataDir string
	Timeout     time.Duration
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
)

func LoadConfig() (ConfigResult, error) {
	envLookup := runtimeconfig.DefaultEnvLookup

	storePath := configadmin.ResolveStorePath(envLookup)
	cacheTTL := 30 * time.Second
	if ttlValue, ok := envLookup("CONFIG_ADMIN_CACHE_TTL"); ok && strings.TrimSpace(ttlValue) != "" {
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
			Retention:                        30 * 24 * time.Hour,
			MaxSessions:                      100,
			SessionTTL:                       1 * time.Hour,
			MaxEvents:                        1000,
			AsyncBatchSize:                   200,
			AsyncFlushInterval:               250 * time.Millisecond,
			AsyncAppendTimeout:               50 * time.Millisecond,
			AsyncQueueCapacity:               8192,
			AsyncFlushRequestCoalesceWindow:  8 * time.Millisecond,
			AsyncBackpressureHighWatermark:   (8192 * 80) / 100,
			DegradeDebugEventsOnBackpressure: true,
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
					ToolPreset:    string(presets.ToolPresetLarkLocal),
					ReplyTimeout:  3 * time.Minute,
				},
				BaseDomain:         "https://open.larkoffice.com",
				ToolMode:           "cli",
				AutoUploadFiles:    true,
				AutoUploadMaxBytes: 2 * 1024 * 1024,
				AutoUploadAllowExt: []string{".txt", ".md", ".json", ".yaml", ".yml", ".csv", ".log", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".pdf", ".docx", ".xlsx", ".pptx"},
				Browser: LarkBrowserConfig{
					Headless: true,
					Timeout:  60 * time.Second,
				},
				ReactEmoji:                  "WAVE, Get, THINKING, MUSCLE, THUMBSUP, OK, THANKS, APPLAUSE, LGTM",
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
	if err := validateLarkPersistenceConfig(&cfg); err != nil {
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
	if larkCfg.Enabled != nil {
		cfg.Channels.Lark.Enabled = *larkCfg.Enabled
	}
	if appID := strings.TrimSpace(larkCfg.AppID); appID != "" {
		cfg.Channels.Lark.AppID = appID
	}
	if appSecret := strings.TrimSpace(larkCfg.AppSecret); appSecret != "" {
		cfg.Channels.Lark.AppSecret = appSecret
	}
	if calendarID := strings.TrimSpace(larkCfg.TenantCalendarID); calendarID != "" {
		cfg.Channels.Lark.TenantCalendarID = calendarID
	}
	if baseDomain := strings.TrimSpace(larkCfg.BaseDomain); baseDomain != "" {
		cfg.Channels.Lark.BaseDomain = baseDomain
	}
	if workspaceDir := strings.TrimSpace(larkCfg.WorkspaceDir); workspaceDir != "" {
		cfg.Channels.Lark.WorkspaceDir = workspaceDir
	}
	if larkCfg.AutoUploadFiles != nil {
		cfg.Channels.Lark.AutoUploadFiles = *larkCfg.AutoUploadFiles
	}
	if larkCfg.AutoUploadMaxBytes != nil && *larkCfg.AutoUploadMaxBytes > 0 {
		cfg.Channels.Lark.AutoUploadMaxBytes = *larkCfg.AutoUploadMaxBytes
	}
	if len(larkCfg.AutoUploadAllowExt) > 0 {
		cfg.Channels.Lark.AutoUploadAllowExt = append([]string(nil), larkCfg.AutoUploadAllowExt...)
	}
	if larkCfg.Browser != nil {
		if cdpURL := strings.TrimSpace(larkCfg.Browser.CDPURL); cdpURL != "" {
			cfg.Channels.Lark.Browser.CDPURL = cdpURL
		}
		if chromePath := strings.TrimSpace(larkCfg.Browser.ChromePath); chromePath != "" {
			cfg.Channels.Lark.Browser.ChromePath = chromePath
		}
		if larkCfg.Browser.Headless != nil {
			cfg.Channels.Lark.Browser.Headless = *larkCfg.Browser.Headless
		}
		if userDataDir := strings.TrimSpace(larkCfg.Browser.UserDataDir); userDataDir != "" {
			cfg.Channels.Lark.Browser.UserDataDir = userDataDir
		}
		if larkCfg.Browser.TimeoutSeconds != nil && *larkCfg.Browser.TimeoutSeconds > 0 {
			cfg.Channels.Lark.Browser.Timeout = time.Duration(*larkCfg.Browser.TimeoutSeconds) * time.Second
		}
	}
	if prefix := strings.TrimSpace(larkCfg.SessionPrefix); prefix != "" {
		cfg.Channels.Lark.SessionPrefix = prefix
	}
	if replyPrefix := strings.TrimSpace(larkCfg.ReplyPrefix); replyPrefix != "" {
		cfg.Channels.Lark.ReplyPrefix = replyPrefix
	}
	if larkCfg.AllowGroups != nil {
		cfg.Channels.Lark.AllowGroups = *larkCfg.AllowGroups
	}
	if larkCfg.AllowDirect != nil {
		cfg.Channels.Lark.AllowDirect = *larkCfg.AllowDirect
	}
	if agentPreset := strings.TrimSpace(larkCfg.AgentPreset); agentPreset != "" {
		cfg.Channels.Lark.AgentPreset = agentPreset
	}
	if toolPreset := strings.TrimSpace(larkCfg.ToolPreset); toolPreset != "" {
		cfg.Channels.Lark.ToolPreset = toolPreset
	}
	if toolMode := strings.TrimSpace(larkCfg.ToolMode); toolMode != "" {
		cfg.Channels.Lark.ToolMode = toolMode
	}
	if larkCfg.ReplyTimeoutSeconds != nil && *larkCfg.ReplyTimeoutSeconds > 0 {
		cfg.Channels.Lark.ReplyTimeout = time.Duration(*larkCfg.ReplyTimeoutSeconds) * time.Second
	}
	if reactEmoji := strings.TrimSpace(larkCfg.ReactEmoji); reactEmoji != "" {
		cfg.Channels.Lark.ReactEmoji = reactEmoji
	}
	if emoji := strings.TrimSpace(larkCfg.InjectionAckReactEmoji); emoji != "" {
		cfg.Channels.Lark.InjectionAckReactEmoji = emoji
	}
	if emoji := strings.TrimSpace(larkCfg.FinalAnswerReviewReactEmoji); emoji != "" {
		cfg.Channels.Lark.FinalAnswerReviewReactEmoji = emoji
	}
	if larkCfg.MemoryEnabled != nil {
		cfg.Channels.Lark.MemoryEnabled = *larkCfg.MemoryEnabled
	}
	if larkCfg.ShowToolProgress != nil {
		cfg.Channels.Lark.ShowToolProgress = *larkCfg.ShowToolProgress
	}
	if larkCfg.ShowPlanClarifyMessages != nil {
		cfg.Channels.Lark.ShowPlanClarifyMessages = *larkCfg.ShowPlanClarifyMessages
	}
	if larkCfg.AutoChatContextSize != nil && *larkCfg.AutoChatContextSize > 0 {
		cfg.Channels.Lark.AutoChatContextSize = *larkCfg.AutoChatContextSize
	}
	if larkCfg.PlanReviewEnabled != nil {
		cfg.Channels.Lark.PlanReviewEnabled = *larkCfg.PlanReviewEnabled
	}
	if larkCfg.PlanReviewRequireConfirmation != nil {
		cfg.Channels.Lark.PlanReviewRequireConfirmation = *larkCfg.PlanReviewRequireConfirmation
	}
	if larkCfg.PlanReviewPendingTTLMinutes != nil && *larkCfg.PlanReviewPendingTTLMinutes > 0 {
		cfg.Channels.Lark.PlanReviewPendingTTL = time.Duration(*larkCfg.PlanReviewPendingTTLMinutes) * time.Minute
	}
	if larkCfg.ActiveSlotTTLMinutes != nil && *larkCfg.ActiveSlotTTLMinutes > 0 {
		cfg.Channels.Lark.ActiveSlotTTL = time.Duration(*larkCfg.ActiveSlotTTLMinutes) * time.Minute
	}
	if larkCfg.ActiveSlotMaxEntries != nil && *larkCfg.ActiveSlotMaxEntries > 0 {
		cfg.Channels.Lark.ActiveSlotMaxEntries = *larkCfg.ActiveSlotMaxEntries
	}
	if larkCfg.PendingInputRelayTTLMinutes != nil && *larkCfg.PendingInputRelayTTLMinutes > 0 {
		cfg.Channels.Lark.PendingInputRelayTTL = time.Duration(*larkCfg.PendingInputRelayTTLMinutes) * time.Minute
	}
	if larkCfg.PendingInputRelayMaxChats != nil && *larkCfg.PendingInputRelayMaxChats > 0 {
		cfg.Channels.Lark.PendingInputRelayMaxChats = *larkCfg.PendingInputRelayMaxChats
	}
	if larkCfg.PendingInputRelayMaxPerChat != nil && *larkCfg.PendingInputRelayMaxPerChat > 0 {
		cfg.Channels.Lark.PendingInputRelayMaxPerChat = *larkCfg.PendingInputRelayMaxPerChat
	}
	if larkCfg.AIChatSessionTTLMinutes != nil && *larkCfg.AIChatSessionTTLMinutes > 0 {
		cfg.Channels.Lark.AIChatSessionTTL = time.Duration(*larkCfg.AIChatSessionTTLMinutes) * time.Minute
	}
	if larkCfg.StateCleanupIntervalSeconds != nil && *larkCfg.StateCleanupIntervalSeconds > 0 {
		cfg.Channels.Lark.StateCleanupInterval = time.Duration(*larkCfg.StateCleanupIntervalSeconds) * time.Second
	}
	if larkCfg.Persistence != nil {
		if mode := strings.TrimSpace(strings.ToLower(larkCfg.Persistence.Mode)); mode != "" {
			cfg.Channels.Lark.PersistenceMode = mode
		}
		if dir := strings.TrimSpace(larkCfg.Persistence.Dir); dir != "" {
			cfg.Channels.Lark.PersistenceDir = dir
		}
		if larkCfg.Persistence.RetentionHours != nil && *larkCfg.Persistence.RetentionHours > 0 {
			cfg.Channels.Lark.PersistenceRetention = time.Duration(*larkCfg.Persistence.RetentionHours) * time.Hour
		}
		if larkCfg.Persistence.MaxTasksPerChat != nil && *larkCfg.Persistence.MaxTasksPerChat > 0 {
			cfg.Channels.Lark.PersistenceMaxTasksPerChat = *larkCfg.Persistence.MaxTasksPerChat
		}
	}
	if larkCfg.MaxConcurrentTasks != nil && *larkCfg.MaxConcurrentTasks > 0 {
		cfg.Channels.Lark.MaxConcurrentTasks = *larkCfg.MaxConcurrentTasks
	}
	if larkCfg.DefaultPlanMode != nil {
		cfg.Channels.Lark.DefaultPlanMode = strings.TrimSpace(*larkCfg.DefaultPlanMode)
	}
}

func validateLarkPersistenceConfig(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	mode := strings.TrimSpace(strings.ToLower(cfg.Channels.Lark.PersistenceMode))
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
	if file.Server.EventHistoryAsyncBatchSize != nil && *file.Server.EventHistoryAsyncBatchSize > 0 {
		cfg.EventHistory.AsyncBatchSize = *file.Server.EventHistoryAsyncBatchSize
	}
	if file.Server.EventHistoryAsyncFlushMS != nil && *file.Server.EventHistoryAsyncFlushMS > 0 {
		cfg.EventHistory.AsyncFlushInterval = time.Duration(*file.Server.EventHistoryAsyncFlushMS) * time.Millisecond
	}
	if file.Server.EventHistoryAsyncAppendMS != nil && *file.Server.EventHistoryAsyncAppendMS > 0 {
		cfg.EventHistory.AsyncAppendTimeout = time.Duration(*file.Server.EventHistoryAsyncAppendMS) * time.Millisecond
	}
	if file.Server.EventHistoryAsyncQueueSize != nil && *file.Server.EventHistoryAsyncQueueSize > 0 {
		cfg.EventHistory.AsyncQueueCapacity = *file.Server.EventHistoryAsyncQueueSize
		if file.Server.EventHistoryAsyncBackpressureHighWatermark == nil {
			cfg.EventHistory.AsyncBackpressureHighWatermark = (*file.Server.EventHistoryAsyncQueueSize * 80) / 100
		}
	}
	if file.Server.EventHistoryAsyncFlushRequestCoalesceMS != nil {
		if *file.Server.EventHistoryAsyncFlushRequestCoalesceMS <= 0 {
			cfg.EventHistory.AsyncFlushRequestCoalesceWindow = 0
		} else {
			cfg.EventHistory.AsyncFlushRequestCoalesceWindow = time.Duration(*file.Server.EventHistoryAsyncFlushRequestCoalesceMS) * time.Millisecond
		}
	}
	if file.Server.EventHistoryAsyncBackpressureHighWatermark != nil {
		cfg.EventHistory.AsyncBackpressureHighWatermark = *file.Server.EventHistoryAsyncBackpressureHighWatermark
	}
	if file.Server.EventHistoryDegradeDebugEventsOnBackpressure != nil {
		cfg.EventHistory.DegradeDebugEventsOnBackpressure = *file.Server.EventHistoryDegradeDebugEventsOnBackpressure
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
	if values == nil {
		return nil
	}
	origins := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		origin := strings.TrimSpace(value)
		if origin == "" {
			continue
		}
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	return origins
}
