package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"alex/internal/shared/httpclient"
	"alex/internal/shared/utils"
)

type claudeOAuthFile struct {
	ClaudeAiOauth claudeOAuthTokens `json:"claudeAiOauth"`
}

type claudeOAuthTokens struct {
	AccessToken      string   `json:"accessToken"`
	RefreshToken     string   `json:"refreshToken"`
	ExpiresAt        int64    `json:"expiresAt"` // Unix milliseconds
	Scopes           []string `json:"scopes,omitempty"`
	SubscriptionType string   `json:"subscriptionType,omitempty"`
}

// claudeOAuthNeedsRefresh returns true when the access token expires within the refresh skew window.
// expiresAtMs is a Unix timestamp in milliseconds; zero means unknown (no refresh).
func claudeOAuthNeedsRefresh(expiresAtMs int64, now time.Time) bool {
	if expiresAtMs <= 0 {
		return false
	}
	expiry := time.UnixMilli(expiresAtMs)
	return expiry.Before(now.Add(claudeOAuthRefreshSkew))
}

func refreshClaudeOAuth(payload claudeOAuthFile) (claudeOAuthFile, error) {
	refreshToken := strings.TrimSpace(payload.ClaudeAiOauth.RefreshToken)
	if refreshToken == "" {
		return claudeOAuthFile{}, io.ErrUnexpectedEOF
	}

	form := url.Values{}
	form.Set("client_id", claudeOAuthClientID)
	form.Set("refresh_token", refreshToken)
	form.Set("grant_type", "refresh_token")

	req, err := http.NewRequest(http.MethodPost, claudeOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return claudeOAuthFile{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := httpclient.New(10*time.Second, nil)
	resp, err := client.Do(req)
	if err != nil {
		return claudeOAuthFile{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return claudeOAuthFile{}, fmt.Errorf("claude token refresh failed: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return claudeOAuthFile{}, err
	}
	var refreshed struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &refreshed); err != nil {
		return claudeOAuthFile{}, err
	}
	if utils.IsBlank(refreshed.AccessToken) {
		return claudeOAuthFile{}, io.ErrUnexpectedEOF
	}

	updated := payload
	updated.ClaudeAiOauth.AccessToken = strings.TrimSpace(refreshed.AccessToken)
	if utils.HasContent(refreshed.RefreshToken) {
		updated.ClaudeAiOauth.RefreshToken = strings.TrimSpace(refreshed.RefreshToken)
	}
	if refreshed.ExpiresIn > 0 {
		updated.ClaudeAiOauth.ExpiresAt = time.Now().Add(time.Duration(refreshed.ExpiresIn) * time.Second).UnixMilli()
	}
	return updated, nil
}

func writeClaudeAuthFile(path string, payload claudeOAuthFile) error {
	if path == "" {
		return io.ErrUnexpectedEOF
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ForceRefreshClaudeOAuth loads the Claude OAuth credential from macOS Keychain,
// unconditionally refreshes it using the refresh token, writes the updated
// credential back to the Keychain, and returns the new access token.
// This is called reactively by the retry client when a 401 is received for an
// OAuth token whose local expiresAt has not yet been reached (server-side revocation).
func ForceRefreshClaudeOAuth() (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("keychain refresh only supported on macOS")
	}

	out, err := exec.Command("security", "find-generic-password", "-s", "Claude Code-credentials", "-w").Output()
	if err != nil {
		return "", fmt.Errorf("read keychain: %w", err)
	}
	data := bytes.TrimSpace(out)
	if len(data) == 0 {
		return "", fmt.Errorf("keychain entry is empty")
	}

	var creds claudeOAuthFile
	if err := json.Unmarshal(data, &creds); err != nil {
		return "", fmt.Errorf("parse keychain JSON: %w", err)
	}
	if creds.ClaudeAiOauth.RefreshToken == "" {
		return "", fmt.Errorf("no refresh token available")
	}

	refreshed, err := refreshClaudeOAuth(creds)
	if err != nil {
		return "", fmt.Errorf("refresh token: %w", err)
	}

	token := strings.TrimSpace(refreshed.ClaudeAiOauth.AccessToken)
	if token == "" {
		return "", fmt.Errorf("refreshed token is empty")
	}

	// Write back to Keychain.
	updatedJSON, err := json.Marshal(refreshed)
	if err != nil {
		return token, nil // refresh succeeded, write-back failed — still usable
	}
	_ = writeClaudeKeychainCredential(string(updatedJSON))

	return token, nil
}

// writeClaudeKeychainCredential updates the Claude Code credentials in macOS Keychain.
func writeClaudeKeychainCredential(jsonData string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("keychain write only supported on macOS")
	}
	// -U updates existing entry; -s service; -a account; -w password.
	cmd := exec.Command("security", "add-generic-password",
		"-U",
		"-s", "Claude Code-credentials",
		"-a", "claude-credentials",
		"-w", jsonData,
	)
	return cmd.Run()
}
