package config

// FileConfig captures the on-disk YAML configuration sections.
type FileConfig struct {
	Runtime     *RuntimeFileConfig `yaml:"runtime"`
	Overrides   *Overrides         `yaml:"overrides"`
	Server      *ServerConfig      `yaml:"server"`
	Auth        *AuthConfig        `yaml:"auth"`
	Session     *SessionConfig     `yaml:"session"`
	Analytics   *AnalyticsConfig   `yaml:"analytics"`
	Attachments *AttachmentsConfig `yaml:"attachments"`
	Web         *WebConfig         `yaml:"web"`
}

// RuntimeFileConfig mirrors RuntimeConfig for YAML decoding (runtime section).
type RuntimeFileConfig struct {
	LLMProvider             string   `yaml:"llm_provider"`
	LLMModel                string   `yaml:"llm_model"`
	LLMSmallProvider        string   `yaml:"llm_small_provider"`
	LLMSmallModel           string   `yaml:"llm_small_model"`
	LLMVisionModel          string   `yaml:"llm_vision_model"`
	APIKey                  string   `yaml:"api_key"`
	ArkAPIKey               string   `yaml:"ark_api_key"`
	BaseURL                 string   `yaml:"base_url"`
	SandboxBaseURL          string   `yaml:"sandbox_base_url"`
	ACPExecutorAddr         string   `yaml:"acp_executor_addr"`
	ACPExecutorCWD          string   `yaml:"acp_executor_cwd"`
	ACPExecutorAutoApprove  *bool    `yaml:"acp_executor_auto_approve"`
	ACPExecutorMaxCLICalls  *int     `yaml:"acp_executor_max_cli_calls"`
	ACPExecutorMaxDuration  *int     `yaml:"acp_executor_max_duration_seconds"`
	ACPExecutorRequireManifest *bool `yaml:"acp_executor_require_manifest"`
	TavilyAPIKey            string   `yaml:"tavily_api_key"`
	SeedreamTextEndpointID  string   `yaml:"seedream_text_endpoint_id"`
	SeedreamImageEndpointID string   `yaml:"seedream_image_endpoint_id"`
	SeedreamTextModel       string   `yaml:"seedream_text_model"`
	SeedreamImageModel      string   `yaml:"seedream_image_model"`
	SeedreamVisionModel     string   `yaml:"seedream_vision_model"`
	SeedreamVideoModel      string   `yaml:"seedream_video_model"`
	Environment             string   `yaml:"environment"`
	Verbose                 *bool    `yaml:"verbose"`
	DisableTUI              *bool    `yaml:"disable_tui"`
	FollowTranscript        *bool    `yaml:"follow_transcript"`
	FollowStream            *bool    `yaml:"follow_stream"`
	MaxIterations           *int     `yaml:"max_iterations"`
	MaxTokens               *int     `yaml:"max_tokens"`
	UserRateLimitRPS        *float64 `yaml:"user_rate_limit_rps"`
	UserRateLimitBurst      *int     `yaml:"user_rate_limit_burst"`
	Temperature             *float64 `yaml:"temperature"`
	TopP                    *float64 `yaml:"top_p"`
	StopSequences           []string `yaml:"stop_sequences"`
	SessionDir              string   `yaml:"session_dir"`
	CostDir                 string   `yaml:"cost_dir"`
	AgentPreset             string   `yaml:"agent_preset"`
	ToolPreset              string   `yaml:"tool_preset"`
}

// ServerConfig captures server-specific YAML configuration.
type ServerConfig struct {
	Port             string   `yaml:"port"`
	EnableMCP        *bool    `yaml:"enable_mcp"`
	MaxTaskBodyBytes *int64   `yaml:"max_task_body_bytes"`
	AllowedOrigins   []string `yaml:"allowed_origins"`
}

// AuthConfig captures authentication configuration stored in YAML.
type AuthConfig struct {
	JWTSecret             string `yaml:"jwt_secret"`
	AccessTokenTTLMinutes string `yaml:"access_token_ttl_minutes"`
	RefreshTokenTTLDays   string `yaml:"refresh_token_ttl_days"`
	StateTTLMinutes       string `yaml:"state_ttl_minutes"`
	RedirectBaseURL       string `yaml:"redirect_base_url"`
	GoogleClientID        string `yaml:"google_client_id"`
	GoogleClientSecret    string `yaml:"google_client_secret"`
	GoogleAuthURL         string `yaml:"google_auth_url"`
	GoogleTokenURL        string `yaml:"google_token_url"`
	GoogleUserInfoURL     string `yaml:"google_userinfo_url"`
	WeChatAppID           string `yaml:"wechat_app_id"`
	WeChatAuthURL         string `yaml:"wechat_auth_url"`
	DatabaseURL           string `yaml:"database_url"`
	BootstrapEmail        string `yaml:"bootstrap_email"`
	BootstrapPassword     string `yaml:"bootstrap_password"`
	BootstrapDisplayName  string `yaml:"bootstrap_display_name"`
}

// SessionConfig captures session persistence configuration.
type SessionConfig struct {
	DatabaseURL string `yaml:"database_url"`
	Dir         string `yaml:"dir"`
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
