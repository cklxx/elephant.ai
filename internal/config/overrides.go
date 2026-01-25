package config

func applyOverrides(cfg *RuntimeConfig, meta *Metadata, overrides Overrides) {
	if overrides.LLMProvider != nil {
		cfg.LLMProvider = *overrides.LLMProvider
		meta.sources["llm_provider"] = SourceOverride
	}
	if overrides.LLMModel != nil {
		cfg.LLMModel = *overrides.LLMModel
		meta.sources["llm_model"] = SourceOverride
	}
	if overrides.LLMSmallProvider != nil {
		cfg.LLMSmallProvider = *overrides.LLMSmallProvider
		meta.sources["llm_small_provider"] = SourceOverride
	}
	if overrides.LLMSmallModel != nil {
		cfg.LLMSmallModel = *overrides.LLMSmallModel
		meta.sources["llm_small_model"] = SourceOverride
	}
	if overrides.LLMVisionModel != nil {
		cfg.LLMVisionModel = *overrides.LLMVisionModel
		meta.sources["llm_vision_model"] = SourceOverride
	}
	if overrides.APIKey != nil {
		cfg.APIKey = *overrides.APIKey
		meta.sources["api_key"] = SourceOverride
	}
	if overrides.ArkAPIKey != nil {
		cfg.ArkAPIKey = *overrides.ArkAPIKey
		meta.sources["ark_api_key"] = SourceOverride
	}
	if overrides.BaseURL != nil {
		cfg.BaseURL = *overrides.BaseURL
		meta.sources["base_url"] = SourceOverride
	}
	if overrides.SandboxBaseURL != nil {
		cfg.SandboxBaseURL = *overrides.SandboxBaseURL
		meta.sources["sandbox_base_url"] = SourceOverride
	}
	if overrides.ACPExecutorAddr != nil {
		cfg.ACPExecutorAddr = *overrides.ACPExecutorAddr
		meta.sources["acp_executor_addr"] = SourceOverride
	}
	if overrides.ACPExecutorCWD != nil {
		cfg.ACPExecutorCWD = *overrides.ACPExecutorCWD
		meta.sources["acp_executor_cwd"] = SourceOverride
	}
	if overrides.ACPExecutorMode != nil {
		cfg.ACPExecutorMode = *overrides.ACPExecutorMode
		meta.sources["acp_executor_mode"] = SourceOverride
	}
	if overrides.ACPExecutorAutoApprove != nil {
		cfg.ACPExecutorAutoApprove = *overrides.ACPExecutorAutoApprove
		meta.sources["acp_executor_auto_approve"] = SourceOverride
	}
	if overrides.ACPExecutorMaxCLICalls != nil {
		cfg.ACPExecutorMaxCLICalls = *overrides.ACPExecutorMaxCLICalls
		meta.sources["acp_executor_max_cli_calls"] = SourceOverride
	}
	if overrides.ACPExecutorMaxDuration != nil {
		cfg.ACPExecutorMaxDuration = *overrides.ACPExecutorMaxDuration
		meta.sources["acp_executor_max_duration_seconds"] = SourceOverride
	}
	if overrides.ACPExecutorRequireManifest != nil {
		cfg.ACPExecutorRequireManifest = *overrides.ACPExecutorRequireManifest
		meta.sources["acp_executor_require_manifest"] = SourceOverride
	}
	if overrides.TavilyAPIKey != nil {
		cfg.TavilyAPIKey = *overrides.TavilyAPIKey
		meta.sources["tavily_api_key"] = SourceOverride
	}
	if overrides.SeedreamTextEndpointID != nil {
		cfg.SeedreamTextEndpointID = *overrides.SeedreamTextEndpointID
		meta.sources["seedream_text_endpoint_id"] = SourceOverride
	}
	if overrides.SeedreamImageEndpointID != nil {
		cfg.SeedreamImageEndpointID = *overrides.SeedreamImageEndpointID
		meta.sources["seedream_image_endpoint_id"] = SourceOverride
	}
	if overrides.SeedreamTextModel != nil {
		cfg.SeedreamTextModel = *overrides.SeedreamTextModel
		meta.sources["seedream_text_model"] = SourceOverride
	}
	if overrides.SeedreamImageModel != nil {
		cfg.SeedreamImageModel = *overrides.SeedreamImageModel
		meta.sources["seedream_image_model"] = SourceOverride
	}
	if overrides.SeedreamVisionModel != nil {
		cfg.SeedreamVisionModel = *overrides.SeedreamVisionModel
		meta.sources["seedream_vision_model"] = SourceOverride
	}
	if overrides.SeedreamVideoModel != nil {
		cfg.SeedreamVideoModel = *overrides.SeedreamVideoModel
		meta.sources["seedream_video_model"] = SourceOverride
	}
	if overrides.Environment != nil {
		cfg.Environment = *overrides.Environment
		meta.sources["environment"] = SourceOverride
	}
	if overrides.Verbose != nil {
		cfg.Verbose = *overrides.Verbose
		meta.sources["verbose"] = SourceOverride
	}
	if overrides.DisableTUI != nil {
		cfg.DisableTUI = *overrides.DisableTUI
		meta.sources["disable_tui"] = SourceOverride
	}
	if overrides.FollowTranscript != nil {
		cfg.FollowTranscript = *overrides.FollowTranscript
		meta.sources["follow_transcript"] = SourceOverride
	}
	if overrides.FollowStream != nil {
		cfg.FollowStream = *overrides.FollowStream
		meta.sources["follow_stream"] = SourceOverride
	}
	if overrides.MaxIterations != nil {
		cfg.MaxIterations = *overrides.MaxIterations
		meta.sources["max_iterations"] = SourceOverride
	}
	if overrides.MaxTokens != nil {
		cfg.MaxTokens = *overrides.MaxTokens
		meta.sources["max_tokens"] = SourceOverride
	}
	if overrides.ToolMaxConcurrent != nil {
		cfg.ToolMaxConcurrent = *overrides.ToolMaxConcurrent
		meta.sources["tool_max_concurrent"] = SourceOverride
	}
	if overrides.LLMCacheSize != nil {
		cfg.LLMCacheSize = *overrides.LLMCacheSize
		meta.sources["llm_cache_size"] = SourceOverride
	}
	if overrides.LLMCacheTTLSeconds != nil {
		cfg.LLMCacheTTLSeconds = *overrides.LLMCacheTTLSeconds
		meta.sources["llm_cache_ttl_seconds"] = SourceOverride
	}
	if overrides.UserRateLimitRPS != nil {
		cfg.UserRateLimitRPS = *overrides.UserRateLimitRPS
		meta.sources["user_rate_limit_rps"] = SourceOverride
	}
	if overrides.UserRateLimitBurst != nil {
		cfg.UserRateLimitBurst = *overrides.UserRateLimitBurst
		meta.sources["user_rate_limit_burst"] = SourceOverride
	}
	if overrides.Temperature != nil {
		cfg.Temperature = *overrides.Temperature
		cfg.TemperatureProvided = true
		meta.sources["temperature"] = SourceOverride
	}
	if overrides.TopP != nil {
		cfg.TopP = *overrides.TopP
		meta.sources["top_p"] = SourceOverride
	}
	if overrides.StopSequences != nil {
		cfg.StopSequences = append([]string(nil), *overrides.StopSequences...)
		meta.sources["stop_sequences"] = SourceOverride
	}
	if overrides.SessionDir != nil {
		cfg.SessionDir = *overrides.SessionDir
		meta.sources["session_dir"] = SourceOverride
	}
	if overrides.CostDir != nil {
		cfg.CostDir = *overrides.CostDir
		meta.sources["cost_dir"] = SourceOverride
	}
	if overrides.AgentPreset != nil {
		cfg.AgentPreset = *overrides.AgentPreset
		meta.sources["agent_preset"] = SourceOverride
	}
	if overrides.ToolPreset != nil {
		cfg.ToolPreset = *overrides.ToolPreset
		meta.sources["tool_preset"] = SourceOverride
	}
}
