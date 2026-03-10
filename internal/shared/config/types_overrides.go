package config

import "time"

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
	LLMVisionModel             *string              `json:"llm_vision_model,omitempty" yaml:"llm_vision_model,omitempty"`
	APIKey                     *string              `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	ArkAPIKey                  *string              `json:"ark_api_key,omitempty" yaml:"ark_api_key,omitempty"`
	BaseURL                    *string              `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	TavilyAPIKey               *string              `json:"tavily_api_key,omitempty" yaml:"tavily_api_key,omitempty"`
	MoltbookAPIKey             *string              `json:"moltbook_api_key,omitempty" yaml:"moltbook_api_key,omitempty"`
	MoltbookBaseURL            *string              `json:"moltbook_base_url,omitempty" yaml:"moltbook_base_url,omitempty"`
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
	KimiRateLimitRPS           *float64             `json:"kimi_rate_limit_rps,omitempty" yaml:"kimi_rate_limit_rps,omitempty"`
	KimiRateLimitBurst         *int                 `json:"kimi_rate_limit_burst,omitempty" yaml:"kimi_rate_limit_burst,omitempty"`
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
	WebSearchMaxResponseBytes *int `json:"web_search_max_response_bytes,omitempty" yaml:"web_search_max_response_bytes,omitempty"`
	ModelListMaxResponseBytes *int `json:"model_list_max_response_bytes,omitempty" yaml:"model_list_max_response_bytes,omitempty"`
}
