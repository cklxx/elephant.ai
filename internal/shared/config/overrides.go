package config

func applyOverrides(cfg *RuntimeConfig, meta *Metadata, overrides Overrides) {
	applyString := func(source *string, target *string, key string) {
		if source == nil {
			return
		}
		*target = *source
		meta.sources[key] = SourceOverride
	}
	applyBool := func(source *bool, target *bool, key string) {
		if source == nil {
			return
		}
		*target = *source
		meta.sources[key] = SourceOverride
	}
	applyInt := func(source *int, target *int, key string) {
		if source == nil {
			return
		}
		*target = *source
		meta.sources[key] = SourceOverride
	}
	applyFloat := func(source *float64, target *float64, key string) {
		if source == nil {
			return
		}
		*target = *source
		meta.sources[key] = SourceOverride
	}

	for _, field := range []struct {
		source *string
		target *string
		key    string
	}{
		{overrides.LLMProvider, &cfg.LLMProvider, "llm_provider"},
		{overrides.LLMModel, &cfg.LLMModel, "llm_model"},
		{overrides.LLMVisionModel, &cfg.LLMVisionModel, "llm_vision_model"},
		{overrides.APIKey, &cfg.APIKey, "api_key"},
		{overrides.ArkAPIKey, &cfg.ArkAPIKey, "ark_api_key"},
		{overrides.BaseURL, &cfg.BaseURL, "base_url"},
		{overrides.ACPExecutorAddr, &cfg.ACPExecutorAddr, "acp_executor_addr"},
		{overrides.ACPExecutorCWD, &cfg.ACPExecutorCWD, "acp_executor_cwd"},
		{overrides.ACPExecutorMode, &cfg.ACPExecutorMode, "acp_executor_mode"},
		{overrides.TavilyAPIKey, &cfg.TavilyAPIKey, "tavily_api_key"},
		{overrides.MoltbookAPIKey, &cfg.MoltbookAPIKey, "moltbook_api_key"},
		{overrides.MoltbookBaseURL, &cfg.MoltbookBaseURL, "moltbook_base_url"},
		{overrides.SeedreamTextEndpointID, &cfg.SeedreamTextEndpointID, "seedream_text_endpoint_id"},
		{overrides.SeedreamImageEndpointID, &cfg.SeedreamImageEndpointID, "seedream_image_endpoint_id"},
		{overrides.SeedreamTextModel, &cfg.SeedreamTextModel, "seedream_text_model"},
		{overrides.SeedreamImageModel, &cfg.SeedreamImageModel, "seedream_image_model"},
		{overrides.SeedreamVisionModel, &cfg.SeedreamVisionModel, "seedream_vision_model"},
		{overrides.SeedreamVideoModel, &cfg.SeedreamVideoModel, "seedream_video_model"},
		{overrides.Profile, &cfg.Profile, "profile"},
		{overrides.Environment, &cfg.Environment, "environment"},
		{overrides.SessionDir, &cfg.SessionDir, "session_dir"},
		{overrides.CostDir, &cfg.CostDir, "cost_dir"},
		{overrides.AgentPreset, &cfg.AgentPreset, "agent_preset"},
		{overrides.ToolPreset, &cfg.ToolPreset, "tool_preset"},
		{overrides.Toolset, &cfg.Toolset, "toolset"},
	} {
		applyString(field.source, field.target, field.key)
	}

	for _, field := range []struct {
		source *bool
		target *bool
		key    string
	}{
		{overrides.ACPExecutorAutoApprove, &cfg.ACPExecutorAutoApprove, "acp_executor_auto_approve"},
		{overrides.ACPExecutorRequireManifest, &cfg.ACPExecutorRequireManifest, "acp_executor_require_manifest"},
		{overrides.Verbose, &cfg.Verbose, "verbose"},
		{overrides.DisableTUI, &cfg.DisableTUI, "disable_tui"},
		{overrides.FollowTranscript, &cfg.FollowTranscript, "follow_transcript"},
		{overrides.FollowStream, &cfg.FollowStream, "follow_stream"},
	} {
		applyBool(field.source, field.target, field.key)
	}

	for _, field := range []struct {
		source *int
		target *int
		key    string
	}{
		{overrides.ACPExecutorMaxCLICalls, &cfg.ACPExecutorMaxCLICalls, "acp_executor_max_cli_calls"},
		{overrides.ACPExecutorMaxDuration, &cfg.ACPExecutorMaxDuration, "acp_executor_max_duration_seconds"},
		{overrides.MaxIterations, &cfg.MaxIterations, "max_iterations"},
		{overrides.MaxTokens, &cfg.MaxTokens, "max_tokens"},
		{overrides.ToolMaxConcurrent, &cfg.ToolMaxConcurrent, "tool_max_concurrent"},
		{overrides.LLMCacheSize, &cfg.LLMCacheSize, "llm_cache_size"},
		{overrides.LLMCacheTTLSeconds, &cfg.LLMCacheTTLSeconds, "llm_cache_ttl_seconds"},
		{overrides.UserRateLimitBurst, &cfg.UserRateLimitBurst, "user_rate_limit_burst"},
		{overrides.KimiRateLimitBurst, &cfg.KimiRateLimitBurst, "kimi_rate_limit_burst"},
		{overrides.SessionStaleAfterSeconds, &cfg.SessionStaleAfterSeconds, "session_stale_after_seconds"},
	} {
		applyInt(field.source, field.target, field.key)
	}

	for _, field := range []struct {
		source *float64
		target *float64
		key    string
	}{
		{overrides.UserRateLimitRPS, &cfg.UserRateLimitRPS, "user_rate_limit_rps"},
		{overrides.KimiRateLimitRPS, &cfg.KimiRateLimitRPS, "kimi_rate_limit_rps"},
		{overrides.TopP, &cfg.TopP, "top_p"},
	} {
		applyFloat(field.source, field.target, field.key)
	}

	if overrides.Temperature != nil {
		cfg.Temperature = *overrides.Temperature
		cfg.TemperatureProvided = true
		meta.sources["temperature"] = SourceOverride
	}
	if overrides.StopSequences != nil {
		cfg.StopSequences = append([]string(nil), *overrides.StopSequences...)
		meta.sources["stop_sequences"] = SourceOverride
	}

	if overrides.Browser != nil {
		for _, field := range []struct {
			source *string
			target *string
			key    string
		}{
			{overrides.Browser.Connector, &cfg.Browser.Connector, "browser.connector"},
			{overrides.Browser.CDPURL, &cfg.Browser.CDPURL, "browser.cdp_url"},
			{overrides.Browser.ChromePath, &cfg.Browser.ChromePath, "browser.chrome_path"},
			{overrides.Browser.UserDataDir, &cfg.Browser.UserDataDir, "browser.user_data_dir"},
			{overrides.Browser.BridgeListen, &cfg.Browser.BridgeListen, "browser.bridge_listen_addr"},
			{overrides.Browser.BridgeToken, &cfg.Browser.BridgeToken, "browser.bridge_token"},
		} {
			applyString(field.source, field.target, field.key)
		}
		applyBool(overrides.Browser.Headless, &cfg.Browser.Headless, "browser.headless")
		applyInt(overrides.Browser.TimeoutSeconds, &cfg.Browser.TimeoutSeconds, "browser.timeout_seconds")
	}
	if overrides.HTTPLimits != nil {
		applyHTTPLimitsOverrides(cfg, meta, overrides.HTTPLimits)
	}
	if overrides.Proactive != nil {
		cfg.Proactive = *overrides.Proactive
		meta.sources["proactive"] = SourceOverride
	}
}
