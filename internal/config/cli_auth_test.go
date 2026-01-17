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
	if creds.Codex.Model != "gpt-5-codex" {
		t.Fatalf("expected codex model, got %q", creds.Codex.Model)
	}
}
