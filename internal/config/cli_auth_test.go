package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
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
	future := time.Now().Add(2 * time.Hour).UnixMilli()
	oauth := fmt.Sprintf(`{"access_token":"ag-access","refresh_token":"ag-refresh","expiry_date":%d}`, future)
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

func TestLoadCLICredentialsReadsAntigravityOAuthPath(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	agDir := filepath.Join(tmp, ".antigravity")
	if err := os.MkdirAll(agDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	oauth := `{"access_token":"ag-access","refresh_token":"ag-refresh","expiry_date":4102444800000}`
	if err := os.WriteFile(filepath.Join(agDir, "oauth_creds.json"), []byte(oauth), 0o600); err != nil {
		t.Fatalf("write oauth: %v", err)
	}

	creds := LoadCLICredentials(
		WithHomeDir(func() (string, error) { return tmp, nil }),
		WithEnv(func(string) (string, bool) { return "", false }),
	)

	if creds.Antigravity.APIKey != "ag-access" {
		t.Fatalf("expected antigravity api key, got %q", creds.Antigravity.APIKey)
	}
}

func TestLoadCLICredentialsRefreshesAntigravityOAuth(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse query: %v", err)
		}
		if values.Get("grant_type") != "refresh_token" {
			t.Fatalf("expected refresh_token grant, got %q", values.Get("grant_type"))
		}
		if values.Get("refresh_token") != "ag-refresh" {
			t.Fatalf("expected refresh_token, got %q", values.Get("refresh_token"))
		}
		if values.Get("client_id") != "ag-client" || values.Get("client_secret") != "ag-secret" {
			t.Fatalf("unexpected client credentials")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"ag-new","expires_in":3600,"refresh_token":"ag-new-refresh","token_type":"Bearer"}`))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	geminiDir := filepath.Join(tmp, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	expired := time.Now().Add(-time.Hour).UnixMilli()
	oauth := fmt.Sprintf(`{"access_token":"ag-old","refresh_token":"ag-refresh","expiry_date":%d,"token_uri":"%s","client_id":"ag-client","client_secret":"ag-secret"}`, expired, srv.URL)
	oauthPath := filepath.Join(geminiDir, "oauth_creds.json")
	if err := os.WriteFile(oauthPath, []byte(oauth), 0o600); err != nil {
		t.Fatalf("write oauth: %v", err)
	}

	creds := LoadCLICredentials(
		WithHomeDir(func() (string, error) { return tmp, nil }),
		WithEnv(func(string) (string, bool) { return "", false }),
	)

	if creds.Antigravity.APIKey != "ag-new" {
		t.Fatalf("expected refreshed token, got %q", creds.Antigravity.APIKey)
	}

	updated, err := os.ReadFile(oauthPath)
	if err != nil {
		t.Fatalf("read oauth: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(updated, &payload); err != nil {
		t.Fatalf("unmarshal oauth: %v", err)
	}
	if payload["access_token"] != "ag-new" {
		t.Fatalf("expected updated access_token, got %v", payload["access_token"])
	}
	if payload["refresh_token"] != "ag-new-refresh" {
		t.Fatalf("expected updated refresh_token, got %v", payload["refresh_token"])
	}
	if expiry, ok := payload["expiry_date"].(float64); !ok || int64(expiry) <= time.Now().UnixMilli() {
		t.Fatalf("expected updated expiry_date, got %v", payload["expiry_date"])
	}
}

func TestLoadCLICredentialsFallsBackToExpiredAntigravityToken(t *testing.T) {
	t.Parallel()
	// Simulate a refresh failure by pointing token_uri at a server that returns 400.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	geminiDir := filepath.Join(tmp, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	expired := time.Now().Add(-24 * time.Hour).UnixMilli()
	oauth := fmt.Sprintf(`{"access_token":"ag-expired","refresh_token":"ag-refresh","expiry_date":%d,"token_uri":"%s","client_id":"c","client_secret":"s"}`, expired, srv.URL)
	oauthPath := filepath.Join(geminiDir, "oauth_creds.json")
	if err := os.WriteFile(oauthPath, []byte(oauth), 0o600); err != nil {
		t.Fatalf("write oauth: %v", err)
	}

	creds := LoadCLICredentials(
		WithHomeDir(func() (string, error) { return tmp, nil }),
		WithEnv(func(string) (string, bool) { return "", false }),
	)

	// The expired token should still be returned so the catalog can surface
	// the provider with an auth error instead of silently hiding it.
	if creds.Antigravity.APIKey != "ag-expired" {
		t.Fatalf("expected expired token to be returned, got %q", creds.Antigravity.APIKey)
	}
	if creds.Antigravity.Provider != "antigravity" {
		t.Fatalf("expected antigravity provider, got %q", creds.Antigravity.Provider)
	}
}
