# Antigravity OAuth Refresh Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ensure antigravity CLI auth returns a fresh access token by refreshing expired oauth_creds.json tokens (supporting both `~/.antigravity` and `~/.gemini`) so model list and LLM calls stop failing with 401.

**Architecture:** Extend CLI auth loading in `internal/config/cli_auth.go` to parse oauth_creds.json, detect expiration (`expiry_date`, `expire`, `timestamp + expires_in`), refresh via the OAuth token endpoint (`token_uri` if present, else Google default with antigravity CLI client creds), update the credential file in-place, and return the refreshed token. Keep existing opencode auth.json fallback.

**Tech Stack:** Go stdlib (`encoding/json`, `net/http`, `net/url`, `time`), `httptest`, current config loader.

### Task 1: Add failing tests for antigravity OAuth refresh and path support

**Files:**
- Modify: `internal/config/cli_auth_test.go`

**Step 1: Write failing test for refresh**

```go
func TestLoadCLICredentialsRefreshesAntigravityOAuth(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
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
	if err := os.WriteFile(filepath.Join(geminiDir, "oauth_creds.json"), []byte(oauth), 0o600); err != nil {
		t.Fatalf("write oauth: %v", err)
	}

	creds := LoadCLICredentials(
		WithHomeDir(func() (string, error) { return tmp, nil }),
		WithEnv(func(string) (string, bool) { return "", false }),
	)

	if creds.Antigravity.APIKey != "ag-new" {
		t.Fatalf("expected refreshed token, got %q", creds.Antigravity.APIKey)
	}
}
```

**Step 2: Update existing gemini oauth test to avoid refresh**

```go
future := time.Now().Add(2 * time.Hour).UnixMilli()
oauth := fmt.Sprintf(`{"access_token":"ag-access","refresh_token":"ag-refresh","expiry_date":%d}`, future)
```

**Step 3: Add failing test for `~/.antigravity/oauth_creds.json` path**

```go
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
```

**Step 4: Run tests to confirm failure**

Run: `go test ./internal/config -run TestLoadCLICredentialsRefreshesAntigravityOAuth -v`
Expected: FAIL (no refresh logic yet).

### Task 2: Implement antigravity OAuth refresh + path support

**Files:**
- Modify: `internal/config/cli_auth.go`

**Step 1: Add OAuth constants and helpers** (include comment citing proxycast `antigravity.rs` and Google token endpoint)

```go
const (
	antigravityOAuthTokenURL     = "https://oauth2.googleapis.com/token"
	antigravityOAuthClientID     = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
	antigravityOAuthClientSecret = "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"
)

const antigravityOAuthRefreshSkew = 5 * time.Minute
```

**Step 2: Add path helper**

```go
func antigravityOAuthPaths(home string) []string {
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".antigravity", "oauth_creds.json"),
		filepath.Join(home, ".gemini", "oauth_creds.json"),
	}
}
```

**Step 3: Implement expiry detection + refresh**

```go
func antigravityOAuthExpiry(payload geminiOAuthFile) (time.Time, bool) { ... }
func antigravityOAuthNeedsRefresh(payload geminiOAuthFile, now time.Time) bool { ... }
func refreshAntigravityOAuth(ctx context.Context, payload geminiOAuthFile) (antigravityOAuthTokenResponse, error) { ... }
```

**Step 4: Update loadAntigravityGeminiOAuth to**
- iterate `antigravityOAuthPaths`
- parse payload (single object or array)
- refresh when expired/expiring soon and refresh_token present
- update oauth_creds.json with new access_token/expiry_date/expires_in/timestamp/refresh_token (when provided)
- return refreshed token in `CLICredential`

**Step 5: Run tests**

Run: `go test ./internal/config -run TestLoadCLICredentialsRefreshesAntigravityOAuth -v`
Expected: PASS.

### Task 3: Full validation and commit

**Step 1: Run full Go tests**
Run: `go test ./...`

**Step 2: Run web lint + tests**
Run: `cd web && npm run lint`
Run: `cd web && npm test`

**Step 3: Commit**
```bash
git add internal/config/cli_auth.go internal/config/cli_auth_test.go
git commit -m "fix: refresh antigravity oauth tokens from cli creds"
```
