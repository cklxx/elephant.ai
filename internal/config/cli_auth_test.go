package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCLICredentialsReadsCodexAuth(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	codexDir := filepath.Join(tmp, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	auth := `{"tokens":{"access_token":"tok-123","account_id":"acct"}}`
	if err := os.WriteFile(filepath.Join(codexDir, "auth.json"), []byte(auth), 0o600); err != nil {
		t.Fatalf("write auth: %v", err)
	}
	toml := "model = \"gpt-5-codex\"\n"
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte(toml), 0o600); err != nil {
		t.Fatalf("write toml: %v", err)
	}

	creds := LoadCLICredentials(
		WithHomeDir(func() (string, error) { return tmp, nil }),
		WithEnv(func(string) (string, bool) { return "", false }),
	)

	if creds.Codex.APIKey != "tok-123" {
		t.Fatalf("expected codex api key, got %q", creds.Codex.APIKey)
	}
	if creds.Codex.AccountID != "acct" {
		t.Fatalf("expected codex account id, got %q", creds.Codex.AccountID)
	}
	if creds.Codex.Model != "gpt-5-codex" {
		t.Fatalf("expected codex model, got %q", creds.Codex.Model)
	}
}

func TestLoadCLICredentialsReadsGeminiOAuthForAntigravity(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	geminiDir := filepath.Join(tmp, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	oauth := `{"access_token":"ag-access","refresh_token":"ag-refresh","expiry_date":123}`
	if err := os.WriteFile(filepath.Join(geminiDir, "oauth_creds.json"), []byte(oauth), 0o600); err != nil {
		t.Fatalf("write oauth: %v", err)
	}

	creds := LoadCLICredentials(
		WithHomeDir(func() (string, error) { return tmp, nil }),
		WithEnv(func(string) (string, bool) { return "", false }),
	)

	if creds.Antigravity.APIKey != "ag-access" {
		t.Fatalf("expected antigravity api key, got %q", creds.Antigravity.APIKey)
	}
	if creds.Antigravity.BaseURL != "https://cloudcode-pa.googleapis.com" {
		t.Fatalf("expected antigravity base url, got %q", creds.Antigravity.BaseURL)
	}
}
