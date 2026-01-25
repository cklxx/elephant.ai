package preparation

import (
	"context"
	"strings"

	appcontext "alex/internal/agent/app/context"
	storage "alex/internal/agent/ports/storage"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/agent/presets"
)

func (s *ExecutionPreparationService) loadSession(ctx context.Context, id string) (*storage.Session, error) {
	if id == "" {
		session, err := s.sessionStore.Create(ctx)
		if err != nil {
			s.logger.Error("Failed to create session: %v", err)
		}
		return session, err
	}

	session, err := s.sessionStore.Get(ctx, id)
	if err != nil {
		s.logger.Error("Failed to load session: %v", err)
	}
	return session, err
}

func (s *ExecutionPreparationService) selectToolRegistry(ctx context.Context, toolMode presets.ToolMode, resolvedToolPreset string) tools.ToolRegistry {
	// Handle subagent context filtering first
	registry := s.toolRegistry
	if toolMode == "" {
		toolMode = presets.ToolModeCLI
	}
	configPreset := strings.TrimSpace(resolvedToolPreset)
	if configPreset == "" {
		configPreset = strings.TrimSpace(s.config.ToolPreset)
	}
	if appcontext.IsSubagentContext(ctx) {
		registry = s.getRegistryWithoutSubagent()
		s.logger.Debug("Using filtered registry (subagent excluded) for nested call")

		// Apply preset configured for subagents (context overrides allowed)
		return s.presetResolver.ResolveToolRegistry(ctx, registry, toolMode, configPreset)
	}

	return s.presetResolver.ResolveToolRegistry(ctx, registry, toolMode, configPreset)
}

func (s *ExecutionPreparationService) getRegistryWithoutSubagent() tools.ToolRegistry {
	type registryWithFilter interface {
		WithoutSubagent() tools.ToolRegistry
	}

	if filtered, ok := s.toolRegistry.(registryWithFilter); ok {
		return filtered.WithoutSubagent()
	}

	return s.toolRegistry
}
