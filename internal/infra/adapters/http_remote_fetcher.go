package adapters

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

	materialports "alex/internal/domain/materialregistry/ports"
	"alex/internal/shared/httpclient"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
)

// HTTPRemoteFetcher implements materialports.RemoteFetcher using an HTTP client.
type HTTPRemoteFetcher struct {
	client        *http.Client
	maxFetchBytes int64
	allowLocal    bool
}

var _ materialports.RemoteFetcher = (*HTTPRemoteFetcher)(nil)

// NewHTTPRemoteFetcher creates a new HTTPRemoteFetcher.
func NewHTTPRemoteFetcher(client *http.Client, maxFetchBytes int64, allowLocal bool) *HTTPRemoteFetcher {
	if client == nil {
		client = httpclient.New(45*time.Second, logging.Nop())
	}
	return &HTTPRemoteFetcher{
		client:        client,
		maxFetchBytes: maxFetchBytes,
		allowLocal:    allowLocal,
	}
}

// Fetch retrieves content from the given URI after SSRF validation.
func (f *HTTPRemoteFetcher) Fetch(ctx context.Context, uri string, expectedMediaType string) ([]byte, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	opts := defaultURLValidationOptions()
	if f.allowLocal {
		opts.allowLocalhost = true
	}
	parsed, err := validateOutboundURL(uri, opts)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	reader := io.LimitReader(resp.Body, f.maxFetchBytes+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > f.maxFetchBytes {
		return nil, "", fmt.Errorf("attachment exceeds %d bytes", f.maxFetchBytes)
	}

	ct := strings.TrimSpace(expectedMediaType)
	if ct == "" {
		ct = strings.TrimSpace(resp.Header.Get("Content-Type"))
	}
	if parsedMT, _, err := mime.ParseMediaType(ct); err == nil && parsedMT != "" {
		ct = parsedMT
	}
	if ct == "" {
		ct = "application/octet-stream"
	}

	return data, ct, nil
}

// --- URL validation helpers (moved from domain) ---

type urlValidationOptions struct {
	allowLocalhost       bool
	allowPrivateNetworks bool
}

func defaultURLValidationOptions() urlValidationOptions {
	return urlValidationOptions{}
}

func validateOutboundURL(raw string, opts urlValidationOptions) (*neturl.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("url is required")
	}
	parsed, err := neturl.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	scheme := utils.TrimLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme: %s", scheme)
	}
	host := utils.TrimLower(parsed.Hostname())
	if host == "" {
		return nil, fmt.Errorf("url host is required")
	}
	if !opts.allowLocalhost && isLocalHostname(host) {
		return nil, fmt.Errorf("local urls are not allowed")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !opts.allowLocalhost && (ip.IsLoopback() || ip.IsUnspecified()) {
			return nil, fmt.Errorf("local urls are not allowed")
		}
		if !opts.allowPrivateNetworks && isPrivateIP(ip) {
			return nil, fmt.Errorf("private network urls are not allowed")
		}
	}
	return parsed, nil
}

func isLocalHostname(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	return strings.HasSuffix(host, ".localhost")
}

func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsPrivate() {
		return true
	}
	return ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}
