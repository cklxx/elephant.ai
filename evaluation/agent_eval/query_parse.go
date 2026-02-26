package agent_eval

import (
	"strings"
	"time"
)

// ParseOptionalRFC3339 parses an RFC3339 timestamp and reports whether one was provided.
func ParseOptionalRFC3339(raw string) (time.Time, bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, false, nil
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false, err
	}
	return parsed, true, nil
}
