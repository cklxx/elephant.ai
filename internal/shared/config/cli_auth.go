package config

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"alex/internal/shared/httpclient"
	"alex/internal/shared/utils"
)

const (
	codexCLIBaseURL = "https://chatgpt.com/backend-api/codex"

	codexOAuthTokenURL    = "https://auth.openai.com/oauth/token"
	codexOAuthRefreshSkew = 5 * time.Minute

	claudeOAuthTokenURL    = "https://platform.claude.com/v1/oauth/token"
	claudeOAuthClientID    = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	claudeOAuthRefreshSkew = 5 * time.Minute
)

type CLICredential struct {
	Provider  string
	APIKey    string
	AccountID string
	BaseURL   string
	Model     string
	Source    ValueSource
}

type CLICredentials struct {
	Codex  CLICredential
	Claude CLICredential
}

func LoadCLICredentials(opts ...Option) CLICredentials {
	options := loadOptions{
		envLookup: DefaultEnvLookup,
		readFile:  os.ReadFile,
		homeDir:   os.UserHomeDir,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return loadCLICredentials(options)
}

func loadCLICredentials(opts loadOptions) CLICredentials {
	readFile := opts.readFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	envLookup := opts.envLookup
	if envLookup == nil {
		envLookup = DefaultEnvLookup
	}
	home := resolveHomeDir(opts.homeDir)

	return CLICredentials{
		Codex:  loadCodexCLIAuth(readFile, home),
		Claude: loadClaudeCLIAuth(envLookup, readFile, home, opts.cmdRunner),
	}
}

func resolveHomeDir(homeDir func() (string, error)) string {
	if homeDir != nil {
		if resolved, err := homeDir(); err == nil {
			if trimmed := strings.TrimSpace(resolved); trimmed != "" {
				return trimmed
			}
		}
	}
	if resolved, err := os.UserHomeDir(); err == nil {
		return strings.TrimSpace(resolved)
	}
	return ""
}

type codexAuthFile struct {
	OpenAIAPIKey *string `json:"OPENAI_API_KEY"`
	Tokens       struct {
		IDToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		AccountID    string `json:"account_id"`
	} `json:"tokens"`
	LastRefresh string `json:"last_refresh"`
	TokenURL    string `json:"token_url,omitempty"` // override for testing; defaults to codexOAuthTokenURL
}

func loadCodexCLIAuth(readFile func(string) ([]byte, error), home string) CLICredential {
	if readFile == nil || home == "" {
		return CLICredential{}
	}
	path := filepath.Join(home, ".codex", "auth.json")
	data, err := readFile(path)
	if err != nil {
		return CLICredential{}
	}

	var payload codexAuthFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return CLICredential{}
	}
	token := strings.TrimSpace(payload.Tokens.AccessToken)
	if token == "" {
		return CLICredential{}
	}
	accountID := strings.TrimSpace(payload.Tokens.AccountID)

	// Check if the access token needs refresh.
	now := time.Now()
	if needsRefresh, _ := codexOAuthNeedsRefresh(token, now); needsRefresh {
		if utils.HasContent(payload.Tokens.RefreshToken) {
			refreshed, err := refreshCodexOAuth(payload)
			if err == nil {
				payload = refreshed
				token = strings.TrimSpace(payload.Tokens.AccessToken)
				_ = writeCodexAuthFile(path, payload)
			}
			// On failure, fall through with the existing token so the
			// provider surfaces with an auth error instead of hiding.
		}
	}

	model := strings.TrimSpace(loadCodexCLIModel(readFile, home))

	return CLICredential{
		Provider:  "codex",
		APIKey:    token,
		AccountID: accountID,
		BaseURL:   codexCLIBaseURL,
		Model:     model,
		Source:    SourceCodexCLI,
	}
}

func loadCodexCLIModel(readFile func(string) ([]byte, error), home string) string {
	if readFile == nil || home == "" {
		return ""
	}
	data, err := readFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		return ""
	}

	return parseTomlStringValue(data, "model")
}

func parseTomlStringValue(data []byte, key string) string {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		if !strings.HasPrefix(line, key) {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.TrimSpace(parts[0]) != key {
			continue
		}
		value := strings.TrimSpace(parts[1])
		if idx := strings.Index(value, "#"); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
		value = strings.Trim(value, "\"'")
		if value != "" {
			return value
		}
	}
	return ""
}

// parseJWTExpiry decodes the JWT payload (without signature verification)
// and returns the expiration time from the "exp" claim.
func parseJWTExpiry(token string) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp <= 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
}

// parseJWTClientID extracts the client_id claim from a JWT payload.
func parseJWTClientID(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		ClientID string `json:"client_id"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return strings.TrimSpace(claims.ClientID)
}

func codexOAuthNeedsRefresh(token string, now time.Time) (bool, bool) {
	expiry, ok := parseJWTExpiry(token)
	if !ok {
		return false, false
	}
	if expiry.Before(now) {
		return true, true
	}
	if expiry.Before(now.Add(codexOAuthRefreshSkew)) {
		return true, false
	}
	return false, false
}

func refreshCodexOAuth(payload codexAuthFile) (codexAuthFile, error) {
	refreshToken := strings.TrimSpace(payload.Tokens.RefreshToken)
	if refreshToken == "" {
		return codexAuthFile{}, io.ErrUnexpectedEOF
	}

	tokenURL := strings.TrimSpace(payload.TokenURL)
	if tokenURL == "" {
		tokenURL = codexOAuthTokenURL
	}

	clientID := parseJWTClientID(payload.Tokens.AccessToken)
	if clientID == "" {
		return codexAuthFile{}, fmt.Errorf("cannot extract client_id from Codex JWT")
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("refresh_token", refreshToken)
	form.Set("grant_type", "refresh_token")

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return codexAuthFile{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := httpclient.New(10*time.Second, nil)
	resp, err := client.Do(req)
	if err != nil {
		return codexAuthFile{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return codexAuthFile{}, fmt.Errorf("codex token refresh failed: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return codexAuthFile{}, err
	}
	var refreshed struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &refreshed); err != nil {
		return codexAuthFile{}, err
	}
	if utils.IsBlank(refreshed.AccessToken) {
		return codexAuthFile{}, io.ErrUnexpectedEOF
	}

	updated := payload
	updated.Tokens.AccessToken = strings.TrimSpace(refreshed.AccessToken)
	if utils.HasContent(refreshed.RefreshToken) {
		updated.Tokens.RefreshToken = strings.TrimSpace(refreshed.RefreshToken)
	}
	if utils.HasContent(refreshed.IDToken) {
		updated.Tokens.IDToken = strings.TrimSpace(refreshed.IDToken)
	}
	updated.LastRefresh = time.Now().UTC().Format(time.RFC3339Nano)

	return updated, nil
}

func writeCodexAuthFile(path string, payload codexAuthFile) error {
	if path == "" {
		return io.ErrUnexpectedEOF
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

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

func loadClaudeCLIAuth(envLookup EnvLookup, readFile func(string) ([]byte, error), home string, cmdRunner func(string, ...string) ([]byte, error)) CLICredential {
	// Priority 1: env vars.
	if token := lookupClaudeOAuthToken(envLookup); token != "" {
		return CLICredential{
			Provider: "anthropic",
			APIKey:   token,
			Source:   SourceClaudeCLI,
		}
	}

	// Priority 2: credential files.
	if readFile != nil && home != "" {
		for _, path := range claudeAuthPaths(home) {
			if cred := loadClaudeFromFile(readFile, path); cred.APIKey != "" {
				return cred
			}
		}
	}

	// Priority 3: macOS Keychain (Claude Code v2.1+).
	if cred := loadClaudeFromKeychain(cmdRunner); cred.APIKey != "" {
		return cred
	}

	// Priority 4: claude setup-token CLI.
	if cred := loadClaudeFromSetupToken(cmdRunner); cred.APIKey != "" {
		return cred
	}

	return CLICredential{}
}

// loadClaudeFromFile attempts to load Claude credentials from a single file path.
func loadClaudeFromFile(readFile func(string) ([]byte, error), path string) CLICredential {
	data, err := readFile(path)
	if err != nil {
		return CLICredential{}
	}

	// Try to parse the full claudeAiOauth structure so we can refresh when needed.
	var creds claudeOAuthFile
	if jsonErr := json.Unmarshal(data, &creds); jsonErr == nil && creds.ClaudeAiOauth.AccessToken != "" {
		now := time.Now()
		if claudeOAuthNeedsRefresh(creds.ClaudeAiOauth.ExpiresAt, now) && creds.ClaudeAiOauth.RefreshToken != "" {
			refreshed, refreshErr := refreshClaudeOAuth(creds)
			if refreshErr == nil {
				creds = refreshed
				_ = writeClaudeAuthFile(path, creds)
			}
		}
		if token := strings.TrimSpace(creds.ClaudeAiOauth.AccessToken); token != "" {
			return CLICredential{
				Provider: "anthropic",
				APIKey:   token,
				Source:   SourceClaudeCLI,
			}
		}
	}

	// Fall back to generic token extraction for other credential file formats.
	token := extractJSONToken(data, []string{"access_token", "token", "api_key"})
	if token == "" {
		return CLICredential{}
	}
	return CLICredential{
		Provider: "anthropic",
		APIKey:   token,
		Source:   SourceClaudeCLI,
	}
}

// defaultCmdRunner executes a command and returns its combined output.
func defaultCmdRunner(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

// loadClaudeFromKeychain reads Claude Code OAuth credentials from the macOS Keychain.
// Claude Code v2.1+ stores credentials under service "Claude Code-credentials".
// A nil cmdRunner means external commands are disabled (e.g. in tests); returns empty.
func loadClaudeFromKeychain(cmdRunner func(string, ...string) ([]byte, error)) CLICredential {
	if cmdRunner == nil || runtime.GOOS != "darwin" {
		return CLICredential{}
	}

	out, err := cmdRunner("security", "find-generic-password", "-s", "Claude Code-credentials", "-w")
	if err != nil {
		return CLICredential{}
	}

	data := bytes.TrimSpace(out)
	if len(data) == 0 {
		return CLICredential{}
	}

	return parseClaudeOAuthData(data)
}

// loadClaudeFromSetupToken runs `claude setup-token` to generate an OAuth token.
// A nil cmdRunner means external commands are disabled; returns empty.
func loadClaudeFromSetupToken(cmdRunner func(string, ...string) ([]byte, error)) CLICredential {
	if cmdRunner == nil {
		return CLICredential{}
	}

	out, err := cmdRunner("claude", "setup-token")
	if err != nil {
		return CLICredential{}
	}

	data := bytes.TrimSpace(out)
	if len(data) == 0 {
		return CLICredential{}
	}

	return parseClaudeOAuthData(data)
}

// parseClaudeOAuthData parses JSON output (from Keychain or setup-token) into a CLICredential.
func parseClaudeOAuthData(data []byte) CLICredential {
	var creds claudeOAuthFile
	if err := json.Unmarshal(data, &creds); err == nil && creds.ClaudeAiOauth.AccessToken != "" {
		now := time.Now()
		if claudeOAuthNeedsRefresh(creds.ClaudeAiOauth.ExpiresAt, now) && creds.ClaudeAiOauth.RefreshToken != "" {
			refreshed, refreshErr := refreshClaudeOAuth(creds)
			if refreshErr == nil {
				creds = refreshed
			}
		}
		if token := strings.TrimSpace(creds.ClaudeAiOauth.AccessToken); token != "" {
			return CLICredential{
				Provider: "anthropic",
				APIKey:   token,
				Source:   SourceClaudeCLI,
			}
		}
	}

	// Try generic token extraction.
	token := extractJSONToken(data, []string{"access_token", "token", "api_key"})
	if token != "" {
		return CLICredential{
			Provider: "anthropic",
			APIKey:   token,
			Source:   SourceClaudeCLI,
		}
	}
	return CLICredential{}
}

func lookupClaudeOAuthToken(envLookup EnvLookup) string {
	if envLookup == nil {
		envLookup = DefaultEnvLookup
	}
	if value, ok := envLookup("CLAUDE_CODE_OAUTH_TOKEN"); ok {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	if value, ok := envLookup("ANTHROPIC_AUTH_TOKEN"); ok {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func claudeAuthPaths(home string) []string {
	return []string{
		filepath.Join(home, ".claude", ".credentials.json"),
		filepath.Join(home, ".claude", "credentials.json"),
		filepath.Join(home, ".config", "claude", ".credentials.json"),
		filepath.Join(home, ".config", "claude", "credentials.json"),
	}
}

func extractJSONToken(data []byte, keys []string) string {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			if str, ok := value.(string); ok {
				if trimmed := strings.TrimSpace(str); trimmed != "" {
					return trimmed
				}
			}
		}
	}
	// Claude Code stores OAuth credentials nested under claudeAiOauth.accessToken.
	if oauth, ok := raw["claudeAiOauth"].(map[string]any); ok {
		if token, ok := oauth["accessToken"].(string); ok {
			if trimmed := strings.TrimSpace(token); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}
