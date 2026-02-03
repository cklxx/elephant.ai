package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type memoryTokenStore struct {
	mu     sync.Mutex
	tokens map[string]Token
}

func newMemoryTokenStore() *memoryTokenStore {
	return &memoryTokenStore{tokens: make(map[string]Token)}
}

func (s *memoryTokenStore) EnsureSchema(context.Context) error { return nil }

func (s *memoryTokenStore) Get(ctx context.Context, openID string) (Token, error) {
	if err := ctx.Err(); err != nil {
		return Token{}, err
	}
	openID = strings.TrimSpace(openID)
	if openID == "" {
		return Token{}, ErrTokenNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	token, ok := s.tokens[openID]
	if !ok {
		return Token{}, ErrTokenNotFound
	}
	return token, nil
}

func (s *memoryTokenStore) Upsert(ctx context.Context, token Token) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if token.OpenID == "" {
		return ErrTokenNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token.OpenID] = token
	return nil
}

func (s *memoryTokenStore) Delete(ctx context.Context, openID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, openID)
	return nil
}

type missingTokenStore struct{}

func (s missingTokenStore) EnsureSchema(context.Context) error { return nil }
func (s missingTokenStore) Get(context.Context, string) (Token, error) {
	return Token{}, ErrTokenNotFound
}
func (s missingTokenStore) Upsert(context.Context, Token) error  { return nil }
func (s missingTokenStore) Delete(context.Context, string) error { return nil }

func TestService_StartAuthAndHandleCallback(t *testing.T) {
	t.Helper()
	tokenStore := newMemoryTokenStore()
	stateStore := NewMemoryStateStore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/v3/app_access_token/internal") || strings.Contains(r.URL.Path, "/auth/v3/tenant_access_token/internal") {
			resp := map[string]any{
				"code":                0,
				"msg":                 "ok",
				"app_access_token":    "app-token",
				"tenant_access_token": "app-token",
				"expire":              7200,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/open-apis/authen/v1/access_token") {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		resp := map[string]any{
			"code": 0,
			"msg":  "ok",
			"data": map[string]any{
				"open_id":            "ou_123",
				"access_token":       "u-token",
				"refresh_token":      "r-token",
				"expires_in":         3600,
				"refresh_expires_in": 7200,
				"token_type":         "Bearer",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewService(ServiceConfig{
		AppID:        "app",
		AppSecret:    "secret",
		BaseDomain:   server.URL,
		RedirectBase: "http://localhost:8080",
	}, tokenStore, stateStore)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	state, authURL, err := svc.StartAuth(context.Background())
	if err != nil {
		t.Fatalf("StartAuth: %v", err)
	}
	if state == "" {
		t.Fatal("expected state to be generated")
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}
	if !strings.Contains(parsed.Path, "/open-apis/authen/v1/index") {
		t.Fatalf("unexpected auth url path: %s", parsed.Path)
	}
	if got := parsed.Query().Get("app_id"); got != "app" {
		t.Fatalf("auth url app_id=%q, want %q", got, "app")
	}
	if got := parsed.Query().Get("state"); got != state {
		t.Fatalf("auth url state=%q, want %q", got, state)
	}

	token, err := svc.HandleCallback(context.Background(), "code-123", state)
	if err != nil {
		t.Fatalf("HandleCallback: %v", err)
	}
	if token.OpenID != "ou_123" {
		t.Fatalf("token open_id=%q, want %q", token.OpenID, "ou_123")
	}
	stored, err := tokenStore.Get(context.Background(), "ou_123")
	if err != nil {
		t.Fatalf("token store get: %v", err)
	}
	if stored.AccessToken != "u-token" {
		t.Fatalf("stored access token=%q, want %q", stored.AccessToken, "u-token")
	}
}

func TestService_UserAccessToken_Refresh(t *testing.T) {
	tokenStore := newMemoryTokenStore()
	stateStore := NewMemoryStateStore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/v3/app_access_token/internal") || strings.Contains(r.URL.Path, "/auth/v3/tenant_access_token/internal") {
			resp := map[string]any{
				"code":                0,
				"msg":                 "ok",
				"app_access_token":    "app-token",
				"tenant_access_token": "app-token",
				"expire":              7200,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/open-apis/authen/v1/refresh_access_token") {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		resp := map[string]any{
			"code": 0,
			"msg":  "ok",
			"data": map[string]any{
				"open_id":            "ou_123",
				"access_token":       "u-token-new",
				"refresh_token":      "r-token-new",
				"expires_in":         3600,
				"refresh_expires_in": 7200,
				"token_type":         "Bearer",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc, err := NewService(ServiceConfig{
		AppID:        "app",
		AppSecret:    "secret",
		BaseDomain:   server.URL,
		RedirectBase: "http://localhost:8080",
	}, tokenStore, stateStore)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	now := time.Date(2026, 2, 3, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	if err := tokenStore.Upsert(context.Background(), Token{
		OpenID:       "ou_123",
		AccessToken:  "u-token-old",
		RefreshToken: "r-token-old",
		ExpiresAt:    now.Add(-1 * time.Minute),
		UpdatedAt:    now.Add(-2 * time.Minute),
	}); err != nil {
		t.Fatalf("seed token: %v", err)
	}

	token, err := svc.UserAccessToken(context.Background(), "ou_123")
	if err != nil {
		t.Fatalf("UserAccessToken: %v", err)
	}
	if token != "u-token-new" {
		t.Fatalf("token=%q, want %q", token, "u-token-new")
	}
	stored, err := tokenStore.Get(context.Background(), "ou_123")
	if err != nil {
		t.Fatalf("get token: %v", err)
	}
	if stored.RefreshToken != "r-token-new" {
		t.Fatalf("refresh token=%q, want %q", stored.RefreshToken, "r-token-new")
	}
}

func TestService_UserAccessToken_Missing(t *testing.T) {
	svc, err := NewService(ServiceConfig{
		AppID:        "app",
		AppSecret:    "secret",
		BaseDomain:   "https://open.feishu.cn",
		RedirectBase: "http://localhost:8080",
	}, missingTokenStore{}, NewMemoryStateStore())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = svc.UserAccessToken(context.Background(), "ou_404")
	if err == nil {
		t.Fatal("expected error for missing token")
	}
	var need *NeedUserAuthError
	if !errors.As(err, &need) {
		t.Fatalf("expected NeedUserAuthError, got %v", err)
	}
	if !strings.Contains(need.AuthURL, "/api/lark/oauth/start") {
		t.Fatalf("unexpected auth url: %s", need.AuthURL)
	}
}
