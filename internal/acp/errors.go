package acp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
)

// RPCStatusError reports non-2xx HTTP responses from ACP RPC endpoints.
type RPCStatusError struct {
	StatusCode int
	Body       string
}

func (e *RPCStatusError) Error() string {
	if e == nil {
		return "acp rpc status error"
	}
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return fmt.Sprintf("acp rpc status %d", e.StatusCode)
	}
	return fmt.Sprintf("acp rpc status %d: %s", e.StatusCode, body)
}

// IsRetryableError reports whether an ACP transport error is safe to retry.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	var statusErr *RPCStatusError
	if errors.As(err, &statusErr) {
		switch statusErr.StatusCode {
		case 408, 429, 500, 502, 503, 504:
			return true
		default:
			return false
		}
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}
	return false
}
