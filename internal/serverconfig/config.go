package serverconfig

import runtimeconfig "alex/internal/config"

// Config captures all server-level settings that can be edited through the
// configuration center.
type Config struct {
	Runtime            runtimeconfig.RuntimeConfig `json:"runtime"`
	Port               string                      `json:"port"`
	EnableMCP          bool                        `json:"enable_mcp"`
	EnvironmentSummary string                      `json:"environment_summary,omitempty"`
	Auth               AuthConfig                  `json:"auth"`
	Analytics          AnalyticsConfig             `json:"analytics"`
}

// AuthConfig mirrors the authentication-related configuration exposed by the
// server binary.
type AuthConfig struct {
	JWTSecret             string `json:"jwt_secret"`
	AccessTokenTTLMinutes string `json:"access_token_ttl_minutes"`
	RefreshTokenTTLDays   string `json:"refresh_token_ttl_days"`
	StateTTLMinutes       string `json:"state_ttl_minutes"`
	RedirectBaseURL       string `json:"redirect_base_url"`
	GoogleClientID        string `json:"google_client_id"`
	GoogleClientSecret    string `json:"google_client_secret"`
	GoogleAuthURL         string `json:"google_auth_url"`
	GoogleTokenURL        string `json:"google_token_url"`
	GoogleUserInfoURL     string `json:"google_userinfo_url"`
	WeChatAppID           string `json:"wechat_app_id"`
	WeChatAuthURL         string `json:"wechat_auth_url"`
	DatabaseURL           string `json:"database_url"`
	BootstrapEmail        string `json:"bootstrap_email"`
	BootstrapPassword     string `json:"bootstrap_password"`
	BootstrapDisplayName  string `json:"bootstrap_display_name"`
}

// AnalyticsConfig exposes analytics-specific settings.
type AnalyticsConfig struct {
	PostHogAPIKey string `json:"posthog_api_key"`
	PostHogHost   string `json:"posthog_host"`
}

// Clone creates a deep copy of the config to avoid accidental mutations.
func (c Config) Clone() Config {
	clone := c
	if len(clone.Runtime.StopSequences) > 0 {
		clone.Runtime.StopSequences = append([]string(nil), clone.Runtime.StopSequences...)
	}
	return clone
}
