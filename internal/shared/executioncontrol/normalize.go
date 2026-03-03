package executioncontrol

import (
	"alex/internal/shared/utils"
	"strings"
)

const (
	executionModeExecute = "execute"
	executionModePlan    = "plan"

	autonomyLevelControlled = "controlled"
	autonomyLevelSemi       = "semi"
	autonomyLevelFull       = "full"
)

// NormalizeExecutionMode returns either "plan" or "execute" (default).
func NormalizeExecutionMode(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), executionModePlan) {
		return executionModePlan
	}
	return executionModeExecute
}

// NormalizeAutonomyLevel returns one of "full", "semi", "controlled" (default).
func NormalizeAutonomyLevel(raw string) string {
	switch utils.TrimLower(raw) {
	case autonomyLevelFull:
		return autonomyLevelFull
	case autonomyLevelSemi:
		return autonomyLevelSemi
	default:
		return autonomyLevelControlled
	}
}
