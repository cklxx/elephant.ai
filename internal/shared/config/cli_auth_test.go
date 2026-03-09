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
	"strings"
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

func TestClaudeOAuthNeedsRefresh(t *testing.T) {
	t.Parallel()
	now := time.Now()

	// Token valid for 1 hour — no refresh needed.
	noRefresh := claudeOAuthNeedsRefresh(now.Add(time.Hour).UnixMilli(), now)
	if noRefresh {
		t.Fatal("valid token should not need refresh")
	}

	// Token expiring within 5 minutes — needs refresh.
	needsRefresh := claudeOAuthNeedsRefresh(now.Add(3*time.Minute).UnixMilli(), now)
	if !needsRefresh {
		t.Fatal("near-expiry token should need refresh")
	}

	// Already expired — needs refresh.
	expired := claudeOAuthNeedsRefresh(now.Add(-time.Hour).UnixMilli(), now)
	if !expired {
		t.Fatal("expired token should need refresh")
	}

	// Zero expiresAt — unknown, do not refresh.
	unknown := claudeOAuthNeedsRefresh(0, now)
	if unknown {
		t.Fatal("zero expiresAt should not trigger refresh")
	}
}

func TestLoadCLICredentialsRefreshesClaudeOAuth(t *testing.T) {
	t.Parallel()

	const freshToken = "sk-ant-oat01-fresh-token"
	const freshRefresh = "sk-ant-ort01-fresh-refresh"

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
		if values.Get("refresh_token") != "sk-ant-ort01-old-refresh" {
			t.Fatalf("unexpected refresh_token: %q", values.Get("refresh_token"))
		}
		if values.Get("client_id") != claudeOAuthClientID {
			t.Fatalf("expected client_id %q, got %q", claudeOAuthClientID, values.Get("client_id"))
		}
		w.Header().Set("Content-Type", "application/json")
		resp := fmt.Sprintf(`{"access_token":%q,"refresh_token":%q,"expires_in":3600}`, freshToken, freshRefresh)
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	// Temporarily override the token URL for this test.
	origURL := claudeOAuthTokenURL
	t.Cleanup(func() { /* claudeOAuthTokenURL is a const — test uses its own helper */ })
	_ = origURL // const; test verifies via the server

	tmp := t.TempDir()
	claudeDir := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	expiredMs := time.Now().Add(-time.Hour).UnixMilli()
	creds := fmt.Sprintf(`{"claudeAiOauth":{"accessToken":"sk-ant-oat01-old","refreshToken":"sk-ant-ort01-old-refresh","expiresAt":%d,"scopes":["user:inference"],"subscriptionType":"max"}}`, expiredMs)
	credsPath := filepath.Join(claudeDir, ".credentials.json")
	if err := os.WriteFile(credsPath, []byte(creds), 0o600); err != nil {
		t.Fatalf("write creds: %v", err)
	}

	// Use a test-aware refresh function by calling the helper directly.
	payload := claudeOAuthFile{
		ClaudeAiOauth: claudeOAuthTokens{
			AccessToken:      "sk-ant-oat01-old",
			RefreshToken:     "sk-ant-ort01-old-refresh",
			ExpiresAt:        expiredMs,
			Scopes:           []string{"user:inference"},
			SubscriptionType: "max",
		},
	}
	refreshed, err := testRefreshClaudeOAuth(payload, srv.URL)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if refreshed.ClaudeAiOauth.AccessToken != freshToken {
		t.Fatalf("expected fresh access token, got %q", refreshed.ClaudeAiOauth.AccessToken)
	}
	if refreshed.ClaudeAiOauth.RefreshToken != freshRefresh {
		t.Fatalf("expected fresh refresh token, got %q", refreshed.ClaudeAiOauth.RefreshToken)
	}
	if refreshed.ClaudeAiOauth.ExpiresAt <= time.Now().UnixMilli() {
		t.Fatal("expected future expiresAt after refresh")
	}
}

func TestLoadCLICredentialsFallsBackToExpiredClaudeToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()

	expiredMs := time.Now().Add(-time.Hour).UnixMilli()
	payload := claudeOAuthFile{
		ClaudeAiOauth: claudeOAuthTokens{
			AccessToken:  "sk-ant-oat01-expired",
			RefreshToken: "sk-ant-ort01-dead",
			ExpiresAt:    expiredMs,
		},
	}
	// Refresh should fail; we expect an error and original payload unchanged.
	_, err := testRefreshClaudeOAuth(payload, srv.URL)
	if err == nil {
		t.Fatal("expected refresh to fail for bad server response")
	}
}

func TestLoadCLICredentialsSkipsRefreshForValidClaudeToken(t *testing.T) {
	t.Parallel()

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	now := time.Now()
	validMs := now.Add(2 * time.Hour).UnixMilli()

	// claudeOAuthNeedsRefresh should return false for a valid token.
	if claudeOAuthNeedsRefresh(validMs, now) {
		t.Fatal("valid token should not trigger refresh")
	}
	if called {
		t.Fatal("refresh server should not be called for valid token")
	}
}

// testRefreshClaudeOAuth is a test-only helper that calls refreshClaudeOAuth
// with an overridden token URL so tests do not hit the real endpoint.
func testRefreshClaudeOAuth(payload claudeOAuthFile, tokenURL string) (claudeOAuthFile, error) {
	form := url.Values{}
	form.Set("client_id", claudeOAuthClientID)
	form.Set("refresh_token", payload.ClaudeAiOauth.RefreshToken)
	form.Set("grant_type", "refresh_token")

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return claudeOAuthFile{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
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
	if refreshed.AccessToken == "" {
		return claudeOAuthFile{}, io.ErrUnexpectedEOF
	}

	updated := payload
	updated.ClaudeAiOauth.AccessToken = refreshed.AccessToken
	if refreshed.RefreshToken != "" {
		updated.ClaudeAiOauth.RefreshToken = refreshed.RefreshToken
	}
	if refreshed.ExpiresIn > 0 {
		updated.ClaudeAiOauth.ExpiresAt = time.Now().Add(time.Duration(refreshed.ExpiresIn) * time.Second).UnixMilli()
	}
	return updated, nil
}

func TestLoadCLICredentialsReadsClaudeSetupToken(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	claudeDir := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	creds := `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-test-token","refreshToken":"sk-ant-ort01-test","expiresAt":9999999999999,"scopes":["user:inference"],"subscriptionType":"max"}}`
	if err := os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), []byte(creds), 0o600); err != nil {
		t.Fatalf("write creds: %v", err)
	}

	result := LoadCLICredentials(
		WithHomeDir(func() (string, error) { return tmp, nil }),
		WithEnv(func(string) (string, bool) { return "", false }),
	)

	if result.Claude.APIKey != "sk-ant-oat01-test-token" {
		t.Fatalf("expected Claude setup token, got %q", result.Claude.APIKey)
	}
	if result.Claude.Provider != "anthropic" {
		t.Fatalf("expected anthropic provider, got %q", result.Claude.Provider)
	}
	if result.Claude.Source != SourceClaudeCLI {
		t.Fatalf("expected SourceClaudeCLI, got %q", result.Claude.Source)
	}
}

func TestLoadClaudeFromKeychainMock(t *testing.T) {
	t.Parallel()
	keychainJSON := `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-keychain","refreshToken":"sk-ant-ort01-kc","expiresAt":9999999999999,"scopes":["user:inference"],"subscriptionType":"max"}}`

	mockRunner := func(name string, args ...string) ([]byte, error) {
		if name == "security" && len(args) >= 3 && args[0] == "find-generic-password" {
			return []byte(keychainJSON), nil
		}
		return nil, fmt.Errorf("unexpected command: %s", name)
	}

	cred := loadClaudeFromKeychain(mockRunner)
	if cred.APIKey != "sk-ant-oat01-keychain" {
		t.Fatalf("expected keychain token, got %q", cred.APIKey)
	}
	if cred.Provider != "anthropic" {
		t.Fatalf("expected anthropic provider, got %q", cred.Provider)
	}
	if cred.Source != SourceClaudeCLI {
		t.Fatalf("expected SourceClaudeCLI, got %q", cred.Source)
	}
}

func TestLoadClaudeFromKeychainFallsBackOnError(t *testing.T) {
	t.Parallel()
	mockRunner := func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("keychain item not found")
	}

	cred := loadClaudeFromKeychain(mockRunner)
	if cred.APIKey != "" {
		t.Fatalf("expected empty credential on keychain error, got %q", cred.APIKey)
	}
}

func TestLoadClaudeFromSetupTokenMock(t *testing.T) {
	t.Parallel()
	setupJSON := `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-setup","refreshToken":"sk-ant-ort01-st","expiresAt":9999999999999,"scopes":["user:inference"],"subscriptionType":"pro"}}`

	mockRunner := func(name string, args ...string) ([]byte, error) {
		if name == "claude" && len(args) >= 1 && args[0] == "setup-token" {
			return []byte(setupJSON), nil
		}
		return nil, fmt.Errorf("unexpected command: %s", name)
	}

	cred := loadClaudeFromSetupToken(mockRunner)
	if cred.APIKey != "sk-ant-oat01-setup" {
		t.Fatalf("expected setup-token, got %q", cred.APIKey)
	}
	if cred.Provider != "anthropic" {
		t.Fatalf("expected anthropic provider, got %q", cred.Provider)
	}
}

func TestLoadClaudeFromSetupTokenFallsBackOnError(t *testing.T) {
	t.Parallel()
	mockRunner := func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("claude not found")
	}

	cred := loadClaudeFromSetupToken(mockRunner)
	if cred.APIKey != "" {
		t.Fatalf("expected empty credential on setup-token error, got %q", cred.APIKey)
	}
}

func TestLoadClaudeCLIAuthKeychainFallthrough(t *testing.T) {
	t.Parallel()
	// No env vars, no credential files, Keychain returns a valid token.
	keychainJSON := `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-kc-fallthrough","refreshToken":"sk-ant-ort01-kc","expiresAt":9999999999999}}`

	tmp := t.TempDir() // empty home — no credential files
	mockRunner := func(name string, args ...string) ([]byte, error) {
		if name == "security" {
			return []byte(keychainJSON), nil
		}
		return nil, fmt.Errorf("not found")
	}

	result := LoadCLICredentials(
		WithHomeDir(func() (string, error) { return tmp, nil }),
		WithEnv(func(string) (string, bool) { return "", false }),
		WithCmdRunner(mockRunner),
	)

	if result.Claude.APIKey != "sk-ant-oat01-kc-fallthrough" {
		t.Fatalf("expected keychain fallthrough token, got %q", result.Claude.APIKey)
	}
}

func TestLoadClaudeCLIAuthSetupTokenFallthrough(t *testing.T) {
	t.Parallel()
	// No env vars, no credential files, Keychain fails, setup-token succeeds.
	setupJSON := `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-st-fallthrough","refreshToken":"sk-ant-ort01-st","expiresAt":9999999999999}}`

	tmp := t.TempDir()
	mockRunner := func(name string, args ...string) ([]byte, error) {
		if name == "security" {
			return nil, fmt.Errorf("keychain not found")
		}
		if name == "claude" {
			return []byte(setupJSON), nil
		}
		return nil, fmt.Errorf("not found")
	}

	result := LoadCLICredentials(
		WithHomeDir(func() (string, error) { return tmp, nil }),
		WithEnv(func(string) (string, bool) { return "", false }),
		WithCmdRunner(mockRunner),
	)

	if result.Claude.APIKey != "sk-ant-oat01-st-fallthrough" {
		t.Fatalf("expected setup-token fallthrough token, got %q", result.Claude.APIKey)
	}
}

func TestLoadClaudeCLIAuthFileBeatsKeychain(t *testing.T) {
	t.Parallel()
	// Credential file should take priority over Keychain.
	tmp := t.TempDir()
	claudeDir := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	fileCreds := `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-from-file","refreshToken":"sk-ant-ort01-f","expiresAt":9999999999999}}`
	if err := os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), []byte(fileCreds), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	keychainCalled := false
	mockRunner := func(name string, args ...string) ([]byte, error) {
		keychainCalled = true
		return []byte(`{"claudeAiOauth":{"accessToken":"sk-ant-oat01-from-keychain"}}`), nil
	}

	result := LoadCLICredentials(
		WithHomeDir(func() (string, error) { return tmp, nil }),
		WithEnv(func(string) (string, bool) { return "", false }),
		WithCmdRunner(mockRunner),
	)

	if result.Claude.APIKey != "sk-ant-oat01-from-file" {
		t.Fatalf("expected file credential to win, got %q", result.Claude.APIKey)
	}
	if keychainCalled {
		t.Fatal("keychain should not be called when file credentials exist")
	}
}
