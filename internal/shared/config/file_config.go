package config

import (
	"time"

	toolspolicy "alex/internal/infra/tools"
)

// FileConfig captures the on-disk YAML configuration sections.
type FileConfig struct {
	Runtime     *RuntimeFileConfig `json:"runtime,omitempty" yaml:"runtime"`
	Overrides   *Overrides         `json:"overrides,omitempty" yaml:"overrides"`
	Apps        *AppsConfig        `json:"apps,omitempty" yaml:"apps"`
	Channels    *ChannelsConfig    `json:"channels,omitempty" yaml:"channels"`
	Server      *ServerConfig      `json:"server,omitempty" yaml:"server"`
	Agent       *AgentConfig       `json:"agent,omitempty" yaml:"agent"`
	Session     *SessionConfig     `json:"session,omitempty" yaml:"session"`
	Analytics   *AnalyticsConfig   `json:"analytics,omitempty" yaml:"analytics"`
	Attachments *AttachmentsConfig `json:"attachments,omitempty" yaml:"attachments"`
	Web         *WebConfig         `json:"web,omitempty" yaml:"web"`
}

// RuntimeFileConfig mirrors RuntimeConfig for YAML decoding (runtime section).
type RuntimeFileConfig struct {
	LLMProvider                string                    `yaml:"llm_provider"`
	LLMModel                   string                    `yaml:"llm_model"`
	LLMVisionModel             string                    `yaml:"llm_vision_model"`
	APIKey                     string                    `yaml:"api_key"`
	ArkAPIKey                  string                    `yaml:"ark_api_key"`
	BaseURL                    string                    `yaml:"base_url"`
	TavilyAPIKey               string                    `yaml:"tavily_api_key"`
	MoltbookAPIKey             string                    `yaml:"moltbook_api_key"`
	MoltbookBaseURL            string                    `yaml:"moltbook_base_url"`
	Profile                    string                    `yaml:"profile"`
	Environment                string                    `yaml:"environment"`
	Verbose                    *bool                     `yaml:"verbose"`
	DisableTUI                 *bool                     `yaml:"disable_tui"`
	FollowTranscript           *bool                     `yaml:"follow_transcript"`
	FollowStream               *bool                     `yaml:"follow_stream"`
	MaxIterations              *int                      `yaml:"max_iterations"`
	MaxTokens                  *int                      `yaml:"max_tokens"`
	ToolMaxConcurrent          *int                      `yaml:"tool_max_concurrent"`
	LLMCacheSize               *int                      `yaml:"llm_cache_size"`
	LLMCacheTTLSeconds         *int                      `yaml:"llm_cache_ttl_seconds"`
	LLMRequestTimeoutSeconds   *int                      `yaml:"llm_request_timeout_seconds"`
	UserRateLimitRPS           *float64                  `yaml:"user_rate_limit_rps"`
	UserRateLimitBurst         *int                      `yaml:"user_rate_limit_burst"`
	KimiRateLimitRPS           *float64                  `yaml:"kimi_rate_limit_rps"`
	KimiRateLimitBurst         *int                      `yaml:"kimi_rate_limit_burst"`
	Temperature                *float64                  `yaml:"temperature"`
	TopP                       *float64                  `yaml:"top_p"`
	StopSequences              []string                  `yaml:"stop_sequences"`
	SessionDir                 string                    `yaml:"session_dir"`
	CostDir                    string                    `yaml:"cost_dir"`
	SessionStaleAfter          string                    `yaml:"session_stale_after"`
	AgentPreset                string                    `yaml:"agent_preset"`
	ToolPreset                 string                    `yaml:"tool_preset"`
	Toolset                    string                    `yaml:"toolset"`
	Browser                    *RuntimeBrowserConfig     `yaml:"browser"`
	ToolPolicy                 *ToolPolicyFileConfig     `yaml:"tool_policy"`
	HTTPLimits                 *HTTPLimitsFileConfig     `yaml:"http_limits"`
	Proactive                  *ProactiveFileConfig      `yaml:"proactive"`
	ExternalAgents             *ExternalAgentsFileConfig `yaml:"external_agents"`
}

// RuntimeBrowserConfig captures local browser settings in YAML (runtime section).
type RuntimeBrowserConfig struct {
	Connector      string `json:"connector" yaml:"connector"`
	CDPURL         string `json:"cdp_url" yaml:"cdp_url"`
	ChromePath     string `json:"chrome_path" yaml:"chrome_path"`
	Headless       *bool  `json:"headless" yaml:"headless"`
	UserDataDir    string `json:"user_data_dir" yaml:"user_data_dir"`
	TimeoutSeconds *int   `json:"timeout_seconds" yaml:"timeout_seconds"`
	BridgeListen   string `json:"bridge_listen_addr" yaml:"bridge_listen_addr"`
	BridgeToken    string `json:"bridge_token" yaml:"bridge_token"`
}

// ToolPolicyFileConfig mirrors ToolPolicyConfig for YAML decoding with partial overrides.
type ToolPolicyFileConfig struct {
	EnforcementMode string                   `yaml:"enforcement_mode"`
	Timeout         *ToolTimeoutFileConfig   `yaml:"timeout"`
	Retry           *ToolRetryFileConfig     `yaml:"retry"`
	Rules           []toolspolicy.PolicyRule `yaml:"rules,omitempty"`
}

// ToolTimeoutFileConfig mirrors ToolTimeoutConfig for YAML decoding.
type ToolTimeoutFileConfig struct {
	Default *time.Duration           `yaml:"default"`
	PerTool map[string]time.Duration `yaml:"per_tool"`
}

// ToolRetryFileConfig mirrors ToolRetryConfig for YAML decoding.
type ToolRetryFileConfig struct {
	MaxRetries     *int           `yaml:"max_retries"`
	InitialBackoff *time.Duration `yaml:"initial_backoff"`
	MaxBackoff     *time.Duration `yaml:"max_backoff"`
	BackoffFactor  *float64       `yaml:"backoff_factor"`
}

// HTTPLimitsFileConfig mirrors HTTPLimitsConfig for YAML decoding.
type HTTPLimitsFileConfig struct {
	DefaultMaxResponseBytes     *int `yaml:"default_max_response_bytes"`
	WebFetchMaxResponseBytes    *int `yaml:"web_fetch_max_response_bytes"`
	WebSearchMaxResponseBytes *int `yaml:"web_search_max_response_bytes"`
	ModelListMaxResponseBytes *int `yaml:"model_list_max_response_bytes"`
}

// ExternalAgentsFileConfig mirrors ExternalAgentsConfig for YAML decoding.
type ExternalAgentsFileConfig struct {
	MaxParallelAgents *int                  `yaml:"max_parallel_agents"`
	ClaudeCode        *ClaudeCodeFileConfig `yaml:"claude_code"`
	Codex             *CLIAgentFileConfig   `yaml:"codex"`
	Kimi              *CLIAgentFileConfig   `yaml:"kimi"`
	Teams             []TeamFileConfig      `yaml:"teams"`
}

type ClaudeCodeFileConfig struct {
	Enabled                *bool             `yaml:"enabled"`
	Binary                 string            `yaml:"binary"`
	DefaultModel           string            `yaml:"default_model"`
	DefaultMode            string            `yaml:"default_mode"`
	AutonomousAllowedTools []string          `yaml:"autonomous_allowed_tools"`
	PlanAllowedTools       []string          `yaml:"plan_allowed_tools"`
	MaxBudgetUSD           *float64          `yaml:"max_budget_usd"`
	MaxTurns               *int              `yaml:"max_turns"`
	Timeout                string            `yaml:"timeout"`
	ResumeEnabled          *bool             `yaml:"resume_enabled"`
	Env                    map[string]string `yaml:"env"`
}

type CLIAgentFileConfig struct {
	Enabled            *bool             `yaml:"enabled"`
	Binary             string            `yaml:"binary"`
	DefaultModel       string            `yaml:"default_model"`
	ApprovalPolicy     string            `yaml:"approval_policy"`
	Sandbox            string            `yaml:"sandbox"`
	PlanApprovalPolicy string            `yaml:"plan_approval_policy"`
	PlanSandbox        string            `yaml:"plan_sandbox"`
	Timeout            string            `yaml:"timeout"`
	ResumeEnabled      *bool             `yaml:"resume_enabled"`
	Env                map[string]string `yaml:"env"`
}

type CodexFileConfig = CLIAgentFileConfig
type KimiFileConfig = CLIAgentFileConfig

// TeamFileConfig mirrors TeamConfig for YAML decoding.
type TeamFileConfig struct {
	Name        string                `yaml:"name"`
	Description string                `yaml:"description"`
	Roles       []TeamRoleFileConfig  `yaml:"roles"`
	Stages      []TeamStageFileConfig `yaml:"stages"`
}

// TeamRoleFileConfig mirrors TeamRoleConfig for YAML decoding.
type TeamRoleFileConfig struct {
	Name              string            `yaml:"name"`
	AgentType         string            `yaml:"agent_type"`
	CapabilityProfile string            `yaml:"capability_profile"`
	TargetCLI         string            `yaml:"target_cli"`
	PromptTemplate    string            `yaml:"prompt_template"`
	ExecutionMode     string            `yaml:"execution_mode"`
	AutonomyLevel     string            `yaml:"autonomy_level"`
	WorkspaceMode     string            `yaml:"workspace_mode"`
	Config            map[string]string `yaml:"config"`
	InheritContext    *bool             `yaml:"inherit_context"`
}

// TeamStageFileConfig mirrors TeamStageConfig for YAML decoding.
type TeamStageFileConfig struct {
	Name  string   `yaml:"name"`
	Roles []string `yaml:"roles"`
}

// ProactiveFileConfig mirrors ProactiveConfig for YAML decoding.
type ProactiveFileConfig struct {
	Enabled   *bool                `yaml:"enabled"`
	Prompt    *PromptFileConfig    `yaml:"prompt"`
	Memory    *MemoryFileConfig    `yaml:"memory"`
	Skills    *SkillsFileConfig    `yaml:"skills"`
	OKR       *OKRFileConfig       `yaml:"okr"`
	Scheduler *SchedulerFileConfig `yaml:"scheduler"`
	Timer     *TimerFileConfig     `yaml:"timer"`
	Attention *AttentionFileConfig `yaml:"attention"`
}

type PromptFileConfig struct {
	Mode              string   `yaml:"mode"`
	Timezone          string   `yaml:"timezone"`
	BootstrapMaxChars *int     `yaml:"bootstrap_max_chars"`
	BootstrapFiles    []string `yaml:"bootstrap_files"`
	ReplyTagsEnabled  *bool    `yaml:"reply_tags_enabled"`
}

// OKRFileConfig mirrors OKRProactiveConfig for YAML decoding.
type OKRFileConfig struct {
	Enabled    *bool  `yaml:"enabled"`
	GoalsRoot  string `yaml:"goals_root"`
	AutoInject *bool  `yaml:"auto_inject"`
}

type MemoryFileConfig struct {
	Enabled          *bool                  `yaml:"enabled"`
	Index            *MemoryIndexFileConfig `yaml:"index"`
	ArchiveAfterDays *int                   `yaml:"archive_after_days"`
	CleanupInterval  string                 `yaml:"cleanup_interval"`
}

type MemoryIndexFileConfig struct {
	Enabled            *bool    `yaml:"enabled"`
	DBPath             string   `yaml:"db_path"`
	ChunkTokens        *int     `yaml:"chunk_tokens"`
	ChunkOverlap       *int     `yaml:"chunk_overlap"`
	MinScore           *float64 `yaml:"min_score"`
	FusionWeightVector *float64 `yaml:"fusion_weight_vector"`
	FusionWeightBM25   *float64 `yaml:"fusion_weight_bm25"`
	EmbedderModel      string   `yaml:"embedder_model"`
}

type SkillsFileConfig struct {
	AutoActivation           *SkillsAutoActivationFileConfig `yaml:"auto_activation"`
	Feedback                 *SkillsFeedbackFileConfig       `yaml:"feedback"`
	CacheTTLSeconds          *int                            `yaml:"cache_ttl_seconds"`
	MetaOrchestratorEnabled  *bool                           `yaml:"meta_orchestrator_enabled"`
	SoulAutoEvolutionEnabled *bool                           `yaml:"soul_auto_evolution_enabled"`
	ProactiveLevel           string                          `yaml:"proactive_level"`
	PolicyPath               string                          `yaml:"policy_path"`
}

type SkillsAutoActivationFileConfig struct {
	Enabled             *bool    `yaml:"enabled"`
	MaxActivated        *int     `yaml:"max_activated"`
	TokenBudget         *int     `yaml:"token_budget"`
	ConfidenceThreshold *float64 `yaml:"confidence_threshold"`
}

type SkillsFeedbackFileConfig struct {
	Enabled   *bool  `yaml:"enabled"`
	StorePath string `yaml:"store_path"`
}

type SchedulerFileConfig struct {
	Enabled                          *bool                        `yaml:"enabled"`
	Triggers                         []SchedulerTriggerFileConfig `yaml:"triggers"`
	CalendarReminder                 *CalendarReminderFileConfig  `yaml:"calendar_reminder"`
	Heartbeat                        *HeartbeatFileConfig         `yaml:"heartbeat"`
	MilestoneCheckin                 *MilestoneCheckinFileConfig  `yaml:"milestone_checkin"`
	WeeklyPulse                      *WeeklyPulseFileConfig       `yaml:"weekly_pulse"`
	BlockerRadar                     *BlockerRadarFileConfig      `yaml:"blocker_radar"`
	TriggerTimeoutSeconds            *int                         `yaml:"trigger_timeout_seconds"`
	ConcurrencyPolicy                string                       `yaml:"concurrency_policy"`
	LeaderLockEnabled                *bool                        `yaml:"leader_lock_enabled"`
	LeaderLockName                   string                       `yaml:"leader_lock_name"`
	LeaderLockAcquireIntervalSeconds *int                         `yaml:"leader_lock_acquire_interval_seconds"`
	JobStorePath                     string                       `yaml:"job_store_path"`
	CooldownSeconds                  *int                         `yaml:"cooldown_seconds"`
	MaxConcurrent                    *int                         `yaml:"max_concurrent"`
	RecoveryMaxRetries               *int                         `yaml:"recovery_max_retries"`
	RecoveryBackoffSeconds           *int                         `yaml:"recovery_backoff_seconds"`
}

type SchedulerTriggerFileConfig struct {
	Name     string `yaml:"name"`
	Schedule string `yaml:"schedule"`
	Task     string `yaml:"task"`
	Channel  string `yaml:"channel"`
	UserID   string `yaml:"user_id"`
	ChatID   string `yaml:"chat_id"`
}

type CalendarReminderFileConfig struct {
	Enabled          *bool  `yaml:"enabled"`
	Schedule         string `yaml:"schedule"`
	LookAheadMinutes *int   `yaml:"look_ahead_minutes"`
	Channel          string `yaml:"channel"`
	UserID           string `yaml:"user_id"`
	ChatID           string `yaml:"chat_id"`
}

type HeartbeatFileConfig struct {
	Enabled          *bool  `yaml:"enabled"`
	Schedule         string `yaml:"schedule"`
	Task             string `yaml:"task"`
	Channel          string `yaml:"channel"`
	UserID           string `yaml:"user_id"`
	ChatID           string `yaml:"chat_id"`
	QuietHours       []int  `yaml:"quiet_hours"`
	WindowLookbackHr *int   `yaml:"window_lookback_hours"`
}

type MilestoneCheckinFileConfig struct {
	Enabled          *bool  `yaml:"enabled"`
	Schedule         string `yaml:"schedule"`
	LookbackSeconds  *int   `yaml:"lookback_seconds"`
	Channel          string `yaml:"channel"`
	ChatID           string `yaml:"chat_id"`
	IncludeActive    *bool  `yaml:"include_active"`
	IncludeCompleted *bool  `yaml:"include_completed"`
}

type WeeklyPulseFileConfig struct {
	Enabled  *bool  `yaml:"enabled"`
	Schedule string `yaml:"schedule"`
	Channel  string `yaml:"channel"`
	ChatID   string `yaml:"chat_id"`
}

type BlockerRadarFileConfig struct {
	Enabled               *bool  `yaml:"enabled"`
	Schedule              string `yaml:"schedule"`
	StaleThresholdSeconds *int   `yaml:"stale_threshold_seconds"`
	InputWaitSeconds      *int   `yaml:"input_wait_seconds"`
	Channel               string `yaml:"channel"`
	ChatID                string `yaml:"chat_id"`
}

type TimerFileConfig struct {
	Enabled            *bool  `yaml:"enabled"`
	StorePath          string `yaml:"store_path"`
	MaxTimers          *int   `yaml:"max_timers"`
	TaskTimeoutSeconds *int   `yaml:"task_timeout_seconds"`
	HeartbeatEnabled   *bool  `yaml:"heartbeat_enabled"`
	HeartbeatMinutes   *int   `yaml:"heartbeat_minutes"`
}

type AttentionFileConfig struct {
	MaxDailyNotifications *int     `yaml:"max_daily_notifications"`
	MinIntervalSeconds    *int     `yaml:"min_interval_seconds"`
	QuietHours            []int    `yaml:"quiet_hours"`
	PriorityThreshold     *float64 `yaml:"priority_threshold"`
}

// AppsConfig captures user-managed app plugin connectors.
type AppsConfig struct {
	Plugins []AppPluginConfig `json:"plugins" yaml:"plugins"`
}

// AppPluginConfig describes a custom app plugin entry.
type AppPluginConfig struct {
	ID              string   `json:"id" yaml:"id"`
	Name            string   `json:"name" yaml:"name"`
	Description     string   `json:"description" yaml:"description"`
	Capabilities    []string `json:"capabilities,omitempty" yaml:"capabilities"`
	IntegrationNote string   `json:"integration_note,omitempty" yaml:"integration_note"`
	Sources         []string `json:"sources,omitempty" yaml:"sources"`
}

// ChannelsConfig captures channel integrations (e.g., Lark, Telegram).
type ChannelsConfig struct {
	Lark     *LarkChannelConfig     `json:"lark,omitempty" yaml:"lark"`
	Telegram *TelegramChannelConfig `json:"telegram,omitempty" yaml:"telegram"`
}

// BaseChannelConfig captures fields shared by Lark and Telegram channel configs.
type BaseChannelConfig struct {
	Enabled                       *bool  `json:"enabled,omitempty" yaml:"enabled"`
	SessionPrefix                 string `json:"session_prefix,omitempty" yaml:"session_prefix"`
	ReplyPrefix                   string `json:"reply_prefix,omitempty" yaml:"reply_prefix"`
	AllowGroups                   *bool  `json:"allow_groups,omitempty" yaml:"allow_groups"`
	AllowDirect                   *bool  `json:"allow_direct,omitempty" yaml:"allow_direct"`
	AgentPreset                   string `json:"agent_preset,omitempty" yaml:"agent_preset"`
	ToolPreset                    string `json:"tool_preset,omitempty" yaml:"tool_preset"`
	ReplyTimeoutSeconds           *int   `json:"reply_timeout_seconds,omitempty" yaml:"reply_timeout_seconds"`
	MemoryEnabled                 *bool  `json:"memory_enabled,omitempty" yaml:"memory_enabled"`
	ShowToolProgress              *bool  `json:"show_tool_progress,omitempty" yaml:"show_tool_progress"`
	SlowProgressSummaryEnabled    *bool  `json:"slow_progress_summary_enabled,omitempty" yaml:"slow_progress_summary_enabled"`
	SlowProgressSummaryDelaySecs  *int   `json:"slow_progress_summary_delay_seconds,omitempty" yaml:"slow_progress_summary_delay_seconds"`
	PlanReviewEnabled             *bool  `json:"plan_review_enabled,omitempty" yaml:"plan_review_enabled"`
	PlanReviewRequireConfirmation *bool  `json:"plan_review_require_confirmation,omitempty" yaml:"plan_review_require_confirmation"`
	PlanReviewPendingTTLMinutes   *int   `json:"plan_review_pending_ttl_minutes,omitempty" yaml:"plan_review_pending_ttl_minutes"`
	ActiveSlotTTLMinutes          *int   `json:"active_slot_ttl_minutes,omitempty" yaml:"active_slot_ttl_minutes"`
	ActiveSlotMaxEntries          *int   `json:"active_slot_max_entries,omitempty" yaml:"active_slot_max_entries"`
	StateCleanupIntervalSeconds   *int   `json:"state_cleanup_interval_seconds,omitempty" yaml:"state_cleanup_interval_seconds"`
	MaxConcurrentTasks            *int   `json:"max_concurrent_tasks,omitempty" yaml:"max_concurrent_tasks"`
}

// TelegramChannelConfig captures Telegram gateway settings in YAML.
type TelegramChannelConfig struct {
	BotToken          string                     `yaml:"bot_token"`
	AllowedGroups     []int64                    `yaml:"allowed_groups"`
	Persistence       *TelegramPersistenceConfig `yaml:"persistence"`
	BaseChannelConfig `json:",inline" yaml:",inline"`
}

// TelegramPersistenceConfig captures Telegram local persistence settings.
type TelegramPersistenceConfig struct {
	Mode            string `yaml:"mode"`
	Dir             string `yaml:"dir"`
	RetentionHours  *int   `yaml:"retention_hours"`
	MaxTasksPerChat *int   `yaml:"max_tasks_per_chat"`
}

// LarkChannelConfig captures Lark gateway settings in YAML.
type LarkChannelConfig struct {
	AppID                       string                 `json:"app_id" yaml:"app_id"`
	AppSecret                   string                 `json:"app_secret" yaml:"app_secret"`
	TenantCalendarID            string                 `json:"tenant_calendar_id" yaml:"tenant_calendar_id"`
	BaseDomain                  string                 `json:"base_domain" yaml:"base_domain"`
	WorkspaceDir                string                 `json:"workspace_dir" yaml:"workspace_dir"`
	AutoUploadFiles             *bool                  `json:"auto_upload_files" yaml:"auto_upload_files"`
	AutoUploadMaxBytes          *int                   `json:"auto_upload_max_bytes" yaml:"auto_upload_max_bytes"`
	AutoUploadAllowExt          []string               `json:"auto_upload_allow_ext" yaml:"auto_upload_allow_ext"`
	Browser                     *LarkBrowserConfig     `json:"browser" yaml:"browser"`
	ToolMode                    string                 `json:"tool_mode" yaml:"tool_mode"`
	InjectionAckReactEmoji      string                 `json:"injection_ack_react_emoji" yaml:"injection_ack_react_emoji"`
	ShowPlanClarifyMessages     *bool                  `json:"show_plan_clarify_messages" yaml:"show_plan_clarify_messages"`
	ToolFailureAbortThreshold   *int                   `json:"tool_failure_abort_threshold" yaml:"tool_failure_abort_threshold"`
	AutoChatContextSize         *int                   `json:"auto_chat_context_size" yaml:"auto_chat_context_size"`
	PendingInputRelayTTLMinutes *int                   `json:"pending_input_relay_ttl_minutes" yaml:"pending_input_relay_ttl_minutes"`
	PendingInputRelayMaxChats   *int                   `json:"pending_input_relay_max_chats" yaml:"pending_input_relay_max_chats"`
	PendingInputRelayMaxPerChat *int                   `json:"pending_input_relay_max_per_chat" yaml:"pending_input_relay_max_per_chat"`
	AIChatSessionTTLMinutes     *int                   `json:"ai_chat_session_ttl_minutes" yaml:"ai_chat_session_ttl_minutes"`
	Persistence                 *LarkPersistenceConfig `json:"persistence" yaml:"persistence"`
	Delivery                    *LarkDeliveryConfig    `json:"delivery" yaml:"delivery"`
	RateLimiter                 *LarkRateLimiterConfig `json:"rate_limiter" yaml:"rate_limiter"`
	DefaultPlanMode             *string                `json:"default_plan_mode" yaml:"default_plan_mode"`
	BaseChannelConfig           `json:",inline" yaml:",inline"`
}

// LarkPersistenceConfig captures Lark local persistence settings in YAML.
type LarkPersistenceConfig struct {
	Mode            string `json:"mode" yaml:"mode"`
	Dir             string `json:"dir" yaml:"dir"`
	RetentionHours  *int   `json:"retention_hours" yaml:"retention_hours"`
	MaxTasksPerChat *int   `json:"max_tasks_per_chat" yaml:"max_tasks_per_chat"`
}

// LarkDeliveryConfig captures Lark terminal delivery reliability settings.
type LarkDeliveryConfig struct {
	Mode   string                    `json:"mode" yaml:"mode"`
	Worker *LarkDeliveryWorkerConfig `json:"worker" yaml:"worker"`
}

// LarkDeliveryWorkerConfig captures outbox worker settings.
type LarkDeliveryWorkerConfig struct {
	Enabled        *bool    `json:"enabled" yaml:"enabled"`
	PollIntervalMs *int     `json:"poll_interval_ms" yaml:"poll_interval_ms"`
	BatchSize      *int     `json:"batch_size" yaml:"batch_size"`
	MaxAttempts    *int     `json:"max_attempts" yaml:"max_attempts"`
	BaseBackoffMs  *int     `json:"base_backoff_ms" yaml:"base_backoff_ms"`
	MaxBackoffMs   *int     `json:"max_backoff_ms" yaml:"max_backoff_ms"`
	JitterRatio    *float64 `json:"jitter_ratio" yaml:"jitter_ratio"`
}

// LarkRateLimiterConfig captures per-chat and per-user notification rate limits.
type LarkRateLimiterConfig struct {
	Enabled         *bool `json:"enabled" yaml:"enabled"`
	ChatHourlyLimit *int  `json:"chat_hourly_limit" yaml:"chat_hourly_limit"`
	UserDailyLimit  *int  `json:"user_daily_limit" yaml:"user_daily_limit"`
}

// LarkBrowserConfig captures local browser settings for the Lark channel.
type LarkBrowserConfig struct {
	CDPURL         string `json:"cdp_url" yaml:"cdp_url"`
	ChromePath     string `json:"chrome_path" yaml:"chrome_path"`
	Headless       *bool  `json:"headless" yaml:"headless"`
	UserDataDir    string `json:"user_data_dir" yaml:"user_data_dir"`
	TimeoutSeconds *int   `json:"timeout_seconds" yaml:"timeout_seconds"`
}

// ServerConfig captures server-specific YAML configuration.
type ServerConfig struct {
	Port                                   string   `yaml:"port"`
	DebugPort                              string   `yaml:"debug_port"`
	DebugBindHost                          string   `yaml:"debug_bind_host"`
	MaxTaskBodyBytes                       *int64   `yaml:"max_task_body_bytes"`
	AllowedOrigins                         []string `yaml:"allowed_origins"`
	StreamMaxDurationSeconds               *int     `yaml:"stream_max_duration_seconds"`
	StreamMaxBytes                         *int64   `yaml:"stream_max_bytes"`
	StreamMaxConcurrent                    *int     `yaml:"stream_max_concurrent"`
	RateLimitRequestsPerMinute             *int     `yaml:"rate_limit_requests_per_minute"`
	RateLimitBurst                         *int     `yaml:"rate_limit_burst"`
	NonStreamTimeoutSeconds                *int     `yaml:"non_stream_timeout_seconds"`
	TaskExecutionOwnerID                   string   `yaml:"task_execution_owner_id"`
	TaskExecutionLeaseTTLSeconds           *int     `yaml:"task_execution_lease_ttl_seconds"`
	TaskExecutionLeaseRenewIntervalSeconds *int     `yaml:"task_execution_lease_renew_interval_seconds"`
	TaskExecutionMaxInFlight               *int     `yaml:"task_execution_max_in_flight"`
	TaskExecutionResumeClaimBatchSize      *int     `yaml:"task_execution_resume_claim_batch_size"`
	EventHistoryRetentionDays              *int     `yaml:"event_history_retention_days"`
	EventHistoryMaxSessions                *int     `yaml:"event_history_max_sessions"`
	EventHistorySessionTTL                 *int     `yaml:"event_history_session_ttl_seconds"`
	EventHistoryMaxEvents                  *int     `yaml:"event_history_max_events"`
	LeaderAPIToken                         string   `yaml:"leader_api_token"`
	TrustedProxies                         []string `yaml:"trusted_proxies"`
}

// AgentConfig captures agent-level behavioral settings.
type AgentConfig struct {
	SessionStaleAfter string `yaml:"session_stale_after"`
}

// SessionConfig captures session persistence configuration.
type SessionConfig struct {
	Dir string `yaml:"dir"`
}

// AnalyticsConfig captures analytics configuration.
type AnalyticsConfig struct {
	PostHogAPIKey string `yaml:"posthog_api_key"`
	PostHogHost   string `yaml:"posthog_host"`
}

// AttachmentsConfig captures attachment store configuration in YAML.
type AttachmentsConfig struct {
	Provider string `yaml:"provider"`
	Dir      string `yaml:"dir"`

	CloudflareAccountID       string `yaml:"cloudflare_account_id"`
	CloudflareAccessKeyID     string `yaml:"cloudflare_access_key_id"`
	CloudflareSecretAccessKey string `yaml:"cloudflare_secret_access_key"`
	CloudflareBucket          string `yaml:"cloudflare_bucket"`
	CloudflarePublicBaseURL   string `yaml:"cloudflare_public_base_url"`
	CloudflareKeyPrefix       string `yaml:"cloudflare_key_prefix"`
	PresignTTL                string `yaml:"presign_ttl"`
}

// WebConfig captures web-facing configuration used by deployment tooling.
type WebConfig struct {
	APIURL string `yaml:"api_url"`
}
