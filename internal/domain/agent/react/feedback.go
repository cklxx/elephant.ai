package react

import (
	"fmt"
	"strconv"
	"strings"

	"alex/internal/domain/agent"
)

func deriveFeedbackValue(result ToolResult) float64 {
	if reward, ok := extractRewardValue(result.Metadata); ok {
		return reward
	}
	if result.Error != nil {
		return -1
	}
	return 1
}

func buildFeedbackMessage(result ToolResult) string {
	label := strings.TrimSpace(result.CallID)
	if label == "" {
		label = "tool"
	}
	status := "completed"
	if result.Error != nil {
		status = "errored"
	}
	if preview := summarizeForWorld(result.Content, domain.ToolResultPreviewRunes/3); preview != "" {
		return fmt.Sprintf("%s %s: %s", label, status, preview)
	}
	return fmt.Sprintf("%s %s", label, status)
}

func extractRewardValue(metadata map[string]any) (float64, bool) {
	if len(metadata) == 0 {
		return 0, false
	}
	for _, key := range []string{"reward", "score", "value"} {
		raw, ok := metadata[key]
		if !ok {
			continue
		}
		switch v := raw.(type) {
		case float64:
			return v, true
		case float32:
			return float64(v), true
		case int:
			return float64(v), true
		case int64:
			return float64(v), true
		case int32:
			return float64(v), true
		case uint64:
			return float64(v), true
		case uint32:
			return float64(v), true
		case string:
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}
