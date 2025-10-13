package app

import (
	"context"
	"os"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/agent/presets"
	"alex/internal/prompts"
)

// PresetResolver handles preset resolution for both agent and tool presets.
// It resolves presets from context (highest priority), config (fallback), or defaults.
type PresetResolver struct {
	promptLoader *prompts.Loader
	logger       ports.Logger
	clock        ports.Clock
	eventEmitter ports.EventListener
}

// PresetResolverDeps enumerates dependencies for PresetResolver
type PresetResolverDeps struct {
	PromptLoader *prompts.Loader
	Logger       ports.Logger
	Clock        ports.Clock
	EventEmitter ports.EventListener
}

// NewPresetResolver creates a new preset resolver instance.
func NewPresetResolver(promptLoader *prompts.Loader, logger ports.Logger) *PresetResolver {
	return NewPresetResolverWithDeps(PresetResolverDeps{
		PromptLoader: promptLoader,
		Logger:       logger,
	})
}

// NewPresetResolverWithDeps creates a new preset resolver with full dependencies
func NewPresetResolverWithDeps(deps PresetResolverDeps) *PresetResolver {
	logger := deps.Logger
	if logger == nil {
		logger = ports.NoopLogger{}
	}
	promptLoader := deps.PromptLoader
	if promptLoader == nil {
		promptLoader = prompts.New()
	}
	clock := deps.Clock
	if clock == nil {
		clock = ports.SystemClock{}
	}
	eventEmitter := deps.EventEmitter
	if eventEmitter == nil {
		eventEmitter = ports.NoopEventListener{}
	}

	return &PresetResolver{
		promptLoader: promptLoader,
		logger:       logger,
		clock:        clock,
		eventEmitter: eventEmitter,
	}
}

// ResolveSystemPrompt resolves the system prompt based on agent preset configuration.
// Priority: context preset > config preset > default prompt loader.
func (r *PresetResolver) ResolveSystemPrompt(
	ctx context.Context,
	task string,
	analysis *prompts.TaskAnalysisInfo,
	configPreset string,
) string {
	agentPreset, source := r.resolveAgentPreset(ctx, configPreset)

	// If we have a valid preset, use it
	if agentPreset != "" && presets.IsValidPreset(agentPreset) {
		presetConfig, err := presets.GetPromptConfig(presets.AgentPreset(agentPreset))
		if err != nil {
			r.logger.Warn("Failed to load preset prompt: %v, using default", err)
			return r.loadDefaultPrompt(task, analysis)
		}
		r.logger.Info("Using preset system prompt: %s (source=%s)", presetConfig.Name, source)
		return presetConfig.SystemPrompt
	}

	// No preset, use default prompt loader
	return r.loadDefaultPrompt(task, analysis)
}

// ResolveToolRegistry resolves the tool registry based on tool preset configuration.
// Priority: context preset > config preset > base registry.
func (r *PresetResolver) ResolveToolRegistry(
	ctx context.Context,
	baseRegistry ports.ToolRegistry,
	configPreset string,
) ports.ToolRegistry {
	toolPreset, source := r.resolveToolPreset(ctx, configPreset)

	// If we have a valid preset, apply filtering
	if toolPreset != "" && presets.IsValidToolPreset(toolPreset) {
		originalTools := baseRegistry.List()
		originalCount := len(originalTools)

		filteredRegistry, err := presets.NewFilteredToolRegistry(baseRegistry, presets.ToolPreset(toolPreset))
		if err != nil {
			r.logger.Warn("Failed to create filtered registry: %v, using default", err)
			return baseRegistry
		}

		filteredTools := filteredRegistry.List()
		filteredCount := len(filteredTools)

		toolConfig, _ := presets.GetToolConfig(presets.ToolPreset(toolPreset))
		retainedPercent := 0.0
		if originalCount > 0 {
			retainedPercent = float64(filteredCount) / float64(originalCount) * 100.0
		}
		r.logger.Info("Tool filtering applied: preset=%s, original=%d, filtered=%d (%.0f%% retained)",
			toolConfig.Name, originalCount, filteredCount, retainedPercent)
		r.logger.Info("Using tool preset: %s (source=%s, tool_count=%d/%d)", toolConfig.Name, source, filteredCount, originalCount)

		// Extract session ID from context if available
		sessionID := r.extractSessionID(ctx)

		// Emit tool filtering metrics event
		toolNames := make([]string, 0, filteredCount)
		for _, tool := range filteredTools {
			toolNames = append(toolNames, tool.Name)
		}
		filterEvent := domain.NewToolFilteringEvent(
			ports.LevelCore,
			sessionID,
			toolConfig.Name,
			originalCount,
			filteredCount,
			toolNames,
			r.clock.Now(),
		)
		r.eventEmitter.OnEvent(filterEvent)

		return filteredRegistry
	}

	// No preset, return base registry
	return baseRegistry
}

// extractSessionID attempts to extract session ID from context
func (r *PresetResolver) extractSessionID(ctx context.Context) string {
	// Try to extract from various context keys
	if sessionID, ok := ctx.Value("sessionID").(string); ok {
		return sessionID
	}
	if sessionID, ok := ctx.Value(sessionContextKey{}).(string); ok {
		return sessionID
	}
	return ""
}

type sessionContextKey struct{}

// resolveAgentPreset determines the agent preset to use.
// Returns the preset name and source ("context", "config", or "").
func (r *PresetResolver) resolveAgentPreset(ctx context.Context, configPreset string) (preset string, source string) {
	// Check context first (highest priority)
	if presetCfg, ok := ctx.Value(PresetContextKey{}).(PresetConfig); ok && presetCfg.AgentPreset != "" {
		r.logger.Debug("Using agent preset from context: %s", presetCfg.AgentPreset)
		return presetCfg.AgentPreset, "context"
	}

	// Fallback to config
	if configPreset != "" {
		return configPreset, "config"
	}

	// No preset configured
	return "", ""
}

// resolveToolPreset determines the tool preset to use.
// Returns the preset name and source ("context", "config", or "").
func (r *PresetResolver) resolveToolPreset(ctx context.Context, configPreset string) (preset string, source string) {
	// Check context first (highest priority)
	if presetCfg, ok := ctx.Value(PresetContextKey{}).(PresetConfig); ok && presetCfg.ToolPreset != "" {
		r.logger.Debug("Using tool preset from context: %s", presetCfg.ToolPreset)
		return presetCfg.ToolPreset, "context"
	}

	// Fallback to config
	if configPreset != "" {
		return configPreset, "config"
	}

	// No preset configured
	return "", ""
}

// loadDefaultPrompt loads the default system prompt using the prompt loader.
func (r *PresetResolver) loadDefaultPrompt(task string, analysis *prompts.TaskAnalysisInfo) string {
	workingDir, err := os.Getwd()
	if err != nil {
		workingDir = "."
	}

	prompt, err := r.promptLoader.GetSystemPrompt(workingDir, task, analysis)
	if err != nil {
		r.logger.Warn("Failed to load system prompt: %v, using fallback", err)
		return "You are ALEX, a helpful AI coding assistant. Use available tools to help solve the user's task."
	}

	r.logger.Debug("System prompt loaded: %d bytes", len(prompt))
	return prompt
}
