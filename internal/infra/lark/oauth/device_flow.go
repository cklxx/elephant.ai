package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DeviceFlowConfig holds settings for the RFC 8628 device authorization flow.
type DeviceFlowConfig struct {
	AppID     string
	AppSecret string
	Brand     string // "feishu" (default), "lark", or custom domain
	Scopes    []string
}

// DeviceAuthResponse is the response from the device authorization endpoint.
type DeviceAuthResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// DeviceTokenResponse is the successful token response from device flow polling.
type DeviceTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

var (
	ErrDeviceFlowPending  = errors.New("authorization_pending")
	ErrDeviceFlowDenied   = errors.New("access_denied")
	ErrDeviceFlowExpired  = errors.New("expired_token")
	ErrDeviceFlowSlowDown = errors.New("slow_down")
)

// resolveEndpoints maps brand to device auth and token URLs.
func resolveEndpoints(brand string) (deviceAuthURL, tokenURL string) {
	brand = strings.TrimSpace(strings.ToLower(brand))
	switch brand {
	case "", "feishu":
		return "https://accounts.feishu.cn/oauth/v1/device_authorization",
			"https://open.feishu.cn/open-apis/authen/v2/oauth/token"
	case "lark":
		return "https://accounts.larksuite.com/oauth/v1/device_authorization",
			"https://open.larksuite.com/open-apis/authen/v2/oauth/token"
	default:
		// Custom domain: assume same path structure.
		base := strings.TrimRight(brand, "/")
		if !strings.HasPrefix(base, "http") {
			base = "https://" + base
		}
		return base + "/oauth/v1/device_authorization",
			base + "/open-apis/authen/v2/oauth/token"
	}
}

// RequestDeviceAuthorization initiates the device flow (RFC 8628).
// Returns the device code, user code, and verification URL for the user to visit.
func RequestDeviceAuthorization(ctx context.Context, cfg DeviceFlowConfig) (*DeviceAuthResponse, error) {
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return nil, fmt.Errorf("app_id and app_secret required")
	}

	deviceAuthURL, _ := resolveEndpoints(cfg.Brand)

	// Always request offline_access for refresh token.
	scopes := append([]string{}, cfg.Scopes...)
	hasOffline := false
	for _, s := range scopes {
		if s == "offline_access" {
			hasOffline = true
			break
		}
	}
	if !hasOffline {
		scopes = append(scopes, "offline_access")
	}

	form := url.Values{}
	form.Set("scope", strings.Join(scopes, " "))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceAuthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build device auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cfg.AppID, cfg.AppSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device auth request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read device auth response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device auth failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var result DeviceAuthResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse device auth response: %w", err)
	}
	if result.DeviceCode == "" {
		return nil, fmt.Errorf("device auth response missing device_code: %s", string(body))
	}
	if result.Interval <= 0 {
		result.Interval = 5
	}
	return &result, nil
}

// PollDeviceToken polls the token endpoint until the user completes authorization,
// the flow is denied, or the device code expires.
// On success returns the token response; on denial/expiry returns an appropriate error.
func PollDeviceToken(ctx context.Context, cfg DeviceFlowConfig, deviceCode string, interval, expiresIn int) (*DeviceTokenResponse, error) {
	if deviceCode == "" {
		return nil, fmt.Errorf("device_code required")
	}

	_, tokenURL := resolveEndpoints(cfg.Brand)

	if interval <= 0 {
		interval = 5
	}
	if expiresIn <= 0 {
		expiresIn = 600
	}

	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)
	pollInterval := time.Duration(interval) * time.Second

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if time.Now().After(deadline) {
			return nil, ErrDeviceFlowExpired
		}

		result, err := pollOnce(ctx, cfg, tokenURL, deviceCode)
		if err == nil {
			return result, nil
		}

		switch {
		case errors.Is(err, ErrDeviceFlowPending):
			// Keep polling.
		case errors.Is(err, ErrDeviceFlowSlowDown):
			pollInterval += 5 * time.Second
		case errors.Is(err, ErrDeviceFlowDenied):
			return nil, err
		case errors.Is(err, ErrDeviceFlowExpired):
			return nil, err
		default:
			return nil, err
		}

		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func pollOnce(ctx context.Context, cfg DeviceFlowConfig, tokenURL, deviceCode string) (*DeviceTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	form.Set("device_code", deviceCode)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build token poll request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cfg.AppID, cfg.AppSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token poll request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token poll response: %w", err)
	}

	// Check for error responses.
	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
		switch errResp.Error {
		case "authorization_pending":
			return nil, ErrDeviceFlowPending
		case "slow_down":
			return nil, ErrDeviceFlowSlowDown
		case "access_denied":
			return nil, ErrDeviceFlowDenied
		case "expired_token":
			return nil, ErrDeviceFlowExpired
		default:
			return nil, fmt.Errorf("token poll error: %s", errResp.Error)
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token poll failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var result DeviceTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse token poll response: %w", err)
	}
	if result.AccessToken == "" {
		return nil, fmt.Errorf("token poll response missing access_token")
	}
	return &result, nil
}
