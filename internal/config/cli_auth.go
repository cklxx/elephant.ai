package config

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	codexCLIBaseURL        = "https://chatgpt.com/backend-api/codex"
	antigravityDefaultBase = "https://cloudcode-pa.googleapis.com"
	// From proxycast antigravity.rs (Antigravity CLI OAuth client) + Google OAuth token endpoint.
	antigravityOAuthTokenURL     = "https://oauth2.googleapis.com/token"
	antigravityOAuthClientID     = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
	antigravityOAuthClientSecret = "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"
	antigravityOAuthRefreshSkew  = 5 * time.Minute
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
	Codex       CLICredential
	Claude      CLICredential
	Antigravity CLICredential
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
		Codex:       loadCodexCLIAuth(readFile, home),
		Claude:      loadClaudeCLIAuth(envLookup, readFile, home),
		Antigravity: loadAntigravityCLIAuth(envLookup, readFile, home),
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
	Tokens struct {
		AccessToken string `json:"access_token"`
		AccountID   string `json:"account_id"`
	} `json:"tokens"`
}

type geminiOAuthFile struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiryDate   int64  `json:"expiry_date"`
	ExpiresIn    int64  `json:"expires_in"`
	Timestamp    int64  `json:"timestamp"`
	Expire       string `json:"expire"`
	TokenURI     string `json:"token_uri"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

func loadCodexCLIAuth(readFile func(string) ([]byte, error), home string) CLICredential {
	if readFile == nil || home == "" {
		return CLICredential{}
	}
	data, err := readFile(filepath.Join(home, ".codex", "auth.json"))
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

func loadAntigravityCLIAuth(envLookup EnvLookup, readFile func(string) ([]byte, error), home string) CLICredential {
	if readFile == nil {
		return CLICredential{}
	}
	if cred := loadAntigravityGeminiOAuth(readFile, home); cred.APIKey != "" {
		return cred
	}
	for _, path := range antigravityCLIAuthPaths(envLookup, home) {
		data, err := readFile(path)
		if err != nil {
			continue
		}
		token, baseURL, model := parseAntigravityCLIAuth(data)
		if token == "" {
			continue
		}
		return CLICredential{
			Provider: "antigravity",
			APIKey:   token,
			BaseURL:  strings.TrimSpace(baseURL),
			Model:    strings.TrimSpace(model),
			Source:   SourceAntigravityCLI,
		}
	}
	return CLICredential{}
}

func loadAntigravityGeminiOAuth(readFile func(string) ([]byte, error), home string) CLICredential {
	if readFile == nil || home == "" {
		return CLICredential{}
	}
	now := time.Now()
	for _, path := range antigravityOAuthPaths(home) {
		data, err := readFile(path)
		if err != nil {
			continue
		}
		payload, ok := parseAntigravityOAuthFile(data)
		if !ok {
			continue
		}

		token := strings.TrimSpace(payload.AccessToken)
		needsRefresh, expired := antigravityOAuthNeedsRefresh(payload, now)
		if needsRefresh && strings.TrimSpace(payload.RefreshToken) != "" {
			refreshed, err := refreshAntigravityOAuth(payload)
			if err == nil {
				payload = refreshed
				token = strings.TrimSpace(payload.AccessToken)
				_ = writeAntigravityOAuthFile(path, payload)
			} else if expired {
				continue
			}
		} else if expired {
			continue
		}

		if token == "" {
			continue
		}

		return CLICredential{
			Provider: "antigravity",
			APIKey:   token,
			BaseURL:  antigravityDefaultBase,
			Source:   SourceAntigravityCLI,
		}
	}
	return CLICredential{}
}

func antigravityOAuthPaths(home string) []string {
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".antigravity", "oauth_creds.json"),
		filepath.Join(home, ".gemini", "oauth_creds.json"),
	}
}

func parseAntigravityOAuthFile(data []byte) (geminiOAuthFile, bool) {
	var payload geminiOAuthFile
	if err := json.Unmarshal(data, &payload); err == nil {
		if strings.TrimSpace(payload.AccessToken) != "" || strings.TrimSpace(payload.RefreshToken) != "" {
			return payload, true
		}
	}
	var list []geminiOAuthFile
	if err := json.Unmarshal(data, &list); err != nil {
		return geminiOAuthFile{}, false
	}
	for _, item := range list {
		if strings.TrimSpace(item.AccessToken) != "" || strings.TrimSpace(item.RefreshToken) != "" {
			return item, true
		}
	}
	return geminiOAuthFile{}, false
}

func antigravityOAuthNeedsRefresh(payload geminiOAuthFile, now time.Time) (bool, bool) {
	expiry, ok := antigravityOAuthExpiry(payload)
	if !ok {
		return false, false
	}
	if expiry.Before(now) {
		return true, true
	}
	if expiry.Before(now.Add(antigravityOAuthRefreshSkew)) {
		return true, false
	}
	return false, false
}

func antigravityOAuthExpiry(payload geminiOAuthFile) (time.Time, bool) {
	if strings.TrimSpace(payload.Expire) != "" {
		if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(payload.Expire)); err == nil {
			return parsed, true
		}
	}
	if payload.ExpiryDate > 0 {
		return time.UnixMilli(payload.ExpiryDate), true
	}
	if payload.Timestamp > 0 && payload.ExpiresIn > 0 {
		return time.UnixMilli(payload.Timestamp).Add(time.Duration(payload.ExpiresIn) * time.Second), true
	}
	return time.Time{}, false
}

func refreshAntigravityOAuth(payload geminiOAuthFile) (geminiOAuthFile, error) {
	refreshToken := strings.TrimSpace(payload.RefreshToken)
	if refreshToken == "" {
		return geminiOAuthFile{}, io.ErrUnexpectedEOF
	}

	tokenURL := strings.TrimSpace(payload.TokenURI)
	if tokenURL == "" {
		tokenURL = antigravityOAuthTokenURL
	}

	clientID := strings.TrimSpace(payload.ClientID)
	if clientID == "" {
		clientID = antigravityOAuthClientID
	}

	clientSecret := strings.TrimSpace(payload.ClientSecret)
	if clientSecret == "" {
		clientSecret = antigravityOAuthClientSecret
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("refresh_token", refreshToken)
	form.Set("grant_type", "refresh_token")

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return geminiOAuthFile{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return geminiOAuthFile{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return geminiOAuthFile{}, io.ErrUnexpectedEOF
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return geminiOAuthFile{}, err
	}
	var refreshed struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &refreshed); err != nil {
		return geminiOAuthFile{}, err
	}
	if strings.TrimSpace(refreshed.AccessToken) == "" {
		return geminiOAuthFile{}, io.ErrUnexpectedEOF
	}

	updated := payload
	updated.AccessToken = strings.TrimSpace(refreshed.AccessToken)
	if strings.TrimSpace(refreshed.RefreshToken) != "" {
		updated.RefreshToken = strings.TrimSpace(refreshed.RefreshToken)
	}
	if refreshed.ExpiresIn > 0 {
		now := time.Now()
		updated.ExpiresIn = refreshed.ExpiresIn
		updated.Timestamp = now.UnixMilli()
		updated.ExpiryDate = now.Add(time.Duration(refreshed.ExpiresIn) * time.Second).UnixMilli()
		updated.Expire = now.Add(time.Duration(refreshed.ExpiresIn) * time.Second).Format(time.RFC3339)
	}
	if strings.TrimSpace(refreshed.TokenType) != "" {
		updated.TokenType = strings.TrimSpace(refreshed.TokenType)
	}

	return updated, nil
}

func writeAntigravityOAuthFile(path string, payload geminiOAuthFile) error {
	if path == "" {
		return io.ErrUnexpectedEOF
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func antigravityCLIAuthPaths(envLookup EnvLookup, home string) []string {
	var paths []string
	if value, ok := lookupEnv(envLookup, "ALEX_CLI_AUTH_PATH"); ok {
		paths = append(paths, value)
	}
	if dataHome := resolveXDGDataHome(envLookup, home); dataHome != "" {
		paths = append(paths, filepath.Join(dataHome, "opencode", "auth.json"))
	}
	if configHome := resolveXDGConfigHome(envLookup, home); configHome != "" {
		paths = append(paths, filepath.Join(configHome, "opencode", "auth.json"))
	}
	if home != "" {
		paths = append(paths, filepath.Join(home, "Library", "Application Support", "opencode", "auth.json"))
	}

	seen := map[string]struct{}{}
	unique := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		cleaned := filepath.Clean(path)
		if _, exists := seen[cleaned]; exists {
			continue
		}
		seen[cleaned] = struct{}{}
		unique = append(unique, cleaned)
	}
	return unique
}

func resolveXDGDataHome(envLookup EnvLookup, home string) string {
	if value, ok := lookupEnv(envLookup, "XDG_DATA_HOME"); ok {
		return value
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "share")
}

func resolveXDGConfigHome(envLookup EnvLookup, home string) string {
	if value, ok := lookupEnv(envLookup, "XDG_CONFIG_HOME"); ok {
		return value
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".config")
}

func lookupEnv(envLookup EnvLookup, key string) (string, bool) {
	if envLookup == nil {
		envLookup = DefaultEnvLookup
	}
	value, ok := envLookup(key)
	value = strings.TrimSpace(value)
	if !ok || value == "" {
		return "", false
	}
	return value, true
}

func parseAntigravityCLIAuth(data []byte) (string, string, string) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", "", ""
	}

	if token, baseURL, model := extractProviderCredentials(raw, "antigravity"); token != "" {
		return token, baseURL, model
	}
	if token, baseURL, model := extractProviderCredentials(raw, "google"); token != "" {
		return token, baseURL, model
	}

	if provider, ok := raw["provider"].(string); ok {
		if providerMatchesAntigravity(provider) {
			return extractTokenFields(raw)
		}
		return "", "", ""
	}

	return "", "", ""
}

func providerMatchesAntigravity(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "antigravity", "google":
		return true
	default:
		return false
	}
}

func extractProviderCredentials(raw map[string]any, provider string) (string, string, string) {
	for _, key := range []string{"providers", "accounts", "auths"} {
		container, ok := raw[key].(map[string]any)
		if !ok {
			continue
		}
		block, ok := container[provider].(map[string]any)
		if !ok {
			continue
		}
		return extractTokenFields(block)
	}
	return "", "", ""
}

func extractTokenFields(raw map[string]any) (string, string, string) {
	token := firstString(raw, "api_key", "access_token", "access", "token")
	baseURL := firstString(raw, "base_url", "baseUrl", "endpoint", "api_base_url")
	model := firstString(raw, "model")
	return token, baseURL, model
}

func firstString(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok {
			continue
		}
		if str, ok := value.(string); ok {
			if trimmed := strings.TrimSpace(str); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
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
