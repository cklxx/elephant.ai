package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

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

	if parsed.APIKey != "" {
		cfg.APIKey = parsed.APIKey
		meta.sources["api_key"] = SourceFile
	}
	if parsed.ArkAPIKey != "" {
		cfg.ArkAPIKey = parsed.ArkAPIKey
		meta.sources["ark_api_key"] = SourceFile
	}
	if parsed.LLMProvider != "" {
		cfg.LLMProvider = parsed.LLMProvider
		meta.sources["llm_provider"] = SourceFile
	}
	if parsed.LLMModel != "" {
		cfg.LLMModel = parsed.LLMModel
		meta.sources["llm_model"] = SourceFile
	}
	if parsed.LLMSmallProvider != "" {
		cfg.LLMSmallProvider = parsed.LLMSmallProvider
		meta.sources["llm_small_provider"] = SourceFile
	}
	if parsed.LLMSmallModel != "" {
		cfg.LLMSmallModel = parsed.LLMSmallModel
		meta.sources["llm_small_model"] = SourceFile
	}
	if parsed.LLMVisionModel != "" {
		cfg.LLMVisionModel = parsed.LLMVisionModel
		meta.sources["llm_vision_model"] = SourceFile
	}
	if parsed.BaseURL != "" {
		cfg.BaseURL = parsed.BaseURL
		meta.sources["base_url"] = SourceFile
	}
	if parsed.SandboxBaseURL != "" {
		cfg.SandboxBaseURL = parsed.SandboxBaseURL
		meta.sources["sandbox_base_url"] = SourceFile
	}
	if parsed.ACPExecutorAddr != "" {
		cfg.ACPExecutorAddr = parsed.ACPExecutorAddr
		meta.sources["acp_executor_addr"] = SourceFile
	}
	if parsed.ACPExecutorCWD != "" {
		cfg.ACPExecutorCWD = parsed.ACPExecutorCWD
		meta.sources["acp_executor_cwd"] = SourceFile
	}
	if parsed.ACPExecutorMode != "" {
		cfg.ACPExecutorMode = parsed.ACPExecutorMode
		meta.sources["acp_executor_mode"] = SourceFile
	}
	if parsed.ACPExecutorAutoApprove != nil {
		cfg.ACPExecutorAutoApprove = *parsed.ACPExecutorAutoApprove
		meta.sources["acp_executor_auto_approve"] = SourceFile
	}
	if parsed.ACPExecutorMaxCLICalls != nil {
		cfg.ACPExecutorMaxCLICalls = *parsed.ACPExecutorMaxCLICalls
		meta.sources["acp_executor_max_cli_calls"] = SourceFile
	}
	if parsed.ACPExecutorMaxDuration != nil {
		cfg.ACPExecutorMaxDuration = *parsed.ACPExecutorMaxDuration
		meta.sources["acp_executor_max_duration_seconds"] = SourceFile
	}
	if parsed.ACPExecutorRequireManifest != nil {
		cfg.ACPExecutorRequireManifest = *parsed.ACPExecutorRequireManifest
		meta.sources["acp_executor_require_manifest"] = SourceFile
	}
	if parsed.TavilyAPIKey != "" {
		cfg.TavilyAPIKey = parsed.TavilyAPIKey
		meta.sources["tavily_api_key"] = SourceFile
	}
	if parsed.SeedreamTextEndpointID != "" {
		cfg.SeedreamTextEndpointID = parsed.SeedreamTextEndpointID
		meta.sources["seedream_text_endpoint_id"] = SourceFile
	}
	if parsed.SeedreamImageEndpointID != "" {
		cfg.SeedreamImageEndpointID = parsed.SeedreamImageEndpointID
		meta.sources["seedream_image_endpoint_id"] = SourceFile
	}
	if parsed.SeedreamTextModel != "" {
		cfg.SeedreamTextModel = parsed.SeedreamTextModel
		meta.sources["seedream_text_model"] = SourceFile
	}
	if parsed.SeedreamImageModel != "" {
		cfg.SeedreamImageModel = parsed.SeedreamImageModel
		meta.sources["seedream_image_model"] = SourceFile
	}
	if parsed.SeedreamVisionModel != "" {
		cfg.SeedreamVisionModel = parsed.SeedreamVisionModel
		meta.sources["seedream_vision_model"] = SourceFile
	}
	if parsed.SeedreamVideoModel != "" {
		cfg.SeedreamVideoModel = parsed.SeedreamVideoModel
		meta.sources["seedream_video_model"] = SourceFile
	}
	if parsed.Environment != "" {
		cfg.Environment = parsed.Environment
		meta.sources["environment"] = SourceFile
	}
	if parsed.Verbose != nil {
		cfg.Verbose = *parsed.Verbose
		meta.sources["verbose"] = SourceFile
	}
	if parsed.DisableTUI != nil {
		cfg.DisableTUI = *parsed.DisableTUI
		meta.sources["disable_tui"] = SourceFile
	}
	if parsed.FollowTranscript != nil {
		cfg.FollowTranscript = *parsed.FollowTranscript
		meta.sources["follow_transcript"] = SourceFile
	}
	if parsed.FollowStream != nil {
		cfg.FollowStream = *parsed.FollowStream
		meta.sources["follow_stream"] = SourceFile
	}
	if parsed.MaxIterations != nil {
		cfg.MaxIterations = *parsed.MaxIterations
		meta.sources["max_iterations"] = SourceFile
	}
	if parsed.MaxTokens != nil {
		cfg.MaxTokens = *parsed.MaxTokens
		meta.sources["max_tokens"] = SourceFile
	}
	if parsed.ToolMaxConcurrent != nil {
		cfg.ToolMaxConcurrent = *parsed.ToolMaxConcurrent
		meta.sources["tool_max_concurrent"] = SourceFile
	}
	if parsed.LLMCacheSize != nil {
		cfg.LLMCacheSize = *parsed.LLMCacheSize
		meta.sources["llm_cache_size"] = SourceFile
	}
	if parsed.LLMCacheTTLSeconds != nil {
		cfg.LLMCacheTTLSeconds = *parsed.LLMCacheTTLSeconds
		meta.sources["llm_cache_ttl_seconds"] = SourceFile
	}
	if parsed.UserRateLimitRPS != nil {
		cfg.UserRateLimitRPS = *parsed.UserRateLimitRPS
		meta.sources["user_rate_limit_rps"] = SourceFile
	}
	if parsed.UserRateLimitBurst != nil {
		cfg.UserRateLimitBurst = *parsed.UserRateLimitBurst
		meta.sources["user_rate_limit_burst"] = SourceFile
	}
	if parsed.Temperature != nil {
		cfg.Temperature = *parsed.Temperature
		cfg.TemperatureProvided = true
		meta.sources["temperature"] = SourceFile
	}
	if parsed.TopP != nil {
		cfg.TopP = *parsed.TopP
		meta.sources["top_p"] = SourceFile
	}
	if len(parsed.StopSequences) > 0 {
		cfg.StopSequences = append([]string(nil), parsed.StopSequences...)
		meta.sources["stop_sequences"] = SourceFile
	}
	if parsed.SessionDir != "" {
		cfg.SessionDir = parsed.SessionDir
		meta.sources["session_dir"] = SourceFile
	}
	if parsed.CostDir != "" {
		cfg.CostDir = parsed.CostDir
		meta.sources["cost_dir"] = SourceFile
	}
	if parsed.AgentPreset != "" {
		cfg.AgentPreset = parsed.AgentPreset
		meta.sources["agent_preset"] = SourceFile
	}
	if parsed.ToolPreset != "" {
		cfg.ToolPreset = parsed.ToolPreset
		meta.sources["tool_preset"] = SourceFile
	}
	if parsed.Proactive != nil {
		applyProactiveFileConfig(cfg, meta, parsed.Proactive)
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

func expandRuntimeFileConfigEnv(lookup EnvLookup, parsed RuntimeFileConfig) RuntimeFileConfig {
	parsed.LLMProvider = expandEnvValue(lookup, parsed.LLMProvider)
	parsed.LLMModel = expandEnvValue(lookup, parsed.LLMModel)
	parsed.LLMSmallProvider = expandEnvValue(lookup, parsed.LLMSmallProvider)
	parsed.LLMSmallModel = expandEnvValue(lookup, parsed.LLMSmallModel)
	parsed.LLMVisionModel = expandEnvValue(lookup, parsed.LLMVisionModel)
	parsed.APIKey = expandEnvValue(lookup, parsed.APIKey)
	parsed.ArkAPIKey = expandEnvValue(lookup, parsed.ArkAPIKey)
	parsed.BaseURL = expandEnvValue(lookup, parsed.BaseURL)
	parsed.SandboxBaseURL = expandEnvValue(lookup, parsed.SandboxBaseURL)
	parsed.ACPExecutorAddr = expandEnvValue(lookup, parsed.ACPExecutorAddr)
	parsed.ACPExecutorCWD = expandEnvValue(lookup, parsed.ACPExecutorCWD)
	parsed.ACPExecutorMode = expandEnvValue(lookup, parsed.ACPExecutorMode)
	parsed.TavilyAPIKey = expandEnvValue(lookup, parsed.TavilyAPIKey)
	parsed.SeedreamTextEndpointID = expandEnvValue(lookup, parsed.SeedreamTextEndpointID)
	parsed.SeedreamImageEndpointID = expandEnvValue(lookup, parsed.SeedreamImageEndpointID)
	parsed.SeedreamTextModel = expandEnvValue(lookup, parsed.SeedreamTextModel)
	parsed.SeedreamImageModel = expandEnvValue(lookup, parsed.SeedreamImageModel)
	parsed.SeedreamVisionModel = expandEnvValue(lookup, parsed.SeedreamVisionModel)
	parsed.SeedreamVideoModel = expandEnvValue(lookup, parsed.SeedreamVideoModel)
	parsed.Environment = expandEnvValue(lookup, parsed.Environment)
	parsed.SessionDir = expandEnvValue(lookup, parsed.SessionDir)
	parsed.CostDir = expandEnvValue(lookup, parsed.CostDir)
	parsed.AgentPreset = expandEnvValue(lookup, parsed.AgentPreset)
	parsed.ToolPreset = expandEnvValue(lookup, parsed.ToolPreset)
	if parsed.Proactive != nil {
		expandProactiveFileConfigEnv(lookup, parsed.Proactive)
	}

	if len(parsed.StopSequences) > 0 {
		expanded := make([]string, 0, len(parsed.StopSequences))
		for _, seq := range parsed.StopSequences {
			expanded = append(expanded, expandEnvValue(lookup, seq))
		}
		parsed.StopSequences = expanded
	}

	return parsed
}
