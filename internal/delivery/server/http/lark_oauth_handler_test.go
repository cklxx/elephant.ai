package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	larkoauth "alex/internal/infra/lark/oauth"
	"alex/internal/shared/logging"
)

func TestLarkOAuthHandler_StartRedirects(t *testing.T) {
	store, err := larkoauth.NewFileTokenStore(t.TempDir())
	if err != nil {
		t.Fatalf("token store: %v", err)
	}
	svc, err := larkoauth.NewService(larkoauth.ServiceConfig{
		AppID:        "app",
		AppSecret:    "secret",
		BaseDomain:   "https://open.feishu.cn",
		RedirectBase: "http://localhost:8080",
	}, store, larkoauth.NewMemoryStateStore())
	if err != nil {
		t.Fatalf("service: %v", err)
	}

	handler := NewLarkOAuthHandler(svc, logging.Nop())
	req := httptest.NewRequest(http.MethodGet, "/api/lark/oauth/start", nil)
	rec := httptest.NewRecorder()
	handler.HandleStart(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusFound)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/open-apis/authen/v1/index") {
		t.Fatalf("unexpected redirect location: %s", loc)
	}
	if !strings.Contains(loc, "app_id=app") {
		t.Fatalf("missing app_id in redirect: %s", loc)
	}
}

func TestLarkOAuthHandler_CallbackSuccess(t *testing.T) {
	tokenStore, err := larkoauth.NewFileTokenStore(t.TempDir())
	if err != nil {
		t.Fatalf("token store: %v", err)
	}
	stateStore := larkoauth.NewMemoryStateStore()

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

	svc, err := larkoauth.NewService(larkoauth.ServiceConfig{
		AppID:        "app",
		AppSecret:    "secret",
		BaseDomain:   server.URL,
		RedirectBase: "http://localhost:8080",
	}, tokenStore, stateStore)
	if err != nil {
		t.Fatalf("service: %v", err)
	}
	state, _, err := svc.StartAuth(httptest.NewRequest(http.MethodGet, "/", nil).Context())
	if err != nil {
		t.Fatalf("StartAuth: %v", err)
	}

	handler := NewLarkOAuthHandler(svc, logging.Nop())
	req := httptest.NewRequest(http.MethodGet, "/api/lark/oauth/callback?code=code-123&state="+state, nil)
	rec := httptest.NewRecorder()
	handler.HandleCallback(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "Authorization complete") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
	token, err := tokenStore.Get(req.Context(), "ou_123")
	if err != nil {
		t.Fatalf("token store get: %v", err)
	}
	if token.AccessToken != "u-token" {
		t.Fatalf("access_token=%q, want %q", token.AccessToken, "u-token")
	}
}

func TestLarkOAuthHandler_CallbackInvalidState(t *testing.T) {
	store, err := larkoauth.NewFileTokenStore(t.TempDir())
	if err != nil {
		t.Fatalf("token store: %v", err)
	}
	svc, err := larkoauth.NewService(larkoauth.ServiceConfig{
		AppID:        "app",
		AppSecret:    "secret",
		BaseDomain:   "https://open.feishu.cn",
		RedirectBase: "http://localhost:8080",
	}, store, larkoauth.NewMemoryStateStore())
	if err != nil {
		t.Fatalf("service: %v", err)
	}

	handler := NewLarkOAuthHandler(svc, logging.Nop())
	req := httptest.NewRequest(http.MethodGet, "/api/lark/oauth/callback?code=code-123&state=missing", nil)
	rec := httptest.NewRecorder()
	handler.HandleCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusBadRequest)
	}
}
