package config

import (
	"time"

	toolspolicy "alex/internal/infra/tools"
)

// ValueSource describes where a configuration value originated from.
type ValueSource string

const (
	SourceDefault   ValueSource = "default"
	SourceFile      ValueSource = "file"
	SourceEnv       ValueSource = "environment"
	SourceOverride  ValueSource = "override"
	SourceCodexCLI  ValueSource = "codex_cli"
	SourceClaudeCLI ValueSource = "claude_cli"
)

// Seedream defaults target the public Volcano Engine Ark deployment in mainland China.
const (
	DefaultSeedreamTextModel   = "doubao-seedream-4-5-251128"
	DefaultSeedreamImageModel  = "doubao-seedream-4-5-251128"
	DefaultSeedreamVisionModel = "doubao-seed-1-6-vision-250815"
	DefaultSeedreamVideoModel  = "doubao-seedance-1-0-pro-fast-251015"
)

const (
	DefaultLLMProvider       = "openai"
	DefaultLLMModel          = "gpt-4o-mini"
	DefaultLLMBaseURL        = "https://api.openai.com/v1"
	RuntimeProfileQuickstart = "quickstart"
	RuntimeProfileStandard   = "standard"
	RuntimeProfileProduction = "production"
	DefaultRuntimeProfile    = RuntimeProfileStandard
	DefaultMaxTokens         = 8192
	DefaultToolMaxConcurrent = 8
	DefaultLLMCacheSize      = 64
	DefaultLLMCacheTTL       = 30 * time.Minute
	DefaultACPHost           = "127.0.0.1"
	DefaultACPPort           = 9000
	DefaultACPPortFile       = "pids/acp.port"
	DefaultHTTPMaxResponse   = 1 << 20
)

// RuntimeConfig captures user-configurable settings shared across binaries.
type RuntimeConfig struct {
	LLMProvider                string                       `json:"llm_provider" yaml:"llm_provider"`
	LLMModel                   string                       `json:"llm_model" yaml:"llm_model"`
	LLMSmallProvider           string                       `json:"llm_small_provider" yaml:"llm_small_provider"`
	LLMSmallModel              string                       `json:"llm_small_model" yaml:"llm_small_model"`
	LLMVisionModel             string                       `json:"llm_vision_model" yaml:"llm_vision_model"`
	APIKey                     string                       `json:"api_key" yaml:"api_key"`
	ArkAPIKey                  string                       `json:"ark_api_key" yaml:"ark_api_key"`
	BaseURL                    string                       `json:"base_url" yaml:"base_url"`
	SandboxBaseURL             string                       `json:"sandbox_base_url" yaml:"sandbox_base_url"`
	ACPExecutorAddr            string                       `json:"acp_executor_addr" yaml:"acp_executor_addr"`
	ACPExecutorCWD             string                       `json:"acp_executor_cwd" yaml:"acp_executor_cwd"`
	ACPExecutorMode            string                       `json:"acp_executor_mode" yaml:"acp_executor_mode"`
	ACPExecutorAutoApprove     bool                         `json:"acp_executor_auto_approve" yaml:"acp_executor_auto_approve"`
	ACPExecutorMaxCLICalls     int                          `json:"acp_executor_max_cli_calls" yaml:"acp_executor_max_cli_calls"`
	ACPExecutorMaxDuration     int                          `json:"acp_executor_max_duration_seconds" yaml:"acp_executor_max_duration_seconds"`
	ACPExecutorRequireManifest bool                         `json:"acp_executor_require_manifest" yaml:"acp_executor_require_manifest"`
	TavilyAPIKey               string                       `json:"tavily_api_key" yaml:"tavily_api_key"`
	MoltbookAPIKey             string                       `json:"moltbook_api_key" yaml:"moltbook_api_key"`
	MoltbookBaseURL            string                       `json:"moltbook_base_url" yaml:"moltbook_base_url"`
	SeedreamTextEndpointID     string                       `json:"seedream_text_endpoint_id" yaml:"seedream_text_endpoint_id"`
	SeedreamImageEndpointID    string                       `json:"seedream_image_endpoint_id" yaml:"seedream_image_endpoint_id"`
	SeedreamTextModel          string                       `json:"seedream_text_model" yaml:"seedream_text_model"`
	SeedreamImageModel         string                       `json:"seedream_image_model" yaml:"seedream_image_model"`
	SeedreamVisionModel        string                       `json:"seedream_vision_model" yaml:"seedream_vision_model"`
	SeedreamVideoModel         string                       `json:"seedream_video_model" yaml:"seedream_video_model"`
	Profile                    string                       `json:"profile" yaml:"profile"`
	Environment                string                       `json:"environment" yaml:"environment"`
	Verbose                    bool                         `json:"verbose" yaml:"verbose"`
	DisableTUI                 bool                         `json:"disable_tui" yaml:"disable_tui"`
	FollowTranscript           bool                         `json:"follow_transcript" yaml:"follow_transcript"`
	FollowStream               bool                         `json:"follow_stream" yaml:"follow_stream"`
	MaxIterations              int                          `json:"max_iterations" yaml:"max_iterations"`
	MaxTokens                  int                          `json:"max_tokens" yaml:"max_tokens"`
	ToolMaxConcurrent          int                          `json:"tool_max_concurrent" yaml:"tool_max_concurrent"`
	LLMCacheSize               int                          `json:"llm_cache_size" yaml:"llm_cache_size"`
	LLMCacheTTLSeconds         int                          `json:"llm_cache_ttl_seconds" yaml:"llm_cache_ttl_seconds"`
	UserRateLimitRPS           float64                      `json:"user_rate_limit_rps" yaml:"user_rate_limit_rps"`
	UserRateLimitBurst         int                          `json:"user_rate_limit_burst" yaml:"user_rate_limit_burst"`
	Temperature                float64                      `json:"temperature" yaml:"temperature"`
	TemperatureProvided        bool                         `json:"temperature_provided" yaml:"temperature_provided"`
	TopP                       float64                      `json:"top_p" yaml:"top_p"`
	StopSequences              []string                     `json:"stop_sequences" yaml:"stop_sequences"`
	SessionDir                 string                       `json:"session_dir" yaml:"session_dir"`
	CostDir                    string                       `json:"cost_dir" yaml:"cost_dir"`
	SessionStaleAfterSeconds   int                          `json:"session_stale_after_seconds" yaml:"session_stale_after_seconds"`
	AgentPreset                string                       `json:"agent_preset" yaml:"agent_preset"`
	ToolPreset                 string                       `json:"tool_preset" yaml:"tool_preset"`
	Toolset                    string                       `json:"toolset" yaml:"toolset"`
	Browser                    BrowserConfig                `json:"browser" yaml:"browser"`
	HTTPLimits                 HTTPLimitsConfig             `json:"http_limits" yaml:"http_limits"`
	ToolPolicy                 toolspolicy.ToolPolicyConfig `json:"tool_policy" yaml:"tool_policy"`
	Proactive                  ProactiveConfig              `json:"proactive" yaml:"proactive"`
	ExternalAgents             ExternalAgentsConfig         `json:"external_agents" yaml:"external_agents"`
}

// BrowserConfig configures local browser tooling when sandbox is disabled.
type BrowserConfig struct {
	Connector      string `json:"connector" yaml:"connector"`
	CDPURL         string `json:"cdp_url" yaml:"cdp_url"`
	ChromePath     string `json:"chrome_path" yaml:"chrome_path"`
	Headless       bool   `json:"headless" yaml:"headless"`
	UserDataDir    string `json:"user_data_dir" yaml:"user_data_dir"`
	TimeoutSeconds int    `json:"timeout_seconds" yaml:"timeout_seconds"`
	BridgeListen   string `json:"bridge_listen_addr" yaml:"bridge_listen_addr"`
	BridgeToken    string `json:"bridge_token" yaml:"bridge_token"`
}

// HTTPLimitsConfig controls maximum response sizes for outbound HTTP calls.
type HTTPLimitsConfig struct {
	DefaultMaxResponseBytes     int `json:"default_max_response_bytes" yaml:"default_max_response_bytes"`
	WebFetchMaxResponseBytes    int `json:"web_fetch_max_response_bytes" yaml:"web_fetch_max_response_bytes"`
	WebSearchMaxResponseBytes   int `json:"web_search_max_response_bytes" yaml:"web_search_max_response_bytes"`
	MusicSearchMaxResponseBytes int `json:"music_search_max_response_bytes" yaml:"music_search_max_response_bytes"`
	ModelListMaxResponseBytes   int `json:"model_list_max_response_bytes" yaml:"model_list_max_response_bytes"`
	SandboxMaxResponseBytes     int `json:"sandbox_max_response_bytes" yaml:"sandbox_max_response_bytes"`
}

// ExternalAgentsConfig configures external agent executors.
type ExternalAgentsConfig struct {
	ClaudeCode ClaudeCodeConfig `json:"claude_code" yaml:"claude_code"`
	Codex      CodexConfig      `json:"codex" yaml:"codex"`
}

type ClaudeCodeConfig struct {
	Enabled                bool              `json:"enabled" yaml:"enabled"`
	Binary                 string            `json:"binary" yaml:"binary"`
	DefaultModel           string            `json:"default_model" yaml:"default_model"`
	DefaultMode            string            `json:"default_mode" yaml:"default_mode"`
	AutonomousAllowedTools []string          `json:"autonomous_allowed_tools" yaml:"autonomous_allowed_tools"`
	MaxBudgetUSD           float64           `json:"max_budget_usd" yaml:"max_budget_usd"`
	MaxTurns               int               `json:"max_turns" yaml:"max_turns"`
	Timeout                time.Duration     `json:"timeout" yaml:"timeout"`
	Env                    map[string]string `json:"env" yaml:"env"`
}

type CodexConfig struct {
	Enabled        bool              `json:"enabled" yaml:"enabled"`
	Binary         string            `json:"binary" yaml:"binary"`
	DefaultModel   string            `json:"default_model" yaml:"default_model"`
	ApprovalPolicy string            `json:"approval_policy" yaml:"approval_policy"`
	Sandbox        string            `json:"sandbox" yaml:"sandbox"`
	Timeout        time.Duration     `json:"timeout" yaml:"timeout"`
	Env            map[string]string `json:"env" yaml:"env"`
}

// DefaultExternalAgentsConfig provides baseline defaults for external agents.
func DefaultExternalAgentsConfig() ExternalAgentsConfig {
	return ExternalAgentsConfig{
		ClaudeCode: ClaudeCodeConfig{
			Enabled:     false,
			Binary:      "claude",
			DefaultMode: "autonomous",
			MaxTurns:    50,
			Timeout:     30 * time.Minute,
			AutonomousAllowedTools: []string{
				"*",
			},
			Env: map[string]string{},
		},
		Codex: CodexConfig{
			Enabled:        false,
			Binary:         "codex",
			DefaultModel:   "gpt-5.2-codex",
			ApprovalPolicy: "never",
			Sandbox:        "danger-full-access",
			Timeout:        30 * time.Minute,
			Env:            map[string]string{},
		},
	}
}

// DefaultHTTPLimitsConfig provides baseline HTTP response size limits.
func DefaultHTTPLimitsConfig() HTTPLimitsConfig {
	return HTTPLimitsConfig{
		DefaultMaxResponseBytes:     DefaultHTTPMaxResponse,
		WebFetchMaxResponseBytes:    2 * DefaultHTTPMaxResponse,
		WebSearchMaxResponseBytes:   DefaultHTTPMaxResponse,
		MusicSearchMaxResponseBytes: DefaultHTTPMaxResponse,
		ModelListMaxResponseBytes:   512 * 1024,
		SandboxMaxResponseBytes:     8 * 1024 * 1024,
	}
}

// ProactiveConfig captures proactive behavior defaults.
type ProactiveConfig struct {
	Enabled           bool                    `json:"enabled" yaml:"enabled"`
	Prompt            PromptConfig            `json:"prompt" yaml:"prompt"`
	Memory            MemoryConfig            `json:"memory" yaml:"memory"`
	Skills            SkillsConfig            `json:"skills" yaml:"skills"`
	OKR               OKRProactiveConfig      `json:"okr" yaml:"okr"`
	Scheduler         SchedulerConfig         `json:"scheduler" yaml:"scheduler"`
	Timer             TimerConfig             `json:"timer" yaml:"timer"`
	FinalAnswerReview FinalAnswerReviewConfig `json:"final_answer_review" yaml:"final_answer_review"`
	Attention         AttentionConfig         `json:"attention" yaml:"attention"`
	Kernel            KernelProactiveConfig   `json:"kernel" yaml:"kernel"`
}

// KernelProactiveConfig configures the kernel agent loop.
type KernelProactiveConfig struct {
	Enabled        bool                         `json:"enabled" yaml:"enabled"`
	KernelID       string                       `json:"kernel_id" yaml:"kernel_id"`
	Schedule       string                       `json:"schedule" yaml:"schedule"`
	StateDir       string                       `json:"state_dir" yaml:"state_dir"`
	SeedState      string                       `json:"seed_state" yaml:"seed_state"`
	TimeoutSeconds int                          `json:"timeout_seconds" yaml:"timeout_seconds"`
	LeaseSeconds   int                          `json:"lease_seconds" yaml:"lease_seconds"`
	MaxConcurrent  int                          `json:"max_concurrent" yaml:"max_concurrent"`
	Channel        string                       `json:"channel" yaml:"channel"`
	UserID         string                       `json:"user_id" yaml:"user_id"`
	ChatID         string                       `json:"chat_id" yaml:"chat_id"`
	Agents         []KernelAgentProactiveConfig `json:"agents" yaml:"agents"`
}

// KernelAgentProactiveConfig defines a single agent within the kernel loop.
type KernelAgentProactiveConfig struct {
	AgentID  string            `json:"agent_id" yaml:"agent_id"`
	Prompt   string            `json:"prompt" yaml:"prompt"`
	Priority int               `json:"priority" yaml:"priority"`
	Enabled  bool              `json:"enabled" yaml:"enabled"`
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// PromptConfig controls system-prompt assembly behavior.
type PromptConfig struct {
	Mode              string   `json:"mode" yaml:"mode"` // full | minimal | none
	Timezone          string   `json:"timezone" yaml:"timezone"`
	BootstrapMaxChars int      `json:"bootstrap_max_chars" yaml:"bootstrap_max_chars"`
	BootstrapFiles    []string `json:"bootstrap_files" yaml:"bootstrap_files"`
	ReplyTagsEnabled  bool     `json:"reply_tags_enabled" yaml:"reply_tags_enabled"`
}

// OKRProactiveConfig configures OKR goal management behavior.
type OKRProactiveConfig struct {
	Enabled    bool   `json:"enabled" yaml:"enabled"`
	GoalsRoot  string `json:"goals_root" yaml:"goals_root"`   // default: ~/.alex/goals
	AutoInject bool   `json:"auto_inject" yaml:"auto_inject"` // inject OKR context into tasks
}

// MemoryConfig controls loading persistent Markdown memory.
type MemoryConfig struct {
	Enabled bool              `json:"enabled" yaml:"enabled"`
	Index   MemoryIndexConfig `json:"index" yaml:"index"`
}

// MemoryIndexConfig controls local vector indexing for Markdown memory.
type MemoryIndexConfig struct {
	Enabled            bool    `json:"enabled" yaml:"enabled"`
	DBPath             string  `json:"db_path" yaml:"db_path"`
	ChunkTokens        int     `json:"chunk_tokens" yaml:"chunk_tokens"`
	ChunkOverlap       int     `json:"chunk_overlap" yaml:"chunk_overlap"`
	MinScore           float64 `json:"min_score" yaml:"min_score"`
	FusionWeightVector float64 `json:"fusion_weight_vector" yaml:"fusion_weight_vector"`
	FusionWeightBM25   float64 `json:"fusion_weight_bm25" yaml:"fusion_weight_bm25"`
	EmbedderModel      string  `json:"embedder_model" yaml:"embedder_model"`
}

// SkillsConfig controls skill activation and feedback.
type SkillsConfig struct {
	AutoActivation  SkillsAutoActivationConfig `json:"auto_activation" yaml:"auto_activation"`
	Feedback        SkillsFeedbackConfig       `json:"feedback" yaml:"feedback"`
	CacheTTLSeconds int                        `json:"cache_ttl_seconds" yaml:"cache_ttl_seconds"`
}

type SkillsAutoActivationConfig struct {
	Enabled             bool    `json:"enabled" yaml:"enabled"`
	MaxActivated        int     `json:"max_activated" yaml:"max_activated"`
	TokenBudget         int     `json:"token_budget" yaml:"token_budget"`
	ConfidenceThreshold float64 `json:"confidence_threshold" yaml:"confidence_threshold"`
}

type SkillsFeedbackConfig struct {
	Enabled   bool   `json:"enabled" yaml:"enabled"`
	StorePath string `json:"store_path" yaml:"store_path"`
}

// SchedulerConfig configures time-based proactive triggers.
type SchedulerConfig struct {
	Enabled                bool                     `json:"enabled" yaml:"enabled"`
	Triggers               []SchedulerTriggerConfig `json:"triggers" yaml:"triggers"`
	TriggerTimeoutSeconds  int                      `json:"trigger_timeout_seconds" yaml:"trigger_timeout_seconds"`
	ConcurrencyPolicy      string                   `json:"concurrency_policy" yaml:"concurrency_policy"`
	JobStorePath           string                   `json:"job_store_path" yaml:"job_store_path"`
	CooldownSeconds        int                      `json:"cooldown_seconds" yaml:"cooldown_seconds"`
	MaxConcurrent          int                      `json:"max_concurrent" yaml:"max_concurrent"`
	RecoveryMaxRetries     int                      `json:"recovery_max_retries" yaml:"recovery_max_retries"`
	RecoveryBackoffSeconds int                      `json:"recovery_backoff_seconds" yaml:"recovery_backoff_seconds"`
	CalendarReminder       CalendarReminderConfig   `json:"calendar_reminder" yaml:"calendar_reminder"`
	Heartbeat              HeartbeatConfig          `json:"heartbeat" yaml:"heartbeat"`
}

// CalendarReminderConfig configures the periodic calendar reminder trigger.
type CalendarReminderConfig struct {
	Enabled          bool   `json:"enabled" yaml:"enabled"`
	Schedule         string `json:"schedule" yaml:"schedule"`                     // cron expression, default "*/15 * * * *"
	LookAheadMinutes int    `json:"look_ahead_minutes" yaml:"look_ahead_minutes"` // default 120
	Channel          string `json:"channel" yaml:"channel"`                       // delivery channel: lark | moltbook
	UserID           string `json:"user_id" yaml:"user_id"`                       // for channel=lark, this must be Lark open_id (ou_*)
	ChatID           string `json:"chat_id" yaml:"chat_id"`
}

type SchedulerTriggerConfig struct {
	Name             string `json:"name" yaml:"name"`
	Schedule         string `json:"schedule" yaml:"schedule"`
	Task             string `json:"task" yaml:"task"`
	Channel          string `json:"channel" yaml:"channel"`
	UserID           string `json:"user_id" yaml:"user_id"` // for channel=lark, this must be Lark open_id (ou_*)
	ChatID           string `json:"chat_id" yaml:"chat_id"` // channel-specific chat ID for notifications
	ApprovalRequired bool   `json:"approval_required" yaml:"approval_required"`
	Risk             string `json:"risk" yaml:"risk"`
}

// HeartbeatConfig configures periodic heartbeat checks driven by scheduler or timers.
type HeartbeatConfig struct {
	Enabled          bool   `json:"enabled" yaml:"enabled"`
	Schedule         string `json:"schedule" yaml:"schedule"` // cron expression, default "*/30 * * * *"
	Task             string `json:"task" yaml:"task"`
	Channel          string `json:"channel" yaml:"channel"`
	UserID           string `json:"user_id" yaml:"user_id"` // for channel=lark, this must be Lark open_id (ou_*)
	ChatID           string `json:"chat_id" yaml:"chat_id"`
	QuietHours       [2]int `json:"quiet_hours" yaml:"quiet_hours"`
	WindowLookbackHr int    `json:"window_lookback_hours" yaml:"window_lookback_hours"`
}

// TimerConfig configures agent-initiated dynamic timers.
type TimerConfig struct {
	Enabled            bool   `json:"enabled" yaml:"enabled"`
	StorePath          string `json:"store_path" yaml:"store_path"`                     // default: ~/.alex/timers
	MaxTimers          int    `json:"max_timers" yaml:"max_timers"`                     // default: 100
	TaskTimeoutSeconds int    `json:"task_timeout_seconds" yaml:"task_timeout_seconds"` // default: 900
	HeartbeatEnabled   bool   `json:"heartbeat_enabled" yaml:"heartbeat_enabled"`
	HeartbeatMinutes   int    `json:"heartbeat_minutes" yaml:"heartbeat_minutes"`
}

// FinalAnswerReviewConfig controls whether to insert an additional ReAct iteration
// before finalizing when the model returns a plain final answer (no tool calls).
type FinalAnswerReviewConfig struct {
	Enabled            bool `json:"enabled" yaml:"enabled"`
	MaxExtraIterations int  `json:"max_extra_iterations" yaml:"max_extra_iterations"`
}

// AttentionConfig throttles proactive notifications.
type AttentionConfig struct {
	MaxDailyNotifications int     `json:"max_daily_notifications" yaml:"max_daily_notifications"`
	MinIntervalSeconds    int     `json:"min_interval_seconds" yaml:"min_interval_seconds"`
	QuietHours            [2]int  `json:"quiet_hours" yaml:"quiet_hours"`
	PriorityThreshold     float64 `json:"priority_threshold" yaml:"priority_threshold"`
}

// DefaultProactiveConfig returns the baseline proactive defaults.
func DefaultProactiveConfig() ProactiveConfig {
	return ProactiveConfig{
		Enabled: true,
		Prompt: PromptConfig{
			Mode:              "full",
			BootstrapMaxChars: 20000,
			BootstrapFiles: []string{
				"AGENTS.md",
				"SOUL.md",
				"TOOLS.md",
				"IDENTITY.md",
				"USER.md",
				"HEARTBEAT.md",
				"BOOTSTRAP.md",
			},
		},
		Memory: MemoryConfig{
			Enabled: true,
			Index: MemoryIndexConfig{
				Enabled:            true,
				DBPath:             "~/.alex/memory/index.sqlite",
				ChunkTokens:        400,
				ChunkOverlap:       80,
				MinScore:           0.35,
				FusionWeightVector: 0.7,
				FusionWeightBM25:   0.3,
				EmbedderModel:      "nomic-embed-text",
			},
		},
		Skills: SkillsConfig{
			AutoActivation: SkillsAutoActivationConfig{
				Enabled:             true,
				MaxActivated:        3,
				TokenBudget:         4000,
				ConfidenceThreshold: 0.6,
			},
			Feedback: SkillsFeedbackConfig{
				Enabled: false,
			},
			CacheTTLSeconds: 300,
		},
		OKR: OKRProactiveConfig{
			Enabled:    true,
			AutoInject: true,
		},
		Scheduler: SchedulerConfig{
			Enabled:                false,
			TriggerTimeoutSeconds:  900,
			ConcurrencyPolicy:      "skip",
			CooldownSeconds:        0,
			MaxConcurrent:          1,
			RecoveryMaxRetries:     0,
			RecoveryBackoffSeconds: 60,
			CalendarReminder: CalendarReminderConfig{
				Enabled:          false,
				Schedule:         "*/15 * * * *",
				LookAheadMinutes: 120,
			},
			Heartbeat: HeartbeatConfig{
				Enabled:          false,
				Schedule:         "*/30 * * * *",
				Task:             "Read HEARTBEAT.md if it exists. Follow it strictly. If nothing needs attention, reply HEARTBEAT_OK.",
				QuietHours:       [2]int{23, 8},
				WindowLookbackHr: 8,
			},
		},
		Timer: TimerConfig{
			Enabled:            true,
			StorePath:          "~/.alex/timers",
			MaxTimers:          100,
			TaskTimeoutSeconds: 900,
			HeartbeatEnabled:   false,
			HeartbeatMinutes:   30,
		},
		FinalAnswerReview: FinalAnswerReviewConfig{
			Enabled:            true,
			MaxExtraIterations: 1,
		},
		Attention: AttentionConfig{
			MaxDailyNotifications: 5,
			MinIntervalSeconds:    1800,
			QuietHours:            [2]int{22, 8},
			PriorityThreshold:     0.6,
		},
		Kernel: KernelProactiveConfig{
			Enabled:        false,
			KernelID:       "default",
			Schedule:       "*/10 * * * *",
			StateDir:       "~/.alex/kernel",
			TimeoutSeconds: 900,
			LeaseSeconds:   1800,
			MaxConcurrent:  3,
		},
	}
}

// Metadata contains provenance details for loaded configuration.
type Metadata struct {
	sources  map[string]ValueSource
	loadedAt time.Time
}

// Sources returns a copy of the provenance map for JSON serialization.
func (m Metadata) Sources() map[string]ValueSource {
	if m.sources == nil {
		return map[string]ValueSource{}
	}
	copy := make(map[string]ValueSource, len(m.sources))
	for key, value := range m.sources {
		copy[key] = value
	}
	return copy
}

// Source returns the origin for the given configuration field.
func (m Metadata) Source(field string) ValueSource {
	if m.sources == nil {
		return SourceDefault
	}
	if src, ok := m.sources[field]; ok {
		return src
	}
	return SourceDefault
}

// LoadedAt returns the timestamp when the configuration was constructed.
func (m Metadata) LoadedAt() time.Time {
	return m.loadedAt
}

// Overrides conveys caller-specified values that should win over env/file sources.
type Overrides struct {
	LLMProvider                *string              `json:"llm_provider,omitempty" yaml:"llm_provider,omitempty"`
	LLMModel                   *string              `json:"llm_model,omitempty" yaml:"llm_model,omitempty"`
	LLMSmallProvider           *string              `json:"llm_small_provider,omitempty" yaml:"llm_small_provider,omitempty"`
	LLMSmallModel              *string              `json:"llm_small_model,omitempty" yaml:"llm_small_model,omitempty"`
	LLMVisionModel             *string              `json:"llm_vision_model,omitempty" yaml:"llm_vision_model,omitempty"`
	APIKey                     *string              `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	ArkAPIKey                  *string              `json:"ark_api_key,omitempty" yaml:"ark_api_key,omitempty"`
	BaseURL                    *string              `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	SandboxBaseURL             *string              `json:"sandbox_base_url,omitempty" yaml:"sandbox_base_url,omitempty"`
	ACPExecutorAddr            *string              `json:"acp_executor_addr,omitempty" yaml:"acp_executor_addr,omitempty"`
	ACPExecutorCWD             *string              `json:"acp_executor_cwd,omitempty" yaml:"acp_executor_cwd,omitempty"`
	ACPExecutorMode            *string              `json:"acp_executor_mode,omitempty" yaml:"acp_executor_mode,omitempty"`
	ACPExecutorAutoApprove     *bool                `json:"acp_executor_auto_approve,omitempty" yaml:"acp_executor_auto_approve,omitempty"`
	ACPExecutorMaxCLICalls     *int                 `json:"acp_executor_max_cli_calls,omitempty" yaml:"acp_executor_max_cli_calls,omitempty"`
	ACPExecutorMaxDuration     *int                 `json:"acp_executor_max_duration_seconds,omitempty" yaml:"acp_executor_max_duration_seconds,omitempty"`
	ACPExecutorRequireManifest *bool                `json:"acp_executor_require_manifest,omitempty" yaml:"acp_executor_require_manifest,omitempty"`
	TavilyAPIKey               *string              `json:"tavily_api_key,omitempty" yaml:"tavily_api_key,omitempty"`
	MoltbookAPIKey             *string              `json:"moltbook_api_key,omitempty" yaml:"moltbook_api_key,omitempty"`
	MoltbookBaseURL            *string              `json:"moltbook_base_url,omitempty" yaml:"moltbook_base_url,omitempty"`
	SeedreamTextEndpointID     *string              `json:"seedream_text_endpoint_id,omitempty" yaml:"seedream_text_endpoint_id,omitempty"`
	SeedreamImageEndpointID    *string              `json:"seedream_image_endpoint_id,omitempty" yaml:"seedream_image_endpoint_id,omitempty"`
	SeedreamTextModel          *string              `json:"seedream_text_model,omitempty" yaml:"seedream_text_model,omitempty"`
	SeedreamImageModel         *string              `json:"seedream_image_model,omitempty" yaml:"seedream_image_model,omitempty"`
	SeedreamVisionModel        *string              `json:"seedream_vision_model,omitempty" yaml:"seedream_vision_model,omitempty"`
	SeedreamVideoModel         *string              `json:"seedream_video_model,omitempty" yaml:"seedream_video_model,omitempty"`
	Profile                    *string              `json:"profile,omitempty" yaml:"profile,omitempty"`
	Environment                *string              `json:"environment,omitempty" yaml:"environment,omitempty"`
	Verbose                    *bool                `json:"verbose,omitempty" yaml:"verbose,omitempty"`
	DisableTUI                 *bool                `json:"disable_tui,omitempty" yaml:"disable_tui,omitempty"`
	FollowTranscript           *bool                `json:"follow_transcript,omitempty" yaml:"follow_transcript,omitempty"`
	FollowStream               *bool                `json:"follow_stream,omitempty" yaml:"follow_stream,omitempty"`
	MaxIterations              *int                 `json:"max_iterations,omitempty" yaml:"max_iterations,omitempty"`
	MaxTokens                  *int                 `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
	ToolMaxConcurrent          *int                 `json:"tool_max_concurrent,omitempty" yaml:"tool_max_concurrent,omitempty"`
	LLMCacheSize               *int                 `json:"llm_cache_size,omitempty" yaml:"llm_cache_size,omitempty"`
	LLMCacheTTLSeconds         *int                 `json:"llm_cache_ttl_seconds,omitempty" yaml:"llm_cache_ttl_seconds,omitempty"`
	UserRateLimitRPS           *float64             `json:"user_rate_limit_rps,omitempty" yaml:"user_rate_limit_rps,omitempty"`
	UserRateLimitBurst         *int                 `json:"user_rate_limit_burst,omitempty" yaml:"user_rate_limit_burst,omitempty"`
	Temperature                *float64             `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	TopP                       *float64             `json:"top_p,omitempty" yaml:"top_p,omitempty"`
	StopSequences              *[]string            `json:"stop_sequences,omitempty" yaml:"stop_sequences,omitempty"`
	SessionDir                 *string              `json:"session_dir,omitempty" yaml:"session_dir,omitempty"`
	CostDir                    *string              `json:"cost_dir,omitempty" yaml:"cost_dir,omitempty"`
	SessionStaleAfterSeconds   *int                 `json:"session_stale_after_seconds,omitempty" yaml:"session_stale_after_seconds,omitempty"`
	AgentPreset                *string              `json:"agent_preset,omitempty" yaml:"agent_preset,omitempty"`
	ToolPreset                 *string              `json:"tool_preset,omitempty" yaml:"tool_preset,omitempty"`
	Toolset                    *string              `json:"toolset,omitempty" yaml:"toolset,omitempty"`
	Browser                    *BrowserOverrides    `json:"browser,omitempty" yaml:"browser,omitempty"`
	HTTPLimits                 *HTTPLimitsOverrides `json:"http_limits,omitempty" yaml:"http_limits,omitempty"`
	Proactive                  *ProactiveConfig     `json:"proactive,omitempty" yaml:"proactive,omitempty"`
}

// BrowserOverrides allows partial browser config overrides.
type BrowserOverrides struct {
	Connector      *string `json:"connector,omitempty" yaml:"connector,omitempty"`
	CDPURL         *string `json:"cdp_url,omitempty" yaml:"cdp_url,omitempty"`
	ChromePath     *string `json:"chrome_path,omitempty" yaml:"chrome_path,omitempty"`
	Headless       *bool   `json:"headless,omitempty" yaml:"headless,omitempty"`
	UserDataDir    *string `json:"user_data_dir,omitempty" yaml:"user_data_dir,omitempty"`
	TimeoutSeconds *int    `json:"timeout_seconds,omitempty" yaml:"timeout_seconds,omitempty"`
	BridgeListen   *string `json:"bridge_listen_addr,omitempty" yaml:"bridge_listen_addr,omitempty"`
	BridgeToken    *string `json:"bridge_token,omitempty" yaml:"bridge_token,omitempty"`
}

// HTTPLimitsOverrides allows partial HTTP limit overrides.
type HTTPLimitsOverrides struct {
	DefaultMaxResponseBytes     *int `json:"default_max_response_bytes,omitempty" yaml:"default_max_response_bytes,omitempty"`
	WebFetchMaxResponseBytes    *int `json:"web_fetch_max_response_bytes,omitempty" yaml:"web_fetch_max_response_bytes,omitempty"`
	WebSearchMaxResponseBytes   *int `json:"web_search_max_response_bytes,omitempty" yaml:"web_search_max_response_bytes,omitempty"`
	MusicSearchMaxResponseBytes *int `json:"music_search_max_response_bytes,omitempty" yaml:"music_search_max_response_bytes,omitempty"`
	ModelListMaxResponseBytes   *int `json:"model_list_max_response_bytes,omitempty" yaml:"model_list_max_response_bytes,omitempty"`
	SandboxMaxResponseBytes     *int `json:"sandbox_max_response_bytes,omitempty" yaml:"sandbox_max_response_bytes,omitempty"`
}

// EnvLookup resolves the value for an environment variable.
type EnvLookup func(string) (string, bool)
