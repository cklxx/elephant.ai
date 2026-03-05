package httpclient

import (
	"net/http"
	"time"

	"alex/internal/shared/logging"
)

// New returns an http.Client configured for outbound requests.
//
// It respects HTTP(S)_PROXY/ALL_PROXY/NO_PROXY by default, but may bypass
// unreachable loopback proxies to keep local development environments working.
func New(timeout time.Duration, logger logging.Logger) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: Transport(logger),
	}
}

// Transport returns an http.Transport clone with a proxy policy suitable for
// outbound calls.
func Transport(logger logging.Logger) *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Transport{Proxy: proxyFunc(logger)}
	}

	transport := base.Clone()
	transport.Proxy = proxyFunc(logger)
	return transport
}
