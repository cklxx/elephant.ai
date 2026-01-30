package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
