package config

import (
	"encoding/base64"
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

// buildTestJWT creates a minimal JWT with the given client_id and exp claims.
// The signature is fake — only the payload is meaningful for our tests.
func buildTestJWT(clientID string, exp int64) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload := fmt.Sprintf(`{"client_id":"%s","exp":%d}`, clientID, exp)
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString([]byte("fake-sig"))
	return header + "." + payloadB64 + "." + sig
}

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
	if creds.Antigravity.Source != SourceAntigravityIDE {
		t.Fatalf("expected antigravity_ide source for ~/.gemini/ credential, got %q", creds.Antigravity.Source)
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

func TestLoadCLICredentialsUsesGeminiClientForGeminiPath(t *testing.T) {
	t.Parallel()
	// When the credential is at ~/.gemini/ without embedded client_id/secret,
	// the env-configured OAuth client should be injected for refresh.
	const wantClientID = "test-gemini-client-id.apps.googleusercontent.com"
	const wantClientSecret = "test-gemini-client-secret"

	var gotClientID, gotClientSecret string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		gotClientID = values.Get("client_id")
		gotClientSecret = values.Get("client_secret")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"refreshed","expires_in":3600,"token_type":"Bearer"}`))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	geminiDir := filepath.Join(tmp, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	expired := time.Now().Add(-time.Hour).UnixMilli()
	// No client_id or client_secret in the file — simulates real Gemini CLI output.
	oauth := fmt.Sprintf(`{"access_token":"old","refresh_token":"rt","expiry_date":%d,"token_uri":"%s"}`, expired, srv.URL)
	if err := os.WriteFile(filepath.Join(geminiDir, "oauth_creds.json"), []byte(oauth), 0o600); err != nil {
		t.Fatalf("write oauth: %v", err)
	}

	envLookup := func(key string) (string, bool) {
		switch key {
		case "GEMINI_OAUTH_CLIENT_ID":
			return wantClientID, true
		case "GEMINI_OAUTH_CLIENT_SECRET":
			return wantClientSecret, true
		default:
			return "", false
		}
	}

	creds := LoadCLICredentials(
		WithHomeDir(func() (string, error) { return tmp, nil }),
		WithEnv(envLookup),
	)

	if creds.Antigravity.APIKey != "refreshed" {
		t.Fatalf("expected refreshed token, got %q", creds.Antigravity.APIKey)
	}
	if creds.Antigravity.Source != SourceAntigravityIDE {
		t.Fatalf("expected antigravity_ide source, got %q", creds.Antigravity.Source)
	}
	// Verify the env-configured client credentials were used.
	if gotClientID != wantClientID {
		t.Fatalf("expected env client_id %q, got %q", wantClientID, gotClientID)
	}
	if gotClientSecret != wantClientSecret {
		t.Fatalf("expected env client_secret %q, got %q", wantClientSecret, gotClientSecret)
	}
}

func TestLoadCLICredentialsFallsBackWhenNoOAuthClientConfigured(t *testing.T) {
	t.Parallel()
	// When the credential file has no embedded client_id/secret and no env
	// vars are set, refresh should fail and the expired token should still
	// be returned so the catalog can show the provider with an auth error.
	tmp := t.TempDir()
	geminiDir := filepath.Join(tmp, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	expired := time.Now().Add(-time.Hour).UnixMilli()
	oauth := fmt.Sprintf(`{"access_token":"stale","refresh_token":"rt","expiry_date":%d}`, expired)
	if err := os.WriteFile(filepath.Join(geminiDir, "oauth_creds.json"), []byte(oauth), 0o600); err != nil {
		t.Fatalf("write oauth: %v", err)
	}

	creds := LoadCLICredentials(
		WithHomeDir(func() (string, error) { return tmp, nil }),
		WithEnv(func(string) (string, bool) { return "", false }),
	)

	if creds.Antigravity.APIKey != "stale" {
		t.Fatalf("expected stale expired token, got %q", creds.Antigravity.APIKey)
	}
	if creds.Antigravity.Source != SourceAntigravityIDE {
		t.Fatalf("expected antigravity_ide source, got %q", creds.Antigravity.Source)
	}
}

func TestParseJWTExpiry(t *testing.T) {
	t.Parallel()
	wantExp := time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC).Unix()
	jwt := buildTestJWT("app_test", wantExp)

	got, ok := parseJWTExpiry(jwt)
	if !ok {
		t.Fatal("expected parseJWTExpiry to succeed")
	}
	if got.Unix() != wantExp {
		t.Fatalf("expected exp %d, got %d", wantExp, got.Unix())
	}

	// Invalid inputs.
	if _, ok := parseJWTExpiry("not-a-jwt"); ok {
		t.Fatal("expected failure for invalid JWT")
	}
	if _, ok := parseJWTExpiry("a.!!!.c"); ok {
		t.Fatal("expected failure for bad base64")
	}
}

func TestParseJWTClientID(t *testing.T) {
	t.Parallel()
	jwt := buildTestJWT("app_EMoamEEZ73f0CkXaXp7hrann", time.Now().Add(time.Hour).Unix())

	got := parseJWTClientID(jwt)
	if got != "app_EMoamEEZ73f0CkXaXp7hrann" {
		t.Fatalf("expected client_id, got %q", got)
	}
}

func TestCodexOAuthNeedsRefresh(t *testing.T) {
	t.Parallel()
	now := time.Now()

	// Token valid for 1 hour — no refresh needed.
	validJWT := buildTestJWT("app_test", now.Add(time.Hour).Unix())
	needsRefresh, expired := codexOAuthNeedsRefresh(validJWT, now)
	if needsRefresh || expired {
		t.Fatal("valid token should not need refresh")
	}

	// Token expiring within 5 minutes — needs refresh but not expired.
	soonJWT := buildTestJWT("app_test", now.Add(3*time.Minute).Unix())
	needsRefresh, expired = codexOAuthNeedsRefresh(soonJWT, now)
	if !needsRefresh {
		t.Fatal("near-expiry token should need refresh")
	}
	if expired {
		t.Fatal("near-expiry token should not be marked expired")
	}

	// Already expired.
	expiredJWT := buildTestJWT("app_test", now.Add(-time.Hour).Unix())
	needsRefresh, expired = codexOAuthNeedsRefresh(expiredJWT, now)
	if !needsRefresh || !expired {
		t.Fatal("expired token should need refresh and be marked expired")
	}
}

func TestLoadCLICredentialsRefreshesCodexOAuth(t *testing.T) {
	t.Parallel()

	const wantClientID = "app_test_client"
	expiredJWT := buildTestJWT(wantClientID, time.Now().Add(-time.Hour).Unix())
	freshJWT := buildTestJWT(wantClientID, time.Now().Add(10*24*time.Hour).Unix())

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
		if values.Get("refresh_token") != "rt_codex_test" {
			t.Fatalf("unexpected refresh_token: %q", values.Get("refresh_token"))
		}
		if values.Get("client_id") != wantClientID {
			t.Fatalf("expected client_id %q, got %q", wantClientID, values.Get("client_id"))
		}
		w.Header().Set("Content-Type", "application/json")
		resp := fmt.Sprintf(`{"access_token":"%s","refresh_token":"rt_codex_new","id_token":"id_new","expires_in":864000,"token_type":"Bearer"}`, freshJWT)
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	codexDir := filepath.Join(tmp, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	auth := fmt.Sprintf(`{
		"OPENAI_API_KEY": null,
		"tokens": {
			"access_token": "%s",
			"refresh_token": "rt_codex_test",
			"account_id": "acct-123"
		},
		"last_refresh": "2026-02-02T00:00:00Z",
		"token_url": "%s"
	}`, expiredJWT, srv.URL)
	authPath := filepath.Join(codexDir, "auth.json")
	if err := os.WriteFile(authPath, []byte(auth), 0o600); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	creds := LoadCLICredentials(
		WithHomeDir(func() (string, error) { return tmp, nil }),
		WithEnv(func(string) (string, bool) { return "", false }),
	)

	if creds.Codex.APIKey != freshJWT {
		t.Fatalf("expected refreshed token, got %q", creds.Codex.APIKey)
	}
	if creds.Codex.AccountID != "acct-123" {
		t.Fatalf("expected account_id preserved, got %q", creds.Codex.AccountID)
	}

	// Verify the file was updated.
	updated, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatalf("read auth: %v", err)
	}
	var file codexAuthFile
	if err := json.Unmarshal(updated, &file); err != nil {
		t.Fatalf("unmarshal auth: %v", err)
	}
	if file.Tokens.AccessToken != freshJWT {
		t.Fatalf("expected updated access_token in file")
	}
	if file.Tokens.RefreshToken != "rt_codex_new" {
		t.Fatalf("expected updated refresh_token, got %q", file.Tokens.RefreshToken)
	}
	if file.Tokens.IDToken != "id_new" {
		t.Fatalf("expected updated id_token, got %q", file.Tokens.IDToken)
	}
	if file.LastRefresh == "" || file.LastRefresh == "2026-02-02T00:00:00Z" {
		t.Fatalf("expected updated last_refresh, got %q", file.LastRefresh)
	}
}

func TestLoadCLICredentialsFallsBackToExpiredCodexToken(t *testing.T) {
	t.Parallel()
	// Simulate a refresh failure.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()

	expiredJWT := buildTestJWT("app_test", time.Now().Add(-time.Hour).Unix())

	tmp := t.TempDir()
	codexDir := filepath.Join(tmp, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	auth := fmt.Sprintf(`{
		"tokens": {
			"access_token": "%s",
			"refresh_token": "rt_dead",
			"account_id": "acct"
		},
		"token_url": "%s"
	}`, expiredJWT, srv.URL)
	if err := os.WriteFile(filepath.Join(codexDir, "auth.json"), []byte(auth), 0o600); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	creds := LoadCLICredentials(
		WithHomeDir(func() (string, error) { return tmp, nil }),
		WithEnv(func(string) (string, bool) { return "", false }),
	)

	// Expired token should still be returned (not hidden).
	if creds.Codex.APIKey != expiredJWT {
		t.Fatalf("expected expired token to be returned, got %q", creds.Codex.APIKey)
	}
	if creds.Codex.Provider != "codex" {
		t.Fatalf("expected codex provider, got %q", creds.Codex.Provider)
	}
}

func TestLoadCLICredentialsSkipsRefreshForValidCodexToken(t *testing.T) {
	t.Parallel()
	// Server should NOT be called when token is still valid.
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	validJWT := buildTestJWT("app_test", time.Now().Add(2*time.Hour).Unix())

	tmp := t.TempDir()
	codexDir := filepath.Join(tmp, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	auth := fmt.Sprintf(`{
		"tokens": {
			"access_token": "%s",
			"refresh_token": "rt_test",
			"account_id": "acct"
		},
		"token_url": "%s"
	}`, validJWT, srv.URL)
	if err := os.WriteFile(filepath.Join(codexDir, "auth.json"), []byte(auth), 0o600); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	creds := LoadCLICredentials(
		WithHomeDir(func() (string, error) { return tmp, nil }),
		WithEnv(func(string) (string, bool) { return "", false }),
	)

	if creds.Codex.APIKey != validJWT {
		t.Fatalf("expected valid token, got %q", creds.Codex.APIKey)
	}
	if called {
		t.Fatal("refresh endpoint should not be called for valid token")
	}
}
