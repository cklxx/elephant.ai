package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type devtoolsVersionInfo struct {
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func resolveCDPURL(ctx context.Context, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("cdp url is empty")
	}
	if strings.HasPrefix(raw, "ws://") || strings.HasPrefix(raw, "wss://") {
		return raw, nil
	}

	u, err := parseDevToolsHTTPURL(raw)
	if err != nil {
		return "", err
	}

	versionURL := *u
	normalizedPath := strings.TrimRight(versionURL.Path, "/")
	if normalizedPath == "" || normalizedPath == "/json" {
		versionURL.Path = "/json/version"
	} else if normalizedPath != "/json/version" {
		versionURL.Path = "/json/version"
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionURL.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("cdp version endpoint returned %s", resp.Status)
	}

	var info devtoolsVersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}
	wsURL := strings.TrimSpace(info.WebSocketDebuggerURL)
	if wsURL == "" {
		return "", fmt.Errorf("cdp version endpoint returned empty webSocketDebuggerUrl")
	}
	return wsURL, nil
}

func parseDevToolsHTTPURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("devtools url is empty")
	}
	if port, err := strconv.Atoi(raw); err == nil && port > 0 && port <= 65535 {
		raw = fmt.Sprintf("http://127.0.0.1:%d", port)
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "http", "https":
		return u, nil
	default:
		return nil, fmt.Errorf("unsupported cdp url scheme %q", u.Scheme)
	}
}
