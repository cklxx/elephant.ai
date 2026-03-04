package adapters

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	materialports "alex/internal/domain/materialregistry/ports"
	"alex/internal/shared/httpclient"
	"alex/internal/shared/logging"
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
	opts := httpclient.DefaultURLValidationOptions()
	if f.allowLocal {
		opts.AllowLocalhost = true
	}
	parsed, err := httpclient.ValidateOutboundURL(uri, opts)
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
