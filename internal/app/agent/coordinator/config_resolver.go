package coordinator

import (
	"context"
	"fmt"
	"strings"
	"time"

	appconfig "alex/internal/app/agent/config"
	"alex/internal/app/agent/preparation"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/domain/agent/presets"
	runtimeconfig "alex/internal/shared/config"
)

func (c *AgentCoordinator) SetRuntimeConfigResolver(resolver RuntimeConfigResolver) {
	if c == nil {
		return
	}
	c.runtimeResolver = resolver
}

func (c *AgentCoordinator) effectiveConfig(ctx context.Context) appconfig.Config {
	cfg := c.config
	resolver := c.runtimeResolver
	if resolver == nil {
		cfg.LLMProfile = cfg.DefaultLLMProfile()
		return cfg
	}

	runtimeCfg, _, err := resolver(ctx)
	if err != nil {
		logger := c.loggerFor(ctx)
		if logger != nil {
			logger.Warn("Runtime config resolve failed: %v", err)
		}
		cfg.LLMProfile = cfg.DefaultLLMProfile()
		return cfg
	}

	profile, err := runtimeconfig.ResolveLLMProfile(runtimeCfg)
	if err != nil {
		logger := c.loggerFor(ctx)
		if logger != nil {
			logger.Warn("Runtime LLM profile resolve failed: %v", err)
		}
		cfg.LLMProfile = cfg.DefaultLLMProfile()
		return cfg
	}

	cfg.LLMProfile = profile
	cfg.LLMProvider = profile.Provider
	cfg.LLMModel = profile.Model
	cfg.APIKey = profile.APIKey
	cfg.BaseURL = profile.BaseURL
	cfg.LLMSmallProvider = runtimeCfg.LLMSmallProvider
	if strings.TrimSpace(cfg.LLMSmallProvider) == "" {
		cfg.LLMSmallProvider = profile.Provider
	}
	cfg.LLMSmallModel = runtimeCfg.LLMSmallModel
	cfg.LLMVisionModel = runtimeCfg.LLMVisionModel
	cfg.MaxTokens = runtimeCfg.MaxTokens
	cfg.MaxIterations = runtimeCfg.MaxIterations
	cfg.ToolMaxConcurrent = runtimeCfg.ToolMaxConcurrent
	cfg.Temperature = runtimeCfg.Temperature
	cfg.TemperatureProvided = runtimeCfg.TemperatureProvided
	cfg.TopP = runtimeCfg.TopP
	cfg.StopSequences = append([]string(nil), runtimeCfg.StopSequences...)
	if strings.TrimSpace(runtimeCfg.AgentPreset) != "" {
		cfg.AgentPreset = runtimeCfg.AgentPreset
	}
	if strings.TrimSpace(runtimeCfg.ToolPreset) != "" {
		cfg.ToolPreset = runtimeCfg.ToolPreset
	}
	cfg.SessionStaleAfter = time.Duration(runtimeCfg.SessionStaleAfterSeconds) * time.Second
	cfg.Proactive = runtimeCfg.Proactive
	cfg.ToolPolicy = runtimeCfg.ToolPolicy

	return cfg
}

// GetConfig returns the coordinator configuration
func (c *AgentCoordinator) GetConfig() agent.AgentConfig {
	profile := c.config.DefaultLLMProfile()
	return agent.AgentConfig{
		LLMProvider:   profile.Provider,
		LLMModel:      profile.Model,
		MaxTokens:     c.config.MaxTokens,
		MaxIterations: c.config.MaxIterations,
		Temperature:   c.config.Temperature,
		TopP:          c.config.TopP,
		StopSequences: append([]string(nil), c.config.StopSequences...),
		AgentPreset:   c.config.AgentPreset,
		ToolPreset:    c.config.ToolPreset,
		ToolMode:      c.config.ToolMode,
	}
}

// GetSystemPrompt returns the system prompt
func (c *AgentCoordinator) GetSystemPrompt() string {
	if c.contextMgr == nil {
		return preparation.DefaultSystemPrompt
	}
	personaKey := c.config.AgentPreset
	toolMode := c.config.ToolMode
	if strings.TrimSpace(toolMode) == "" {
		toolMode = string(presets.ToolModeCLI)
	}
	toolPreset := c.config.ToolPreset
	if c.prepService != nil {
		if resolved := c.prepService.ResolveAgentPreset(context.Background(), personaKey); resolved != "" {
			personaKey = resolved
		}
		if resolved := c.prepService.ResolveToolPreset(context.Background(), toolPreset); resolved != "" {
			toolPreset = resolved
		}
	} else if toolPreset == "" {
		toolPreset = string(presets.ToolPresetFull)
	}
	session := &storage.Session{ID: "", Messages: nil}
	okrContext := ""
	if c.okrContextProvider != nil {
		okrContext = c.okrContextProvider()
	}
	kernelContext := ""
	if c.kernelContextProvider != nil {
		kernelContext = c.kernelContextProvider()
	}
	window, err := c.contextMgr.BuildWindow(context.Background(), session, agent.ContextWindowConfig{
		TokenLimit:             c.config.MaxTokens,
		PersonaKey:             personaKey,
		ToolMode:               toolMode,
		ToolPreset:             toolPreset,
		EnvironmentSummary:     c.config.EnvironmentSummary,
		PromptMode:             c.config.Proactive.Prompt.Mode,
		PromptTimezone:         c.config.Proactive.Prompt.Timezone,
		BootstrapFiles:         append([]string(nil), c.config.Proactive.Prompt.BootstrapFiles...),
		BootstrapMaxChars:      c.config.Proactive.Prompt.BootstrapMaxChars,
		ReplyTagsEnabled:       c.config.Proactive.Prompt.ReplyTagsEnabled,
		OKRContext:             okrContext,
		KernelAlignmentContext: kernelContext,
	})
	if err != nil {
		if c.logger != nil {
			c.logger.Warn("Failed to build preview context window: %v", err)
		}
		return preparation.DefaultSystemPrompt
	}
	if prompt := strings.TrimSpace(window.SystemPrompt); prompt != "" {
		return prompt
	}
	return preparation.DefaultSystemPrompt
}

// PreviewContextWindow constructs the current context window for a session
// without mutating session state. This is intended for diagnostics in
// development flows.
func (c *AgentCoordinator) PreviewContextWindow(ctx context.Context, sessionID string) (agent.ContextWindowPreview, error) {
	preview := agent.ContextWindowPreview{}

	if c.contextMgr == nil {
		return preview, fmt.Errorf("context manager not configured")
	}

	cfg := c.effectiveConfig(ctx)

	session, err := c.GetSession(ctx, sessionID)
	if err != nil {
		return preview, err
	}

	toolMode := presets.ToolMode(strings.TrimSpace(cfg.ToolMode))
	if toolMode == "" {
		toolMode = presets.ToolModeCLI
	}
	toolPreset := strings.TrimSpace(cfg.ToolPreset)
	if c.prepService != nil {
		if resolved := c.prepService.ResolveToolPreset(ctx, toolPreset); resolved != "" {
			toolPreset = resolved
		}
		if resolved := c.prepService.ResolveAgentPreset(ctx, cfg.AgentPreset); resolved != "" {
			preview.PersonaKey = resolved
		}
	}
	if preview.PersonaKey == "" {
		preview.PersonaKey = cfg.AgentPreset
	}
	if toolMode == presets.ToolModeCLI && toolPreset == "" {
		toolPreset = string(presets.ToolPresetFull)
	}
	window, err := c.contextMgr.BuildWindow(ctx, session, agent.ContextWindowConfig{
		TokenLimit:         cfg.MaxTokens,
		PersonaKey:         preview.PersonaKey,
		ToolMode:           string(toolMode),
		ToolPreset:         toolPreset,
		EnvironmentSummary: cfg.EnvironmentSummary,
		PromptMode:         cfg.Proactive.Prompt.Mode,
		PromptTimezone:     cfg.Proactive.Prompt.Timezone,
		BootstrapFiles:     append([]string(nil), cfg.Proactive.Prompt.BootstrapFiles...),
		BootstrapMaxChars:  cfg.Proactive.Prompt.BootstrapMaxChars,
		ReplyTagsEnabled:   cfg.Proactive.Prompt.ReplyTagsEnabled,
	})
	if err != nil {
		return preview, fmt.Errorf("build context window: %w", err)
	}

	preview.Window = window
	preview.TokenEstimate = c.contextMgr.EstimateTokens(window.Messages)
	preview.TokenLimit = cfg.MaxTokens
	preview.ToolMode = string(toolMode)
	preview.ToolPreset = toolPreset

	return preview, nil
}
