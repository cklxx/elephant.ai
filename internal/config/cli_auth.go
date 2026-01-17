package config

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const codexCLIBaseURL = "https://chatgpt.com/backend-api/codex"

type CLICredential struct {
	Provider string
	APIKey   string
	BaseURL  string
	Model    string
	Source   ValueSource
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

	model := strings.TrimSpace(loadCodexCLIModel(readFile, home))

	return CLICredential{
		Provider: "codex",
		APIKey:   token,
		BaseURL:  codexCLIBaseURL,
		Model:    model,
		Source:   SourceCodexCLI,
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
