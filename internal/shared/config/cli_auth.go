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
	"path/filepath"
	"strings"
	"time"
)

const (
	codexCLIBaseURL = "https://chatgpt.com/backend-api/codex"

	codexOAuthTokenURL    = "https://auth.openai.com/oauth/token"
	codexOAuthRefreshSkew = 5 * time.Minute
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
		Claude: loadClaudeCLIAuth(envLookup, readFile, home),
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
		if strings.TrimSpace(payload.Tokens.RefreshToken) != "" {
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

	client := &http.Client{Timeout: 10 * time.Second}
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
	if strings.TrimSpace(refreshed.AccessToken) == "" {
		return codexAuthFile{}, io.ErrUnexpectedEOF
	}

	updated := payload
	updated.Tokens.AccessToken = strings.TrimSpace(refreshed.AccessToken)
	if strings.TrimSpace(refreshed.RefreshToken) != "" {
		updated.Tokens.RefreshToken = strings.TrimSpace(refreshed.RefreshToken)
	}
	if strings.TrimSpace(refreshed.IDToken) != "" {
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

func loadClaudeCLIAuth(envLookup EnvLookup, readFile func(string) ([]byte, error), home string) CLICredential {
	if token := lookupClaudeOAuthToken(envLookup); token != "" {
		return CLICredential{
			Provider: "anthropic",
			APIKey:   token,
			Source:   SourceClaudeCLI,
		}
	}
	if readFile == nil || home == "" {
		return CLICredential{}
	}
	for _, path := range claudeAuthPaths(home) {
		data, err := readFile(path)
		if err != nil {
			continue
		}
		token := extractJSONToken(data, []string{"access_token", "token", "api_key"})
		if token == "" {
			continue
		}
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
	return ""
}
