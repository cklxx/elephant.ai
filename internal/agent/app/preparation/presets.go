package preparation

import (
	"context"
	"strings"

	"alex/internal/agent/presets"
)

// SetEnvironmentSummary updates the environment summary used when preparing prompts.
func (s *ExecutionPreparationService) SetEnvironmentSummary(summary string) {
	s.config.EnvironmentSummary = summary
}

func (s *ExecutionPreparationService) ResolveAgentPreset(ctx context.Context, preset string) string {
	if s.presetResolver == nil {
		return ""
	}
	resolved, _ := s.presetResolver.resolveAgentPreset(ctx, preset)
	return resolved
}

func (s *ExecutionPreparationService) ResolveToolPreset(ctx context.Context, preset string) string {
	if s.presetResolver == nil {
		return ""
	}
	toolMode := presets.ToolMode(strings.TrimSpace(s.config.ToolMode))
	if toolMode == "" {
		toolMode = presets.ToolModeCLI
	}
	resolved, _ := s.presetResolver.resolveToolPreset(ctx, toolMode, preset)
	return resolved
}
