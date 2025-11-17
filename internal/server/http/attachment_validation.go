package http

import (
	"fmt"
	"regexp"
	"strings"
)

const maxAttachmentNameLength = 80

var disallowedAttachmentRunes = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// normalizeAttachmentName converts arbitrary filenames into placeholder-safe names
// so downstream components can rely on deterministic keys.
func normalizeAttachmentName(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("attachment name is required")
	}
	sanitized := disallowedAttachmentRunes.ReplaceAllString(trimmed, "_")
	sanitized = strings.Trim(sanitized, "._-")
	sanitized = strings.TrimSpace(sanitized)
	if len(sanitized) == 0 {
		return "", fmt.Errorf("attachment name '%s' is invalid", raw)
	}
	if len(sanitized) > maxAttachmentNameLength {
		sanitized = sanitized[:maxAttachmentNameLength]
	}
	return sanitized, nil
}

func estimateBase64Size(data string) int64 {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return 0
	}
	length := len(trimmed)
	padding := 0
	if strings.HasSuffix(trimmed, "==") {
		padding = 2
	} else if strings.HasSuffix(trimmed, "=") {
		padding = 1
	}
	decoded := (length/4)*3 - padding
	if decoded < 0 {
		return 0
	}
	return int64(decoded)
}
