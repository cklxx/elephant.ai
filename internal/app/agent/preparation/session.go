package preparation

import (
	"context"
	"errors"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	storage "alex/internal/domain/agent/ports/storage"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/domain/agent/presets"
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
	if err == nil {
		return session, nil
	}
	if !errors.Is(err, storage.ErrSessionNotFound) {
		s.logger.Error("Failed to load session: %v", err)
		return nil, err
	}

	now := time.Now()
	if s.clock != nil {
		now = s.clock.Now()
	}
	session = storage.NewSession(id, now)
	if saveErr := s.sessionStore.Save(ctx, session); saveErr != nil {
		s.logger.Error("Failed to create missing session %s: %v", id, saveErr)
		return nil, saveErr
	}
	return session, nil
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
		registry = s.getRegistryWithoutOrchestration()
		s.logger.Debug("Using filtered registry (orchestration excluded) for nested call")
	}
	registry = s.applyToolPolicy(ctx, registry)

	// Apply preset configured for subagents (context overrides allowed)
	return s.presetResolver.ResolveToolRegistry(ctx, registry, toolMode, configPreset)
}

func (s *ExecutionPreparationService) getRegistryWithoutOrchestration() tools.ToolRegistry {
	type registryWithFilter interface {
		WithoutOrchestration() tools.ToolRegistry
	}

	if filtered, ok := s.toolRegistry.(registryWithFilter); ok {
		return filtered.WithoutOrchestration()
	}

	return s.toolRegistry
}

func (s *ExecutionPreparationService) applyToolPolicy(ctx context.Context, registry tools.ToolRegistry) tools.ToolRegistry {
	if registry == nil || s.toolPolicy == nil {
		return registry
	}
	type policyWrapper interface {
		WithPolicy(policy tools.ToolPolicy, channel string) tools.ToolRegistry
	}
	if wrapper, ok := registry.(policyWrapper); ok {
		channel := strings.TrimSpace(appcontext.ChannelFromContext(ctx))
		return wrapper.WithPolicy(s.toolPolicy, channel)
	}
	return registry
}
