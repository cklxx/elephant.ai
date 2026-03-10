package config

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"time"
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
		cmdRunner: defaultCmdRunner,
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

// defaultCmdRunner executes a command and returns its combined output.
func defaultCmdRunner(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
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
