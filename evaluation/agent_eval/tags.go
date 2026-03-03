package agent_eval

import (
	"strings"

	"alex/internal/shared/utils"
)

// ParseCSVTags trims and splits comma-separated tags.
func ParseCSVTags(raw string) []string {
	if utils.IsBlank(raw) {
		return nil
	}

	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		tags = append(tags, trimmed)
	}
	return tags
}
