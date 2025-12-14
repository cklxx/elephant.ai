package di

import runtimeconfig "alex/internal/config"

// ConfigFromRuntimeConfig maps the shared RuntimeConfig into the dependency
// injection container configuration.
func ConfigFromRuntimeConfig(runtime runtimeconfig.RuntimeConfig) Config {
	return Config{
		LLMProvider:             runtime.LLMProvider,
		LLMModel:                runtime.LLMModel,
		LLMVisionModel:          runtime.LLMVisionModel,
		APIKey:                  runtime.APIKey,
		ArkAPIKey:               runtime.ArkAPIKey,
		BaseURL:                 runtime.BaseURL,
		TavilyAPIKey:            runtime.TavilyAPIKey,
		SeedreamTextEndpointID:  runtime.SeedreamTextEndpointID,
		SeedreamImageEndpointID: runtime.SeedreamImageEndpointID,
		SeedreamTextModel:       runtime.SeedreamTextModel,
		SeedreamImageModel:      runtime.SeedreamImageModel,
		SeedreamVisionModel:     runtime.SeedreamVisionModel,
		SeedreamVideoModel:      runtime.SeedreamVideoModel,
		MaxTokens:               runtime.MaxTokens,
		MaxIterations:           runtime.MaxIterations,
		UserRateLimitRPS:        runtime.UserRateLimitRPS,
		UserRateLimitBurst:      runtime.UserRateLimitBurst,
		Temperature:             runtime.Temperature,
		TemperatureSet:          runtime.TemperatureProvided,
		TopP:                    runtime.TopP,
		StopSequences:           append([]string(nil), runtime.StopSequences...),
		AgentPreset:             runtime.AgentPreset,
		ToolPreset:              runtime.ToolPreset,
		Environment:             runtime.Environment,
		Verbose:                 runtime.Verbose,
		DisableTUI:              runtime.DisableTUI,
		FollowTranscript:        runtime.FollowTranscript,
		FollowStream:            runtime.FollowStream,
		SessionDir:              runtime.SessionDir,
		CostDir:                 runtime.CostDir,
	}
}
