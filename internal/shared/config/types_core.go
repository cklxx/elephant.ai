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
	SourceKimiCLI   ValueSource = "kimi_cli"
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
	DefaultLLMCacheTTL     = 30 * time.Minute
	DefaultHTTPMaxResponse = 1 << 20
)

// RuntimeConfig captures user-configurable settings shared across binaries.
type RuntimeConfig struct {
	LLMProvider                string                       `json:"llm_provider" yaml:"llm_provider"`
	LLMModel                   string                       `json:"llm_model" yaml:"llm_model"`
	LLMVisionModel             string                       `json:"llm_vision_model" yaml:"llm_vision_model"`
	APIKey                     string                       `json:"api_key" yaml:"api_key"`
	ArkAPIKey                  string                       `json:"ark_api_key" yaml:"ark_api_key"`
	BaseURL                    string                       `json:"base_url" yaml:"base_url"`
	TavilyAPIKey               string                       `json:"tavily_api_key" yaml:"tavily_api_key"`
	MoltbookAPIKey             string                       `json:"moltbook_api_key" yaml:"moltbook_api_key"`
	MoltbookBaseURL            string                       `json:"moltbook_base_url" yaml:"moltbook_base_url"`
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
	LLMRequestTimeoutSeconds   int                          `json:"llm_request_timeout_seconds" yaml:"llm_request_timeout_seconds"`
	UserRateLimitRPS           float64                      `json:"user_rate_limit_rps" yaml:"user_rate_limit_rps"`
	UserRateLimitBurst         int                          `json:"user_rate_limit_burst" yaml:"user_rate_limit_burst"`
	KimiRateLimitRPS           float64                      `json:"kimi_rate_limit_rps" yaml:"kimi_rate_limit_rps"`
	KimiRateLimitBurst         int                          `json:"kimi_rate_limit_burst" yaml:"kimi_rate_limit_burst"`
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
	LLMFallbackRules           []LLMFallbackRuleConfig      `json:"llm_fallback_rules" yaml:"llm_fallback_rules"`
}

// EnvLookup resolves the value for an environment variable.
type EnvLookup func(string) (string, bool)
