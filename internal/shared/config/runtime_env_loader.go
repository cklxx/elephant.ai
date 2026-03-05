package config

import (
	"fmt"
	"strconv"
	"strings"
)

func applyEnv(cfg *RuntimeConfig, meta *Metadata, opts loadOptions) error {
	lookup := opts.envLookup
	if lookup == nil {
		lookup = DefaultEnvLookup
	}

	setEnvString(lookup, meta, "ARK_API_KEY", "ark_api_key", func(value string) { cfg.ArkAPIKey = value })
	setEnvString(lookup, meta, "LLM_PROVIDER", "llm_provider", func(value string) { cfg.LLMProvider = value })
	setEnvString(lookup, meta, "LLM_MODEL", "llm_model", func(value string) { cfg.LLMModel = value })
	setEnvString(lookup, meta, "LLM_VISION_MODEL", "llm_vision_model", func(value string) { cfg.LLMVisionModel = value })
	setEnvString(lookup, meta, "LLM_BASE_URL", "base_url", func(value string) { cfg.BaseURL = value })
	setEnvString(lookup, meta, "ACP_EXECUTOR_ADDR", "acp_executor_addr", func(value string) { cfg.ACPExecutorAddr = value })
	setEnvString(lookup, meta, "ACP_EXECUTOR_CWD", "acp_executor_cwd", func(value string) { cfg.ACPExecutorCWD = value })
	setEnvString(lookup, meta, "ACP_EXECUTOR_MODE", "acp_executor_mode", func(value string) { cfg.ACPExecutorMode = value })
	if err := setEnvBool(lookup, meta, "ACP_EXECUTOR_AUTO_APPROVE", "acp_executor_auto_approve", func(value bool) { cfg.ACPExecutorAutoApprove = value }); err != nil {
		return err
	}
	if err := setEnvInt(lookup, meta, "ACP_EXECUTOR_MAX_CLI_CALLS", "acp_executor_max_cli_calls", func(value int) { cfg.ACPExecutorMaxCLICalls = value }); err != nil {
		return err
	}
	if err := setEnvInt(lookup, meta, "ACP_EXECUTOR_MAX_DURATION_SECONDS", "acp_executor_max_duration_seconds", func(value int) { cfg.ACPExecutorMaxDuration = value }); err != nil {
		return err
	}
	if err := setEnvBool(lookup, meta, "ACP_EXECUTOR_REQUIRE_MANIFEST", "acp_executor_require_manifest", func(value bool) { cfg.ACPExecutorRequireManifest = value }); err != nil {
		return err
	}
	setEnvString(lookup, meta, "TAVILY_API_KEY", "tavily_api_key", func(value string) { cfg.TavilyAPIKey = value })
	setEnvString(lookup, meta, "MOLTBOOK_API_KEY", "moltbook_api_key", func(value string) { cfg.MoltbookAPIKey = value })
	setEnvString(lookup, meta, "MOLTBOOK_BASE_URL", "moltbook_base_url", func(value string) { cfg.MoltbookBaseURL = value })
	setEnvString(lookup, meta, "SEEDREAM_TEXT_ENDPOINT_ID", "seedream_text_endpoint_id", func(value string) { cfg.SeedreamTextEndpointID = value })
	setEnvString(lookup, meta, "SEEDREAM_IMAGE_ENDPOINT_ID", "seedream_image_endpoint_id", func(value string) { cfg.SeedreamImageEndpointID = value })
	setEnvString(lookup, meta, "SEEDREAM_TEXT_MODEL", "seedream_text_model", func(value string) { cfg.SeedreamTextModel = value })
	setEnvString(lookup, meta, "SEEDREAM_IMAGE_MODEL", "seedream_image_model", func(value string) { cfg.SeedreamImageModel = value })
	setEnvString(lookup, meta, "SEEDREAM_VISION_MODEL", "seedream_vision_model", func(value string) { cfg.SeedreamVisionModel = value })
	setEnvString(lookup, meta, "SEEDREAM_VIDEO_MODEL", "seedream_video_model", func(value string) { cfg.SeedreamVideoModel = value })
	setEnvString(lookup, meta, "AGENT_PRESET", "agent_preset", func(value string) { cfg.AgentPreset = value })
	setEnvString(lookup, meta, "TOOL_PRESET", "tool_preset", func(value string) { cfg.ToolPreset = value })
	setEnvString(lookup, meta, "ALEX_TOOLSET", "toolset", func(value string) { cfg.Toolset = value })
	setEnvString(lookup, meta, "ALEX_BROWSER_CONNECTOR", "browser.connector", func(value string) { cfg.Browser.Connector = value })
	setEnvString(lookup, meta, "ALEX_BROWSER_CDP_URL", "browser.cdp_url", func(value string) { cfg.Browser.CDPURL = value })
	setEnvString(lookup, meta, "ALEX_BROWSER_CHROME_PATH", "browser.chrome_path", func(value string) { cfg.Browser.ChromePath = value })
	if err := setEnvBool(lookup, meta, "ALEX_BROWSER_HEADLESS", "browser.headless", func(value bool) { cfg.Browser.Headless = value }); err != nil {
		return err
	}
	setEnvString(lookup, meta, "ALEX_BROWSER_USER_DATA_DIR", "browser.user_data_dir", func(value string) { cfg.Browser.UserDataDir = value })
	if err := setEnvInt(lookup, meta, "ALEX_BROWSER_TIMEOUT_SECONDS", "browser.timeout_seconds", func(value int) { cfg.Browser.TimeoutSeconds = value }); err != nil {
		return err
	}
	setEnvString(lookup, meta, "ALEX_BROWSER_BRIDGE_LISTEN_ADDR", "browser.bridge_listen_addr", func(value string) { cfg.Browser.BridgeListen = value })
	setEnvString(lookup, meta, "ALEX_BROWSER_BRIDGE_TOKEN", "browser.bridge_token", func(value string) { cfg.Browser.BridgeToken = value })
	setEnvString(lookup, meta, "ALEX_PROFILE", "profile", func(value string) { cfg.Profile = value })
	setEnvString(lookup, meta, "ALEX_ENV", "environment", func(value string) { cfg.Environment = value })
	if err := setEnvBool(lookup, meta, "ALEX_VERBOSE", "verbose", func(value bool) { cfg.Verbose = value }); err != nil {
		return err
	}
	if err := setEnvBool(lookup, meta, "ALEX_NO_TUI", "disable_tui", func(value bool) { cfg.DisableTUI = value }); err != nil {
		return err
	}
	if err := setEnvBool(lookup, meta, "ALEX_TUI_FOLLOW_TRANSCRIPT", "follow_transcript", func(value bool) { cfg.FollowTranscript = value }); err != nil {
		return err
	}
	if err := setEnvBool(lookup, meta, "ALEX_TUI_FOLLOW_STREAM", "follow_stream", func(value bool) { cfg.FollowStream = value }); err != nil {
		return err
	}
	if err := setEnvInt(lookup, meta, "LLM_MAX_ITERATIONS", "max_iterations", func(value int) { cfg.MaxIterations = value }); err != nil {
		return err
	}
	if err := setEnvInt(lookup, meta, "LLM_MAX_TOKENS", "max_tokens", func(value int) { cfg.MaxTokens = value }); err != nil {
		return err
	}
	if err := setEnvInt(lookup, meta, "TOOL_MAX_CONCURRENT", "tool_max_concurrent", func(value int) { cfg.ToolMaxConcurrent = value }); err != nil {
		return err
	}
	if err := setEnvInt(lookup, meta, "LLM_CACHE_SIZE", "llm_cache_size", func(value int) { cfg.LLMCacheSize = value }); err != nil {
		return err
	}
	if err := setEnvInt(lookup, meta, "LLM_CACHE_TTL_SECONDS", "llm_cache_ttl_seconds", func(value int) { cfg.LLMCacheTTLSeconds = value }); err != nil {
		return err
	}
	if err := setEnvFloat(lookup, meta, "USER_LLM_RPS", "user_rate_limit_rps", func(value float64) { cfg.UserRateLimitRPS = value }); err != nil {
		return err
	}
	if err := setEnvInt(lookup, meta, "USER_LLM_BURST", "user_rate_limit_burst", func(value int) { cfg.UserRateLimitBurst = value }); err != nil {
		return err
	}
	if err := setEnvFloat(lookup, meta, "KIMI_LLM_RPS", "kimi_rate_limit_rps", func(value float64) { cfg.KimiRateLimitRPS = value }); err != nil {
		return err
	}
	if err := setEnvInt(lookup, meta, "KIMI_LLM_BURST", "kimi_rate_limit_burst", func(value int) { cfg.KimiRateLimitBurst = value }); err != nil {
		return err
	}
	if err := setEnvFloat(lookup, meta, "LLM_TEMPERATURE", "temperature", func(value float64) {
		cfg.Temperature = value
		cfg.TemperatureProvided = true
	}); err != nil {
		return err
	}
	if err := setEnvFloat(lookup, meta, "LLM_TOP_P", "top_p", func(value float64) { cfg.TopP = value }); err != nil {
		return err
	}
	setEnvString(lookup, meta, "LLM_STOP", "stop_sequences", func(value string) {
		cfg.StopSequences = parseStopSequences(value)
	})
	setEnvString(lookup, meta, "ALEX_SESSION_DIR", "session_dir", func(value string) { cfg.SessionDir = value })
	setEnvString(lookup, meta, "ALEX_COST_DIR", "cost_dir", func(value string) { cfg.CostDir = value })
	if err := setEnvDuration(lookup, meta, "AGENT_SESSION_STALE_AFTER", "session_stale_after_seconds", func(value int) { cfg.SessionStaleAfterSeconds = value }); err != nil {
		return err
	}

	return nil
}

func setEnvString(lookup EnvLookup, meta *Metadata, envKey, metaKey string, setter func(string)) {
	if value, ok := lookup(envKey); ok && value != "" {
		setter(value)
		meta.sources[metaKey] = SourceEnv
	}
}

func setEnvParsed[T any](
	lookup EnvLookup,
	meta *Metadata,
	envKey string,
	metaKey string,
	parser func(string) (T, error),
	setter func(T),
) error {
	value, ok := lookup(envKey)
	if !ok || value == "" {
		return nil
	}
	parsed, err := parser(value)
	if err != nil {
		return fmt.Errorf("parse %s: %w", envKey, err)
	}
	setter(parsed)
	meta.sources[metaKey] = SourceEnv
	return nil
}

func setEnvBool(lookup EnvLookup, meta *Metadata, envKey, metaKey string, setter func(bool)) error {
	return setEnvParsed(lookup, meta, envKey, metaKey, parseBoolEnv, setter)
}

func setEnvInt(lookup EnvLookup, meta *Metadata, envKey, metaKey string, setter func(int)) error {
	return setEnvParsed(lookup, meta, envKey, metaKey, strconv.Atoi, setter)
}

func setEnvFloat(lookup EnvLookup, meta *Metadata, envKey, metaKey string, setter func(float64)) error {
	return setEnvParsed(lookup, meta, envKey, metaKey, func(value string) (float64, error) {
		return strconv.ParseFloat(value, 64)
	}, setter)
}

func setEnvDuration(lookup EnvLookup, meta *Metadata, envKey, metaKey string, setter func(int)) error {
	return setEnvParsed(lookup, meta, envKey, metaKey, parseDurationSeconds, setter)
}

func parseStopSequences(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case ',', ';', ' ', '\n', '\t':
			return true
		default:
			return false
		}
	})
	filtered := parts[:0]
	for _, token := range parts {
		trimmed := strings.TrimSpace(token)
		if trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	return append([]string(nil), filtered...)
}

func parseBoolEnv(value string) (bool, error) {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	switch lower {
	case "1", "true", "t", "yes", "y", "on":
		return true, nil
	case "0", "false", "f", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value %q", value)
	}
}
