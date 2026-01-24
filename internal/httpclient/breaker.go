package httpclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	alexerrors "alex/internal/errors"
	"alex/internal/logging"
)

type circuitBreakerRoundTripper struct {
	base    http.RoundTripper
	breaker *alexerrors.CircuitBreaker
}

// NewWithCircuitBreaker builds an HTTP client guarded by a circuit breaker.
func NewWithCircuitBreaker(timeout time.Duration, logger logging.Logger, name string) *http.Client {
	return NewWithCircuitBreakerConfig(timeout, logger, name, alexerrors.DefaultCircuitBreakerConfig())
}

// NewWithCircuitBreakerConfig builds an HTTP client guarded by a custom circuit breaker config.
func NewWithCircuitBreakerConfig(timeout time.Duration, logger logging.Logger, name string, config alexerrors.CircuitBreakerConfig) *http.Client {
	client := New(timeout, logger)
	client.Transport = WrapTransportWithCircuitBreaker(client.Transport, name, config)
	return client
}

// WrapTransportWithCircuitBreaker wraps a transport with circuit breaker protection.
func WrapTransportWithCircuitBreaker(base http.RoundTripper, name string, config alexerrors.CircuitBreakerConfig) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if name == "" {
		name = "http-client"
	}
	return &circuitBreakerRoundTripper{
		base:    base,
		breaker: alexerrors.NewCircuitBreaker(name, config),
	}
}

func (t *circuitBreakerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}
	if err := t.breaker.Allow(); err != nil {
		return nil, err
	}
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			t.breaker.Mark(nil)
			return nil, err
		}
		t.breaker.Mark(err)
		return nil, err
	}
	if isBreakerFailureStatus(resp.StatusCode) {
		t.breaker.Mark(fmt.Errorf("http status %d", resp.StatusCode))
	} else {
		t.breaker.Mark(nil)
	}
	return resp, nil
}

func isBreakerFailureStatus(status int) bool {
	return status >= http.StatusInternalServerError || status == http.StatusTooManyRequests
}
