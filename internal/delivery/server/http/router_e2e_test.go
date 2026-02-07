package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	authapp "alex/internal/app/auth"
	serverapp "alex/internal/delivery/server/app"
	"alex/internal/infra/attachments"
	"alex/internal/infra/auth/adapters"
)

func TestRouterE2EGoogleLoginRouteWhenAuthEnabled(t *testing.T) {
	authHandler, authService, _ := newE2EAuth(t)
	router := NewRouter(
		RouterDeps{
			Broadcaster:   serverapp.NewEventBroadcaster(),
			HealthChecker: serverapp.NewHealthChecker(),
			AttachmentCfg: attachments.StoreConfig{Dir: t.TempDir()},
			AuthHandler:   authHandler,
			AuthService:   authService,
		},
		RouterConfig{Environment: "development"},
	)

	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := server.Client().Get(server.URL + "/api/auth/google/login")
	if err != nil {
		t.Fatalf("request google login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		t.Fatalf("expected auth module enabled (non-503), got %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when oauth provider is not configured, got %d", resp.StatusCode)
	}
}

func TestRouterE2EDevLogIndexWithAuth(t *testing.T) {
	logDir := t.TempDir()
	requestDir := filepath.Join(t.TempDir(), "requests")
	if err := os.MkdirAll(requestDir, 0o755); err != nil {
		t.Fatalf("mkdir request dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(logDir, "alex-service.log"),
		[]byte("2026-02-08 09:00:00 [INFO] [SERVICE] [API] [log_id=log-e2e] ready\n"),
		0o644,
	); err != nil {
		t.Fatalf("write service log: %v", err)
	}
	t.Setenv("ALEX_LOG_DIR", logDir)
	t.Setenv("ALEX_REQUEST_LOG_DIR", requestDir)

	authHandler, authService, accessToken := newE2EAuth(t)
	router := NewRouter(
		RouterDeps{
			Broadcaster:   serverapp.NewEventBroadcaster(),
			HealthChecker: serverapp.NewHealthChecker(),
			AttachmentCfg: attachments.StoreConfig{Dir: t.TempDir()},
			AuthHandler:   authHandler,
			AuthService:   authService,
		},
		RouterConfig{Environment: "development"},
	)

	server := httptest.NewServer(router)
	defer server.Close()

	unauthorizedResp, err := server.Client().Get(server.URL + "/api/dev/logs/index?limit=5")
	if err != nil {
		t.Fatalf("request dev logs index (unauthorized): %v", err)
	}
	unauthorizedResp.Body.Close()
	if unauthorizedResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", unauthorizedResp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/dev/logs/index?limit=5", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("request dev logs index with auth: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var payload struct {
		Entries []struct {
			LogID string `json:"log_id"`
		} `json:"entries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Entries) == 0 {
		t.Fatalf("expected at least one log entry")
	}
	if strings.TrimSpace(payload.Entries[0].LogID) == "" {
		t.Fatalf("expected non-empty log_id in response")
	}
}

func newE2EAuth(t *testing.T) (*AuthHandler, *authapp.Service, string) {
	t.Helper()

	users, identities, sessions, states := adapters.NewMemoryStores()
	tokenManager := adapters.NewJWTTokenManager("e2e-secret", "e2e", 15*time.Minute)
	sessions.SetVerifier(func(plain, encoded string) (bool, error) {
		return tokenManager.VerifyRefreshToken(plain, encoded)
	})
	service := authapp.NewService(users, identities, sessions, tokenManager, states, nil, authapp.Config{})

	ctx := context.Background()
	if _, err := service.RegisterLocal(ctx, "e2e@example.com", "password", "E2E"); err != nil {
		t.Fatalf("register e2e user: %v", err)
	}
	tokens, err := service.LoginWithPassword(ctx, "e2e@example.com", "password", "e2e-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("login e2e user: %v", err)
	}

	return NewAuthHandler(service, false), service, tokens.AccessToken
}
