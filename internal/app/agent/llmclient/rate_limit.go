package llmclient

import "strings"

// IsRateLimitError reports whether err indicates provider-side rate limiting.
func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "usage_limit_reached") ||
		strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "too many requests") ||
		strings.Contains(lower, "429")
}
