package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	serverapp "alex/internal/delivery/server/app"
	"alex/internal/infra/attachments"
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

func TestRouterRegistersDevLogIndexInDevelopment(t *testing.T) {
	logDir := t.TempDir()
	requestDir := filepath.Join(t.TempDir(), "requests")
	if err := os.MkdirAll(requestDir, 0o755); err != nil {
		t.Fatalf("mkdir request dir: %v", err)
	}
	t.Setenv("ALEX_LOG_DIR", logDir)
	t.Setenv("ALEX_REQUEST_LOG_DIR", requestDir)

	router := NewRouter(
		RouterDeps{
			Broadcaster:   serverapp.NewEventBroadcaster(),
			HealthChecker: serverapp.NewHealthChecker(),
			AttachmentCfg: attachments.StoreConfig{Dir: t.TempDir()},
		},
		RouterConfig{Environment: "development"},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/dev/logs/index?limit=5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var payload struct {
		Entries []any `json:"entries"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Entries == nil {
		t.Fatalf("expected entries field to be present")
	}
}

func TestRouterDoesNotExposeDevLogIndexOutsideDevelopment(t *testing.T) {
	router := NewRouter(
		RouterDeps{
			Broadcaster:   serverapp.NewEventBroadcaster(),
			HealthChecker: serverapp.NewHealthChecker(),
			AttachmentCfg: attachments.StoreConfig{Dir: t.TempDir()},
		},
		RouterConfig{Environment: "production"},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/dev/logs/index?limit=5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}
