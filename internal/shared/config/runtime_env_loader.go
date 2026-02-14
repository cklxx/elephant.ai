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

	if value, ok := lookup("ARK_API_KEY"); ok && value != "" {
		cfg.ArkAPIKey = value
		meta.sources["ark_api_key"] = SourceEnv
	}
	if value, ok := lookup("LLM_PROVIDER"); ok && value != "" {
		cfg.LLMProvider = value
		meta.sources["llm_provider"] = SourceEnv
	}
	if value, ok := lookup("LLM_MODEL"); ok && value != "" {
		cfg.LLMModel = value
		meta.sources["llm_model"] = SourceEnv
	}
	if value, ok := lookup("LLM_SMALL_PROVIDER"); ok && value != "" {
		cfg.LLMSmallProvider = value
		meta.sources["llm_small_provider"] = SourceEnv
	}
	if value, ok := lookup("LLM_SMALL_MODEL"); ok && value != "" {
		cfg.LLMSmallModel = value
		meta.sources["llm_small_model"] = SourceEnv
	}
	if value, ok := lookup("LLM_VISION_MODEL"); ok && value != "" {
		cfg.LLMVisionModel = value
		meta.sources["llm_vision_model"] = SourceEnv
	}
	if value, ok := lookup("LLM_BASE_URL"); ok && value != "" {
		cfg.BaseURL = value
		meta.sources["base_url"] = SourceEnv
	}
	if value, ok := lookup("ACP_EXECUTOR_ADDR"); ok && value != "" {
		cfg.ACPExecutorAddr = value
		meta.sources["acp_executor_addr"] = SourceEnv
	}
	if value, ok := lookup("ACP_EXECUTOR_CWD"); ok && value != "" {
		cfg.ACPExecutorCWD = value
		meta.sources["acp_executor_cwd"] = SourceEnv
	}
	if value, ok := lookup("ACP_EXECUTOR_MODE"); ok && value != "" {
		cfg.ACPExecutorMode = value
		meta.sources["acp_executor_mode"] = SourceEnv
	}
	if value, ok := lookup("ACP_EXECUTOR_AUTO_APPROVE"); ok && value != "" {
		parsed, err := parseBoolEnv(value)
		if err != nil {
			return fmt.Errorf("parse ACP_EXECUTOR_AUTO_APPROVE: %w", err)
		}
		cfg.ACPExecutorAutoApprove = parsed
		meta.sources["acp_executor_auto_approve"] = SourceEnv
	}
	if value, ok := lookup("ACP_EXECUTOR_MAX_CLI_CALLS"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse ACP_EXECUTOR_MAX_CLI_CALLS: %w", err)
		}
		cfg.ACPExecutorMaxCLICalls = parsed
		meta.sources["acp_executor_max_cli_calls"] = SourceEnv
	}
	if value, ok := lookup("ACP_EXECUTOR_MAX_DURATION_SECONDS"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse ACP_EXECUTOR_MAX_DURATION_SECONDS: %w", err)
		}
		cfg.ACPExecutorMaxDuration = parsed
		meta.sources["acp_executor_max_duration_seconds"] = SourceEnv
	}
	if value, ok := lookup("ACP_EXECUTOR_REQUIRE_MANIFEST"); ok && value != "" {
		parsed, err := parseBoolEnv(value)
		if err != nil {
			return fmt.Errorf("parse ACP_EXECUTOR_REQUIRE_MANIFEST: %w", err)
		}
		cfg.ACPExecutorRequireManifest = parsed
		meta.sources["acp_executor_require_manifest"] = SourceEnv
	}
	if value, ok := lookup("TAVILY_API_KEY"); ok && value != "" {
		cfg.TavilyAPIKey = value
		meta.sources["tavily_api_key"] = SourceEnv
	}
	if value, ok := lookup("MOLTBOOK_API_KEY"); ok && value != "" {
		cfg.MoltbookAPIKey = value
		meta.sources["moltbook_api_key"] = SourceEnv
	}
	if value, ok := lookup("MOLTBOOK_BASE_URL"); ok && value != "" {
		cfg.MoltbookBaseURL = value
		meta.sources["moltbook_base_url"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_TEXT_ENDPOINT_ID"); ok && value != "" {
		cfg.SeedreamTextEndpointID = value
		meta.sources["seedream_text_endpoint_id"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_IMAGE_ENDPOINT_ID"); ok && value != "" {
		cfg.SeedreamImageEndpointID = value
		meta.sources["seedream_image_endpoint_id"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_TEXT_MODEL"); ok && value != "" {
		cfg.SeedreamTextModel = value
		meta.sources["seedream_text_model"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_IMAGE_MODEL"); ok && value != "" {
		cfg.SeedreamImageModel = value
		meta.sources["seedream_image_model"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_VISION_MODEL"); ok && value != "" {
		cfg.SeedreamVisionModel = value
		meta.sources["seedream_vision_model"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_VIDEO_MODEL"); ok && value != "" {
		cfg.SeedreamVideoModel = value
		meta.sources["seedream_video_model"] = SourceEnv
	}
	if value, ok := lookup("AGENT_PRESET"); ok && value != "" {
		cfg.AgentPreset = value
		meta.sources["agent_preset"] = SourceEnv
	}
	if value, ok := lookup("TOOL_PRESET"); ok && value != "" {
		cfg.ToolPreset = value
		meta.sources["tool_preset"] = SourceEnv
	}
	if value, ok := lookup("ALEX_TOOLSET"); ok && value != "" {
		cfg.Toolset = value
		meta.sources["toolset"] = SourceEnv
	}
	if value, ok := lookup("ALEX_BROWSER_CONNECTOR"); ok && value != "" {
		cfg.Browser.Connector = value
		meta.sources["browser.connector"] = SourceEnv
	}
	if value, ok := lookup("ALEX_BROWSER_CDP_URL"); ok && value != "" {
		cfg.Browser.CDPURL = value
		meta.sources["browser.cdp_url"] = SourceEnv
	}
	if value, ok := lookup("ALEX_BROWSER_CHROME_PATH"); ok && value != "" {
		cfg.Browser.ChromePath = value
		meta.sources["browser.chrome_path"] = SourceEnv
	}
	if value, ok := lookup("ALEX_BROWSER_HEADLESS"); ok && value != "" {
		parsed, err := parseBoolEnv(value)
		if err != nil {
			return fmt.Errorf("parse ALEX_BROWSER_HEADLESS: %w", err)
		}
		cfg.Browser.Headless = parsed
		meta.sources["browser.headless"] = SourceEnv
	}
	if value, ok := lookup("ALEX_BROWSER_USER_DATA_DIR"); ok && value != "" {
		cfg.Browser.UserDataDir = value
		meta.sources["browser.user_data_dir"] = SourceEnv
	}
	if value, ok := lookup("ALEX_BROWSER_TIMEOUT_SECONDS"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse ALEX_BROWSER_TIMEOUT_SECONDS: %w", err)
		}
		cfg.Browser.TimeoutSeconds = parsed
		meta.sources["browser.timeout_seconds"] = SourceEnv
	}
	if value, ok := lookup("ALEX_BROWSER_BRIDGE_LISTEN_ADDR"); ok && value != "" {
		cfg.Browser.BridgeListen = value
		meta.sources["browser.bridge_listen_addr"] = SourceEnv
	}
	if value, ok := lookup("ALEX_BROWSER_BRIDGE_TOKEN"); ok && value != "" {
		cfg.Browser.BridgeToken = value
		meta.sources["browser.bridge_token"] = SourceEnv
	}
	if value, ok := lookup("ALEX_PROFILE"); ok && value != "" {
		cfg.Profile = value
		meta.sources["profile"] = SourceEnv
	}
	if value, ok := lookup("ALEX_ENV"); ok && value != "" {
		cfg.Environment = value
		meta.sources["environment"] = SourceEnv
	}
	if value, ok := lookup("ALEX_VERBOSE"); ok && value != "" {
		parsed, err := parseBoolEnv(value)
		if err != nil {
			return fmt.Errorf("parse ALEX_VERBOSE: %w", err)
		}
		cfg.Verbose = parsed
		meta.sources["verbose"] = SourceEnv
	}
	if value, ok := lookup("ALEX_NO_TUI"); ok && value != "" {
		parsed, err := parseBoolEnv(value)
		if err != nil {
			return fmt.Errorf("parse ALEX_NO_TUI: %w", err)
		}
		cfg.DisableTUI = parsed
		meta.sources["disable_tui"] = SourceEnv
	}
	if value, ok := lookup("ALEX_TUI_FOLLOW_TRANSCRIPT"); ok && value != "" {
		parsed, err := parseBoolEnv(value)
		if err != nil {
			return fmt.Errorf("parse ALEX_TUI_FOLLOW_TRANSCRIPT: %w", err)
		}
		cfg.FollowTranscript = parsed
		meta.sources["follow_transcript"] = SourceEnv
	}
	if value, ok := lookup("ALEX_TUI_FOLLOW_STREAM"); ok && value != "" {
		parsed, err := parseBoolEnv(value)
		if err != nil {
			return fmt.Errorf("parse ALEX_TUI_FOLLOW_STREAM: %w", err)
		}
		cfg.FollowStream = parsed
		meta.sources["follow_stream"] = SourceEnv
	}
	if value, ok := lookup("LLM_MAX_ITERATIONS"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse LLM_MAX_ITERATIONS: %w", err)
		}
		cfg.MaxIterations = parsed
		meta.sources["max_iterations"] = SourceEnv
	}
	if value, ok := lookup("LLM_MAX_TOKENS"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse LLM_MAX_TOKENS: %w", err)
		}
		cfg.MaxTokens = parsed
		meta.sources["max_tokens"] = SourceEnv
	}
	if value, ok := lookup("TOOL_MAX_CONCURRENT"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse TOOL_MAX_CONCURRENT: %w", err)
		}
		cfg.ToolMaxConcurrent = parsed
		meta.sources["tool_max_concurrent"] = SourceEnv
	}
	if value, ok := lookup("LLM_CACHE_SIZE"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse LLM_CACHE_SIZE: %w", err)
		}
		cfg.LLMCacheSize = parsed
		meta.sources["llm_cache_size"] = SourceEnv
	}
	if value, ok := lookup("LLM_CACHE_TTL_SECONDS"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse LLM_CACHE_TTL_SECONDS: %w", err)
		}
		cfg.LLMCacheTTLSeconds = parsed
		meta.sources["llm_cache_ttl_seconds"] = SourceEnv
	}
	if value, ok := lookup("USER_LLM_RPS"); ok && value != "" {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("parse USER_LLM_RPS: %w", err)
		}
		cfg.UserRateLimitRPS = parsed
		meta.sources["user_rate_limit_rps"] = SourceEnv
	}
	if value, ok := lookup("USER_LLM_BURST"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse USER_LLM_BURST: %w", err)
		}
		cfg.UserRateLimitBurst = parsed
		meta.sources["user_rate_limit_burst"] = SourceEnv
	}
	if value, ok := lookup("LLM_TEMPERATURE"); ok && value != "" {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("parse LLM_TEMPERATURE: %w", err)
		}
		cfg.Temperature = parsed
		cfg.TemperatureProvided = true
		meta.sources["temperature"] = SourceEnv
	}
	if value, ok := lookup("LLM_TOP_P"); ok && value != "" {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("parse LLM_TOP_P: %w", err)
		}
		cfg.TopP = parsed
		meta.sources["top_p"] = SourceEnv
	}
	if value, ok := lookup("LLM_STOP"); ok && value != "" {
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
		cfg.StopSequences = append([]string(nil), filtered...)
		meta.sources["stop_sequences"] = SourceEnv
	}
	if value, ok := lookup("ALEX_SESSION_DIR"); ok && value != "" {
		cfg.SessionDir = value
		meta.sources["session_dir"] = SourceEnv
	}
	if value, ok := lookup("ALEX_COST_DIR"); ok && value != "" {
		cfg.CostDir = value
		meta.sources["cost_dir"] = SourceEnv
	}
	if value, ok := lookup("AGENT_SESSION_STALE_AFTER"); ok && value != "" {
		seconds, err := parseDurationSeconds(value)
		if err != nil {
			return fmt.Errorf("parse AGENT_SESSION_STALE_AFTER: %w", err)
		}
		cfg.SessionStaleAfterSeconds = seconds
		meta.sources["session_stale_after_seconds"] = SourceEnv
	}

	return nil
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
