package app

import (
	"context"
	"strings"

	"alex/internal/agent/ports"
	"alex/internal/agent/presets"
)

func (s *ExecutionPreparationService) loadSession(ctx context.Context, id string) (*ports.Session, error) {
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

func (s *ExecutionPreparationService) selectToolRegistry(ctx context.Context, toolMode presets.ToolMode, resolvedToolPreset string) ports.ToolRegistry {
	// Handle subagent context filtering first
	registry := s.toolRegistry
	if toolMode == "" {
		toolMode = presets.ToolModeCLI
	}
	configPreset := strings.TrimSpace(resolvedToolPreset)
	if configPreset == "" {
		configPreset = strings.TrimSpace(s.config.ToolPreset)
	}
	if isSubagentContext(ctx) {
		registry = s.getRegistryWithoutSubagent()
		s.logger.Debug("Using filtered registry (subagent excluded) for nested call")

		// Apply preset configured for subagents (context overrides allowed)
		return s.presetResolver.ResolveToolRegistry(ctx, registry, toolMode, configPreset)
	}

	return s.presetResolver.ResolveToolRegistry(ctx, registry, toolMode, configPreset)
}

func (s *ExecutionPreparationService) getRegistryWithoutSubagent() ports.ToolRegistry {
	type registryWithFilter interface {
		WithoutSubagent() ports.ToolRegistry
	}

	if filtered, ok := s.toolRegistry.(registryWithFilter); ok {
		return filtered.WithoutSubagent()
	}

	return s.toolRegistry
}
