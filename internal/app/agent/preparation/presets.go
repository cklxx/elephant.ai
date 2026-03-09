package preparation

import (
	"context"
	"strings"

	"alex/internal/domain/agent/presets"
)

// SetEnvironmentSummary updates the environment summary used when preparing prompts.
// It wraps the static string as a provider for consistency with the lazy path.
func (s *ExecutionPreparationService) SetEnvironmentSummary(summary string) {
	s.config.EnvironmentSummary = summary
	s.config.EnvironmentSummaryProvider = func() string { return summary }
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
