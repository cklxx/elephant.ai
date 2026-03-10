package config

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

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
