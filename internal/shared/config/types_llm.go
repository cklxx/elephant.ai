package config

// LLMFallbackRuleConfig defines a model-level failover target for transient LLM failures.
// When the primary model exhausts retries on a transient error (e.g. 529 overloaded),
// the system falls back to the specified model.
type LLMFallbackRuleConfig struct {
	Model            string `json:"model" yaml:"model"`                         // primary model to match
	FallbackProvider string `json:"fallback_provider" yaml:"fallback_provider"` // fallback provider name
	FallbackModel    string `json:"fallback_model" yaml:"fallback_model"`       // fallback model name
	FallbackBaseURL  string `json:"fallback_base_url" yaml:"fallback_base_url"` // fallback base URL (optional)
	FallbackAPIKey   string `json:"fallback_api_key" yaml:"fallback_api_key"`   // fallback API key (optional; inherits primary if empty)
}

// BrowserConfig configures the browser integration backend.
//
// Connector modes:
//   - "extension" (default): Connect to user's existing Chrome via the Playwright
//     Bridge extension. Reuses cookies/sessions/login state.
//   - "headless": Launch an isolated headless browser instance.
//   - "cdp": Connect to an existing browser via a CDP endpoint URL.
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
	WebSearchMaxResponseBytes int `json:"web_search_max_response_bytes" yaml:"web_search_max_response_bytes"`
	ModelListMaxResponseBytes int `json:"model_list_max_response_bytes" yaml:"model_list_max_response_bytes"`
}

// DefaultHTTPLimitsConfig provides baseline HTTP response size limits.
func DefaultHTTPLimitsConfig() HTTPLimitsConfig {
	return HTTPLimitsConfig{
		DefaultMaxResponseBytes:     DefaultHTTPMaxResponse,
		WebFetchMaxResponseBytes:    2 * DefaultHTTPMaxResponse,
		WebSearchMaxResponseBytes: DefaultHTTPMaxResponse,
		ModelListMaxResponseBytes: 512 * 1024,
	}
}
