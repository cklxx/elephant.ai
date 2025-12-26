package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	serverapp "alex/internal/server/app"
)

func TestRouterRegistersAuthEndpointsWhenDisabled(t *testing.T) {
	router := NewRouter(nil, serverapp.NewEventBroadcaster(), serverapp.NewHealthChecker(), nil, nil, "development", nil, nil, nil, nil)

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

func TestCreateTaskBodyLimitFromEnv(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		limit := createTaskBodyLimit(nil)
		if limit != defaultMaxCreateTaskBodySize {
			t.Fatalf("expected default limit %d, got %d", defaultMaxCreateTaskBodySize, limit)
		}
	})

	t.Run("valid override", func(t *testing.T) {
		limit := createTaskBodyLimit(func(key string) (string, bool) {
			if key == "ALEX_WEB_MAX_TASK_BODY_BYTES" {
				return "5242880", true
			}
			return "", false
		})
		if limit != 5<<20 {
			t.Fatalf("expected 5 MiB limit, got %d", limit)
		}
	})

	t.Run("invalid override falls back", func(t *testing.T) {
		limit := createTaskBodyLimit(func(key string) (string, bool) {
			if key == "ALEX_WEB_MAX_TASK_BODY_BYTES" {
				return "not-a-number", true
			}
			return "", false
		})
		if limit != defaultMaxCreateTaskBodySize {
			t.Fatalf("expected default limit %d on parse failure, got %d", defaultMaxCreateTaskBodySize, limit)
		}
	})
}
