package session

import (
	"regexp"
	"strings"
)

var sessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func sanitizeSessionID(sessionID string) string {
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" || !sessionIDPattern.MatchString(trimmed) {
		return "default"
	}
	return trimmed
}
