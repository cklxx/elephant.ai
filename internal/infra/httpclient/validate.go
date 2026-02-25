package httpclient

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// URLValidationOptions controls outbound URL validation rules.
type URLValidationOptions struct {
	AllowLocalhost       bool
	AllowPrivateNetworks bool
}

// DefaultURLValidationOptions returns the baseline outbound fetch rules.
func DefaultURLValidationOptions() URLValidationOptions {
	return URLValidationOptions{}
}

// ValidateOutboundURL ensures the URL is well-formed and avoids local/private targets by default.
func ValidateOutboundURL(raw string, opts URLValidationOptions) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("url is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme: %s", scheme)
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return nil, fmt.Errorf("url host is required")
	}
	if !opts.AllowLocalhost && isLocalHostname(host) {
		return nil, fmt.Errorf("local urls are not allowed")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !opts.AllowLocalhost && (ip.IsLoopback() || ip.IsUnspecified()) {
			return nil, fmt.Errorf("local urls are not allowed")
		}
		if !opts.AllowPrivateNetworks && isPrivateIP(ip) {
			return nil, fmt.Errorf("private network urls are not allowed")
		}
	}
	return parsed, nil
}

func isLocalHostname(host string) bool {
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if strings.HasSuffix(host, ".localhost") {
		return true
	}
	return false
}

func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsPrivate() {
		return true
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	return false
}
