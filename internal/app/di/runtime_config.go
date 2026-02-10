package di

import (
	runtimeconfig "alex/internal/shared/config"
	"time"

	"alex/internal/app/toolregistry"
	"alex/internal/shared/agent/presets"
)

// ConfigFromRuntimeConfig maps the shared RuntimeConfig into the dependency
// injection container configuration.
func ConfigFromRuntimeConfig(runtime runtimeconfig.RuntimeConfig) Config {
	return Config{
		LLMProvider:                runtime.LLMProvider,
		LLMModel:                   runtime.LLMModel,
		LLMSmallProvider:           runtime.LLMSmallProvider,
		LLMSmallModel:              runtime.LLMSmallModel,
		LLMVisionModel:             runtime.LLMVisionModel,
		APIKey:                     runtime.APIKey,
		ArkAPIKey:                  runtime.ArkAPIKey,
		BaseURL:                    runtime.BaseURL,
		SandboxBaseURL:             runtime.SandboxBaseURL,
		ACPExecutorAddr:            runtime.ACPExecutorAddr,
		ACPExecutorCWD:             runtime.ACPExecutorCWD,
		ACPExecutorMode:            runtime.ACPExecutorMode,
		ACPExecutorAutoApprove:     runtime.ACPExecutorAutoApprove,
		ACPExecutorMaxCLICalls:     runtime.ACPExecutorMaxCLICalls,
		ACPExecutorMaxDuration:     runtime.ACPExecutorMaxDuration,
		ACPExecutorRequireManifest: runtime.ACPExecutorRequireManifest,
		TavilyAPIKey:               runtime.TavilyAPIKey,
		MoltbookAPIKey:             runtime.MoltbookAPIKey,
		MoltbookBaseURL:            runtime.MoltbookBaseURL,
		SeedreamTextEndpointID:     runtime.SeedreamTextEndpointID,
		SeedreamImageEndpointID:    runtime.SeedreamImageEndpointID,
		SeedreamTextModel:          runtime.SeedreamTextModel,
		SeedreamImageModel:         runtime.SeedreamImageModel,
		SeedreamVisionModel:        runtime.SeedreamVisionModel,
		SeedreamVideoModel:         runtime.SeedreamVideoModel,
		MaxTokens:                  runtime.MaxTokens,
		MaxIterations:              runtime.MaxIterations,
		ToolMaxConcurrent:          runtime.ToolMaxConcurrent,
		LLMCacheSize:               runtime.LLMCacheSize,
		LLMCacheTTL:                time.Duration(runtime.LLMCacheTTLSeconds) * time.Second,
		UserRateLimitRPS:           runtime.UserRateLimitRPS,
		UserRateLimitBurst:         runtime.UserRateLimitBurst,
		Temperature:                runtime.Temperature,
		TemperatureProvided:        runtime.TemperatureProvided,
		TopP:                       runtime.TopP,
		StopSequences:              append([]string(nil), runtime.StopSequences...),
		AgentPreset:                runtime.AgentPreset,
		ToolPreset:                 runtime.ToolPreset,
		ToolMode:                   string(presets.ToolModeCLI),
		Toolset:                    toolregistry.NormalizeToolset(runtime.Toolset),
		Profile:                    runtime.Profile,
		BrowserConfig: toolregistry.BrowserConfig{
			Connector:        runtime.Browser.Connector,
			CDPURL:           runtime.Browser.CDPURL,
			ChromePath:       runtime.Browser.ChromePath,
			Headless:         runtime.Browser.Headless,
			UserDataDir:      runtime.Browser.UserDataDir,
			Timeout:          time.Duration(runtime.Browser.TimeoutSeconds) * time.Second,
			BridgeListenAddr: runtime.Browser.BridgeListen,
			BridgeToken:      runtime.Browser.BridgeToken,
		},
		Environment:       runtime.Environment,
		Verbose:           runtime.Verbose,
		DisableTUI:        runtime.DisableTUI,
		FollowTranscript:  runtime.FollowTranscript,
		FollowStream:      runtime.FollowStream,
		SessionDir:        runtime.SessionDir,
		CostDir:           runtime.CostDir,
		SessionStaleAfter: time.Duration(runtime.SessionStaleAfterSeconds) * time.Second,
		HTTPLimits:        runtime.HTTPLimits,
		ToolPolicy:        runtime.ToolPolicy,
		Proactive:         runtime.Proactive,
		ExternalAgents:    runtime.ExternalAgents,
	}
}
