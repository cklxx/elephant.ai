package preparation

import (
	"context"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/domain/agent/presets"
	id "alex/internal/shared/utils/id"
)

// PresetResolver handles preset resolution for both agent and tool presets.
// It resolves presets from context (highest priority), config (fallback), or defaults.
type PresetResolver struct {
	logger       agent.Logger
	clock        agent.Clock
	eventEmitter agent.EventListener
}

// PresetResolverDeps enumerates dependencies for PresetResolver
type PresetResolverDeps struct {
	Logger       agent.Logger
	Clock        agent.Clock
	EventEmitter agent.EventListener
}

// NewPresetResolver creates a new preset resolver instance.
func NewPresetResolver(logger agent.Logger) *PresetResolver {
	return NewPresetResolverWithDeps(PresetResolverDeps{Logger: logger})
}

// NewPresetResolverWithDeps creates a new preset resolver with full dependencies
func NewPresetResolverWithDeps(deps PresetResolverDeps) *PresetResolver {
	logger := deps.Logger
	if logger == nil {
		logger = agent.NoopLogger{}
	}
	clock := deps.Clock
	if clock == nil {
		clock = agent.SystemClock{}
	}
	eventEmitter := deps.EventEmitter
	if eventEmitter == nil {
		eventEmitter = agent.NoopEventListener{}
	}

	return &PresetResolver{
		logger:       logger,
		clock:        clock,
		eventEmitter: eventEmitter,
	}
}

// ResolveToolRegistry resolves the tool registry based on tool preset configuration.
// Priority: context preset > config preset > base registry.
func (r *PresetResolver) ResolveToolRegistry(
	ctx context.Context,
	baseRegistry tools.ToolRegistry,
	mode presets.ToolMode,
	configPreset string,
) tools.ToolRegistry {
	toolMode := normalizeToolMode(mode)
	toolPreset, source := r.resolveToolPreset(ctx, toolMode, configPreset)

	originalTools := baseRegistry.List()
	originalCount := len(originalTools)

	config, err := presets.GetToolConfig(toolMode, presets.ToolPreset(toolPreset))
	if err != nil {
		r.logger.Warn("Failed to resolve tool config: %v, using defaults", err)
		config, _ = presets.GetToolConfig(toolMode, presets.ToolPresetFull)
		toolPreset = string(presets.ToolPresetFull)
		source = "default"
	}

	filteredRegistry, err := presets.NewFilteredToolRegistry(baseRegistry, toolMode, presets.ToolPreset(toolPreset))
	if err != nil {
		r.logger.Warn("Failed to create filtered registry: %v, using default", err)
		return baseRegistry
	}

	filteredTools := filteredRegistry.List()
	filteredCount := len(filteredTools)

	retainedPercent := 0.0
	if originalCount > 0 {
		retainedPercent = float64(filteredCount) / float64(originalCount) * 100.0
	}
	r.logger.Info(
		"Tool filtering applied: mode=%s, preset=%s, original=%d, filtered=%d (%.0f%% retained)",
		toolMode,
		config.Name,
		originalCount,
		filteredCount,
		retainedPercent,
	)
	r.logger.Info(
		"Using tool access: mode=%s, preset=%s (source=%s, tool_count=%d/%d)",
		toolMode,
		config.Name,
		source,
		filteredCount,
		originalCount,
	)

	// Extract identifiers from context for event emission
	ids := id.IDsFromContext(ctx)
	sessionID := r.extractSessionID(ctx)
	if sessionID == "" {
		sessionID = ids.SessionID
	} else {
		ids.SessionID = sessionID
	}

	// Emit tool filtering metrics event
	toolNames := make([]string, 0, filteredCount)
	for _, tool := range filteredTools {
		toolNames = append(toolNames, tool.Name)
	}
	filterEvent := domain.NewDiagnosticToolFilteringEvent(
		agent.LevelCore,
		sessionID,
		ids.RunID,
		ids.ParentRunID,
		config.Name,
		originalCount,
		filteredCount,
		toolNames,
		r.clock.Now(),
	)
	r.eventEmitter.OnEvent(filterEvent)

	return filteredRegistry
}

// extractSessionID attempts to extract session ID from context
func (r *PresetResolver) extractSessionID(ctx context.Context) string {
	// Use the shared SessionContextKey from utils/id package
	if sessionID, ok := ctx.Value(id.SessionContextKey{}).(string); ok {
		return sessionID
	}
	// Fallback to legacy string key for backward compatibility
	if sessionID, ok := ctx.Value("sessionID").(string); ok {
		return sessionID
	}
	return ""
}

// resolveAgentPreset determines the agent preset to use.
// Returns the preset name and source ("context", "config", or "").
func (r *PresetResolver) resolveAgentPreset(ctx context.Context, configPreset string) (preset string, source string) {
	// Check context first (highest priority)
	if presetCfg, ok := ctx.Value(appcontext.PresetContextKey{}).(appcontext.PresetConfig); ok && presetCfg.AgentPreset != "" {
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
func (r *PresetResolver) resolveToolPreset(ctx context.Context, mode presets.ToolMode, configPreset string) (preset string, source string) {
	if normalizeToolMode(mode) == presets.ToolModeWeb {
		if presetCfg, ok := ctx.Value(appcontext.PresetContextKey{}).(appcontext.PresetConfig); ok && presetCfg.ToolPreset != "" {
			r.logger.Debug("Using tool preset from context: %s", presetCfg.ToolPreset)
			return presetCfg.ToolPreset, "context"
		}
		if configPreset != "" {
			return configPreset, "config"
		}
		return "", "default"
	}

	// Check context first (highest priority)
	if presetCfg, ok := ctx.Value(appcontext.PresetContextKey{}).(appcontext.PresetConfig); ok && presetCfg.ToolPreset != "" {
		r.logger.Debug("Using tool preset from context: %s", presetCfg.ToolPreset)
		return presetCfg.ToolPreset, "context"
	}

	// Fallback to config
	if configPreset != "" {
		return configPreset, "config"
	}

	// No preset configured, default to full tool access
	return string(presets.ToolPresetFull), "default"
}

func normalizeToolMode(mode presets.ToolMode) presets.ToolMode {
	if mode == "" {
		return presets.ToolModeCLI
	}
	return mode
}
