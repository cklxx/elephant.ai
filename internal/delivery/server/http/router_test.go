package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"alex/internal/attachments"
	serverapp "alex/internal/server/app"
)

func TestRouterRegistersAuthEndpointsWhenDisabled(t *testing.T) {
	router := NewRouter(
		RouterDeps{
			Broadcaster:   serverapp.NewEventBroadcaster(),
			HealthChecker: serverapp.NewHealthChecker(),
			AttachmentCfg: attachments.StoreConfig{Dir: t.TempDir()},
		},
		RouterConfig{Environment: "development"},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"email":"a"}`))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}

	if !strings.Contains(w.Body.String(), "Authentication module not configured") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestRouterRegistersDataCacheEndpoint(t *testing.T) {
	cache := NewDataCache(4, time.Minute)
	url := cache.StoreBytes("text/plain", []byte("hello"))

	router := NewRouter(
		RouterDeps{
			Broadcaster:   serverapp.NewEventBroadcaster(),
			HealthChecker: serverapp.NewHealthChecker(),
			AttachmentCfg: attachments.StoreConfig{Dir: t.TempDir()},
			DataCache:     cache,
		},
		RouterConfig{Environment: "development"},
	)

	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	if strings.TrimSpace(w.Body.String()) != "hello" {
		t.Fatalf("expected cached body, got %q", w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain" {
		t.Fatalf("expected content-type text/plain, got %q", ct)
	}
}
