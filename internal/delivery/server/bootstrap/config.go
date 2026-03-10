package bootstrap

import (
	"context"
	"time"

	"alex/internal/delivery/channels"
	"alex/internal/delivery/channels/lark"
	"alex/internal/infra/attachments"
	runtimeconfig "alex/internal/shared/config"
	configadmin "alex/internal/shared/config/admin"
)

// Config holds server configuration.
type Config struct {
	Runtime            runtimeconfig.RuntimeConfig
	RuntimeMeta        runtimeconfig.Metadata
	Port               string
	DebugPort          string // Debug HTTP port for Lark standalone mode (default "9090")
	DebugBindHost      string // Network interface for debug server (default "127.0.0.1")
	LogDir             string // Structured log / watchdog dump directory (default "logs")
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
	LeaderAPIToken     string
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

// ChannelsConfig captures server-side channel gateways via a plugin registry.
// Individual channel configs are stored inside the registry and accessed
// through typed helper methods (LarkConfig, TelegramConfig).
type ChannelsConfig struct {
	Registry *ChannelRegistry
}

// LarkConfig returns the resolved Lark gateway configuration from the
// channel registry. Returns a zero value if not registered.
func (c ChannelsConfig) LarkConfig() LarkGatewayConfig {
	if c.Registry == nil {
		return LarkGatewayConfig{}
	}
	if v, ok := c.Registry.Config("lark"); ok {
		if cfg, ok := v.(LarkGatewayConfig); ok {
			return cfg
		}
	}
	return LarkGatewayConfig{}
}

// SetLarkConfig stores or replaces the Lark gateway configuration in the
// channel registry.
func (c ChannelsConfig) SetLarkConfig(cfg LarkGatewayConfig) {
	if c.Registry != nil {
		c.Registry.SetConfig("lark", cfg)
	}
}

// TelegramConfig returns the resolved Telegram gateway configuration from the
// channel registry. Returns a zero value if not registered.
func (c ChannelsConfig) TelegramConfig() TelegramGatewayConfig {
	if c.Registry == nil {
		return TelegramGatewayConfig{}
	}
	if v, ok := c.Registry.Config("telegram"); ok {
		if cfg, ok := v.(TelegramGatewayConfig); ok {
			return cfg
		}
	}
	return TelegramGatewayConfig{}
}

// SetTelegramConfig stores or replaces the Telegram gateway configuration in
// the channel registry.
func (c ChannelsConfig) SetTelegramConfig(cfg TelegramGatewayConfig) {
	if c.Registry != nil {
		c.Registry.SetConfig("telegram", cfg)
	}
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
	AttentionGate                 lark.AttentionGateConfig
	RateLimiterEnabled            bool
	RateLimiterChatHourlyLimit    int
	RateLimiterUserDailyLimit     int
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
