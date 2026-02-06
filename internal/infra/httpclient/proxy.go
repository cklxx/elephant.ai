package httpclient

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"alex/internal/shared/logging"
)

const proxyModeEnv = "ALEX_PROXY_MODE"

const (
	proxyDialTimeout = 300 * time.Millisecond
)

type proxyMode uint8

const (
	proxyModeAuto proxyMode = iota
	proxyModeStrict
	proxyModeDirect
)

var (
	resolvedProxyMode proxyMode
	proxyModeOnce     sync.Once

	localProxyBypassCache sync.Map // map[string]bool; true means bypass
	localProxyWarned      sync.Map // map[string]struct{}
)

func proxyFunc(logger logging.Logger) func(*http.Request) (*url.URL, error) {
	log := logging.OrNop(logger)

	return func(req *http.Request) (*url.URL, error) {
		switch proxyModeFromEnv() {
		case proxyModeDirect:
			return nil, nil
		case proxyModeStrict:
			return http.ProxyFromEnvironment(req)
		default:
		}

		if req == nil || req.URL == nil {
			return http.ProxyFromEnvironment(req)
		}

		if isLoopbackHost(req.URL.Hostname()) {
			return nil, nil
		}

		proxyURL, err := http.ProxyFromEnvironment(req)
		if proxyURL == nil || err != nil {
			return proxyURL, err
		}

		if !isLoopbackHost(proxyURL.Hostname()) {
			return proxyURL, nil
		}

		hostPort, ok := proxyHostPort(proxyURL)
		if !ok {
			return proxyURL, nil
		}

		cacheKey := proxyURL.String()
		if bypass, ok := localProxyBypassCache.Load(cacheKey); ok {
			if bypass.(bool) {
				logBypassedLocalProxy(log, cacheKey)
				return nil, nil
			}
			return proxyURL, nil
		}

		if isProxyReachable(req.Context(), hostPort) {
			localProxyBypassCache.Store(cacheKey, false)
			return proxyURL, nil
		}

		localProxyBypassCache.Store(cacheKey, true)
		logBypassedLocalProxy(log, cacheKey)
		return nil, nil
	}
}

func proxyModeFromEnv() proxyMode {
	proxyModeOnce.Do(func() {
		value, _ := os.LookupEnv(proxyModeEnv)
		raw := strings.ToLower(strings.TrimSpace(value))
		switch raw {
		case "", "auto":
			resolvedProxyMode = proxyModeAuto
		case "strict":
			resolvedProxyMode = proxyModeStrict
		case "direct", "none", "off":
			resolvedProxyMode = proxyModeDirect
		default:
			resolvedProxyMode = proxyModeAuto
		}
	})

	return resolvedProxyMode
}

func logBypassedLocalProxy(logger logging.Logger, proxyURL string) {
	if logging.IsNil(logger) {
		return
	}
	if _, loaded := localProxyWarned.LoadOrStore(proxyURL, struct{}{}); loaded {
		return
	}

	redacted := proxyURL
	if parsed, err := url.Parse(proxyURL); err == nil {
		redacted = parsed.Redacted()
	}

	logger.Warn("Local proxy %s is unreachable; bypassing proxy for outbound HTTP requests (set %s=strict to disable).", redacted, proxyModeEnv)
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	if ip.IsUnspecified() {
		return true
	}
	return false
}

func proxyHostPort(proxyURL *url.URL) (string, bool) {
	if proxyURL == nil {
		return "", false
	}

	host := strings.TrimSpace(proxyURL.Hostname())
	if host == "" {
		return "", false
	}

	port := strings.TrimSpace(proxyURL.Port())
	if port == "" {
		scheme := strings.ToLower(strings.TrimSpace(proxyURL.Scheme))
		if scheme == "" {
			scheme = "http"
		}
		switch scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		case "socks5", "socks5h":
			port = "1080"
		default:
			return "", false
		}
	}

	return net.JoinHostPort(host, port), true
}

func isProxyReachable(ctx context.Context, hostPort string) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	dialer := net.Dialer{Timeout: proxyDialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", hostPort)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
