package config

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"os"
	"strconv"
	"strings"
	"time"

	"alex/internal/shared/utils"
	"gopkg.in/yaml.v3"
)

func applyFile(cfg *RuntimeConfig, meta *Metadata, opts loadOptions) error {
	configPath := strings.TrimSpace(opts.configPath)
	if configPath == "" {
		configPath, _ = ResolveConfigPath(opts.envLookup, opts.homeDir)
	}
	if configPath == "" {
		return nil
	}

	data, err := opts.readFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read config file: %w", err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}

	parsed, err := parseRuntimeConfigYAML(data)
	if err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}
	lookup := opts.envLookup
	if lookup == nil {
		lookup = DefaultEnvLookup
	}
	parsed = expandRuntimeFileConfigEnv(lookup, parsed)

	applyString := func(value string, target *string, key string) {
		if value == "" {
			return
		}
		*target = value
		meta.sources[key] = SourceFile
	}
	applyBool := func(value *bool, target *bool, key string) {
		if value == nil {
			return
		}
		*target = *value
		meta.sources[key] = SourceFile
	}
	applyInt := func(value *int, target *int, key string) {
		if value == nil {
			return
		}
		*target = *value
		meta.sources[key] = SourceFile
	}
	applyFloat := func(value *float64, target *float64, key string) {
		if value == nil {
			return
		}
		*target = *value
		meta.sources[key] = SourceFile
	}

	for _, field := range []struct {
		value  string
		target *string
		key    string
	}{
		{parsed.APIKey, &cfg.APIKey, "api_key"},
		{parsed.ArkAPIKey, &cfg.ArkAPIKey, "ark_api_key"},
		{parsed.LLMProvider, &cfg.LLMProvider, "llm_provider"},
		{parsed.LLMModel, &cfg.LLMModel, "llm_model"},
		{parsed.LLMVisionModel, &cfg.LLMVisionModel, "llm_vision_model"},
		{parsed.BaseURL, &cfg.BaseURL, "base_url"},
		{parsed.ACPExecutorAddr, &cfg.ACPExecutorAddr, "acp_executor_addr"},
		{parsed.ACPExecutorCWD, &cfg.ACPExecutorCWD, "acp_executor_cwd"},
		{parsed.ACPExecutorMode, &cfg.ACPExecutorMode, "acp_executor_mode"},
		{parsed.TavilyAPIKey, &cfg.TavilyAPIKey, "tavily_api_key"},
		{parsed.MoltbookAPIKey, &cfg.MoltbookAPIKey, "moltbook_api_key"},
		{parsed.MoltbookBaseURL, &cfg.MoltbookBaseURL, "moltbook_base_url"},
		{parsed.SeedreamTextEndpointID, &cfg.SeedreamTextEndpointID, "seedream_text_endpoint_id"},
		{parsed.SeedreamImageEndpointID, &cfg.SeedreamImageEndpointID, "seedream_image_endpoint_id"},
		{parsed.SeedreamTextModel, &cfg.SeedreamTextModel, "seedream_text_model"},
		{parsed.SeedreamImageModel, &cfg.SeedreamImageModel, "seedream_image_model"},
		{parsed.SeedreamVisionModel, &cfg.SeedreamVisionModel, "seedream_vision_model"},
		{parsed.SeedreamVideoModel, &cfg.SeedreamVideoModel, "seedream_video_model"},
		{parsed.Profile, &cfg.Profile, "profile"},
		{parsed.Environment, &cfg.Environment, "environment"},
		{parsed.SessionDir, &cfg.SessionDir, "session_dir"},
		{parsed.CostDir, &cfg.CostDir, "cost_dir"},
		{parsed.AgentPreset, &cfg.AgentPreset, "agent_preset"},
		{parsed.ToolPreset, &cfg.ToolPreset, "tool_preset"},
		{parsed.Toolset, &cfg.Toolset, "toolset"},
	} {
		applyString(field.value, field.target, field.key)
	}

	for _, field := range []struct {
		value  *bool
		target *bool
		key    string
	}{
		{parsed.ACPExecutorAutoApprove, &cfg.ACPExecutorAutoApprove, "acp_executor_auto_approve"},
		{parsed.ACPExecutorRequireManifest, &cfg.ACPExecutorRequireManifest, "acp_executor_require_manifest"},
		{parsed.Verbose, &cfg.Verbose, "verbose"},
		{parsed.DisableTUI, &cfg.DisableTUI, "disable_tui"},
		{parsed.FollowTranscript, &cfg.FollowTranscript, "follow_transcript"},
		{parsed.FollowStream, &cfg.FollowStream, "follow_stream"},
	} {
		applyBool(field.value, field.target, field.key)
	}

	for _, field := range []struct {
		value  *int
		target *int
		key    string
	}{
		{parsed.ACPExecutorMaxCLICalls, &cfg.ACPExecutorMaxCLICalls, "acp_executor_max_cli_calls"},
		{parsed.ACPExecutorMaxDuration, &cfg.ACPExecutorMaxDuration, "acp_executor_max_duration_seconds"},
		{parsed.MaxIterations, &cfg.MaxIterations, "max_iterations"},
		{parsed.MaxTokens, &cfg.MaxTokens, "max_tokens"},
		{parsed.ToolMaxConcurrent, &cfg.ToolMaxConcurrent, "tool_max_concurrent"},
		{parsed.LLMCacheSize, &cfg.LLMCacheSize, "llm_cache_size"},
		{parsed.LLMCacheTTLSeconds, &cfg.LLMCacheTTLSeconds, "llm_cache_ttl_seconds"},
		{parsed.UserRateLimitBurst, &cfg.UserRateLimitBurst, "user_rate_limit_burst"},
		{parsed.KimiRateLimitBurst, &cfg.KimiRateLimitBurst, "kimi_rate_limit_burst"},
	} {
		applyInt(field.value, field.target, field.key)
	}

	if parsed.LLMRequestTimeoutSeconds != nil && *parsed.LLMRequestTimeoutSeconds > 0 {
		cfg.LLMRequestTimeoutSeconds = *parsed.LLMRequestTimeoutSeconds
		meta.sources["llm_request_timeout_seconds"] = SourceFile
	}

	for _, field := range []struct {
		value  *float64
		target *float64
		key    string
	}{
		{parsed.UserRateLimitRPS, &cfg.UserRateLimitRPS, "user_rate_limit_rps"},
		{parsed.KimiRateLimitRPS, &cfg.KimiRateLimitRPS, "kimi_rate_limit_rps"},
		{parsed.TopP, &cfg.TopP, "top_p"},
	} {
		applyFloat(field.value, field.target, field.key)
	}

	if parsed.Temperature != nil {
		cfg.Temperature = *parsed.Temperature
		cfg.TemperatureProvided = true
		meta.sources["temperature"] = SourceFile
	}
	if len(parsed.StopSequences) > 0 {
		cfg.StopSequences = append([]string(nil), parsed.StopSequences...)
		meta.sources["stop_sequences"] = SourceFile
	}
	if parsed.SessionStaleAfter != "" {
		seconds, err := parseDurationSeconds(parsed.SessionStaleAfter)
		if err != nil {
			return fmt.Errorf("parse session_stale_after: %w", err)
		}
		cfg.SessionStaleAfterSeconds = seconds
		meta.sources["session_stale_after_seconds"] = SourceFile
	}
	if parsed.Browser != nil {
		for _, field := range []struct {
			value  string
			target *string
			key    string
		}{
			{strings.TrimSpace(parsed.Browser.Connector), &cfg.Browser.Connector, "browser.connector"},
			{strings.TrimSpace(parsed.Browser.CDPURL), &cfg.Browser.CDPURL, "browser.cdp_url"},
			{strings.TrimSpace(parsed.Browser.ChromePath), &cfg.Browser.ChromePath, "browser.chrome_path"},
			{strings.TrimSpace(parsed.Browser.UserDataDir), &cfg.Browser.UserDataDir, "browser.user_data_dir"},
			{strings.TrimSpace(parsed.Browser.BridgeListen), &cfg.Browser.BridgeListen, "browser.bridge_listen_addr"},
			{strings.TrimSpace(parsed.Browser.BridgeToken), &cfg.Browser.BridgeToken, "browser.bridge_token"},
		} {
			applyString(field.value, field.target, field.key)
		}
		if parsed.Browser.Headless != nil {
			cfg.Browser.Headless = *parsed.Browser.Headless
			meta.sources["browser.headless"] = SourceFile
		}
		if parsed.Browser.TimeoutSeconds != nil && *parsed.Browser.TimeoutSeconds > 0 {
			cfg.Browser.TimeoutSeconds = *parsed.Browser.TimeoutSeconds
			meta.sources["browser.timeout_seconds"] = SourceFile
		}
	}
	if parsed.ToolPolicy != nil {
		applyToolPolicyFileConfig(cfg, meta, parsed.ToolPolicy)
	}
	if parsed.HTTPLimits != nil {
		applyHTTPLimitsFileConfig(cfg, meta, parsed.HTTPLimits)
	}
	if parsed.Proactive != nil {
		applyProactiveFileConfig(cfg, meta, parsed.Proactive)
	}
	if parsed.ExternalAgents != nil {
		if err := applyExternalAgentsFileConfig(cfg, meta, parsed.ExternalAgents); err != nil {
			return err
		}
	}

	var fileCfg FileConfig
	if err := yaml.Unmarshal(data, &fileCfg); err == nil {
		fileCfg = expandFileConfigEnv(lookup, fileCfg)
		if fileCfg.Agent != nil && utils.HasContent(fileCfg.Agent.SessionStaleAfter) {
			seconds, err := parseDurationSeconds(fileCfg.Agent.SessionStaleAfter)
			if err != nil {
				return fmt.Errorf("parse agent.session_stale_after: %w", err)
			}
			cfg.SessionStaleAfterSeconds = seconds
			meta.sources["session_stale_after_seconds"] = SourceFile
		}
	}

	return nil
}

type runtimeFile struct {
	Runtime *RuntimeFileConfig `yaml:"runtime"`
}

func parseRuntimeConfigYAML(data []byte) (RuntimeFileConfig, error) {
	var wrapped runtimeFile
	if err := yaml.Unmarshal(data, &wrapped); err != nil {
		return RuntimeFileConfig{}, err
	}
	if wrapped.Runtime != nil {
		return *wrapped.Runtime, nil
	}

	var parsed RuntimeFileConfig
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return RuntimeFileConfig{}, err
	}
	return parsed, nil
}

func parseDurationSeconds(value string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	if seconds, err := strconv.Atoi(trimmed); err == nil {
		return seconds, nil
	}
	parsed, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, err
	}
	return int(parsed.Seconds()), nil
}

func parseDuration(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	if seconds, err := strconv.Atoi(trimmed); err == nil {
		return time.Duration(seconds) * time.Second, nil
	}
	return time.ParseDuration(trimmed)
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	return maps.Clone(in)
}

func cloneDurationMap(in map[string]time.Duration) map[string]time.Duration {
	if len(in) == 0 {
		return nil
	}
	return maps.Clone(in)
}
