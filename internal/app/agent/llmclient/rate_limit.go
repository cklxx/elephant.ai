package llmclient

import (
	"errors"
	"net/http"
	"strings"

	alexerrors "alex/internal/shared/errors"
)

// IsRateLimitError reports whether err indicates provider-side rate limiting.
func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	var transient *alexerrors.TransientError
	if errors.As(err, &transient) && transient.StatusCode == http.StatusTooManyRequests {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "usage_limit_reached") ||
		strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "too many requests") ||
		strings.Contains(lower, "429")
}
