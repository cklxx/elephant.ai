package llm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	alexerrors "alex/internal/errors"
)

func wrapRequestError(err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return alexerrors.NewTransientError(err, "Request to LLM provider timed out. Please retry.")
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return alexerrors.NewTransientError(err, "Request to LLM provider timed out. Please retry.")
	}

	return alexerrors.NewTransientError(err, "Failed to reach LLM provider. Please retry shortly.")
}

func mapHTTPError(status int, body []byte, headers http.Header) error {
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(status)
	}

	baseErr := fmt.Errorf("status %d: %s", status, message)

	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		perr := alexerrors.NewPermanentError(baseErr, "Authentication failed. Please verify your API key.")
		perr.StatusCode = status
		return perr
	case status == http.StatusTooManyRequests:
		terr := alexerrors.NewTransientError(baseErr, "Rate limit reached. The system will retry automatically.")
		terr.StatusCode = status
		if retryAfter := parseRetryAfter(headers.Get("Retry-After")); retryAfter > 0 {
			terr.RetryAfter = retryAfter
		}
		return terr
	case status == http.StatusRequestTimeout || status == http.StatusGatewayTimeout:
		terr := alexerrors.NewTransientError(baseErr, "Upstream service timed out. Please retry.")
		terr.StatusCode = status
		return terr
	case status >= 500:
		terr := alexerrors.NewTransientError(baseErr, "Upstream service temporarily unavailable. Please retry.")
		terr.StatusCode = status
		return terr
	case status >= 400:
		perr := alexerrors.NewPermanentError(baseErr, "Request was rejected by the upstream service.")
		perr.StatusCode = status
		return perr
	default:
		terr := alexerrors.NewTransientError(baseErr, "Unexpected response from upstream service. Please retry.")
		terr.StatusCode = status
		return terr
	}
}

func parseRetryAfter(value string) int {
	if value == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return 0
		}
		return seconds
	}

	if t, err := http.ParseTime(value); err == nil {
		delta := int(time.Until(t).Seconds())
		if delta < 0 {
			return 0
		}
		return delta
	}

	return 0
}
