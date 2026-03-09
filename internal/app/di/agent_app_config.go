package di

import (
	appconfig "alex/internal/app/agent/config"
	runtimeconfig "alex/internal/shared/config"
)

func (b *containerBuilder) buildAgentAppConfig() appconfig.Config {
	return appconfig.Config{
		LLMProvider:    b.config.LLMProvider,
		LLMModel:       b.config.LLMModel,
		LLMVisionModel: b.config.LLMVisionModel,
		APIKey:         b.config.APIKey,
		BaseURL:        b.config.BaseURL,
		LLMProfile: runtimeconfig.LLMProfile{
			Provider: b.config.LLMProvider,
			Model:    b.config.LLMModel,
			APIKey:   b.config.APIKey,
			BaseURL:  b.config.BaseURL,
		},
		MaxTokens:           b.config.MaxTokens,
		MaxIterations:       b.config.MaxIterations,
		ToolMaxConcurrent:   b.config.ToolMaxConcurrent,
		MaxBackgroundTasks:  b.config.ExternalAgents.MaxParallelAgents,
		Temperature:         b.config.Temperature,
		TemperatureProvided: b.config.TemperatureProvided,
		TopP:                b.config.TopP,
		StopSequences:       append([]string(nil), b.config.StopSequences...),
		AgentPreset:         b.config.AgentPreset,
		ToolPreset:          b.config.ToolPreset,
		ToolMode:            b.config.ToolMode,
		EnvironmentSummary:         b.config.EnvironmentSummary,
		EnvironmentSummaryProvider: b.config.EnvironmentSummaryProvider,
		SessionStaleAfter:   b.config.SessionStaleAfter,
		Proactive:           b.config.Proactive,
		ToolPolicy:          b.config.ToolPolicy,
	}
}
