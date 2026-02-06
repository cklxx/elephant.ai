package http

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	authapp "alex/internal/app/auth"
	authAdapters "alex/internal/infra/auth/adapters"
	"alex/internal/infra/observability"
	"alex/internal/shared/utils"
	id "alex/internal/shared/utils/id"
)

func TestCORSMiddlewareHonorsEnvironment(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	wrapped := CORSMiddleware("production", []string{"http://localhost:3000"})(handler)

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	req.Header.Set("Origin", "https://malicious.example")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no Access-Control-Allow-Origin in production for unlisted origin, got %q", got)
	}
}

func TestCORSMiddlewareAllowsListedOriginsInProduction(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	wrapped := CORSMiddleware("production", []string{"http://localhost:3000"})(handler)

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("expected allowed origin header, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected credentials header for allowed origin, got %q", got)
	}
}

func TestCORSMiddlewareAllowsAllOriginsInNonProduction(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	wrapped := CORSMiddleware("staging", []string{"http://localhost:3000"})(handler)

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	req.Header.Set("Origin", "https://example.dev")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected wildcard origin in non-production, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("expected no credentials header for wildcard origin, got %q", got)
	}
}

func TestCORSMiddlewareAllowsForwardedOriginInProduction(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	wrapped := CORSMiddleware("production", nil)(handler)

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	req.Header.Set("Origin", "https://alex.example.com")
	req.Header.Set("Forwarded", "proto=https;host=alex.example.com")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://alex.example.com" {
		t.Fatalf("expected forwarded origin to be allowed, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected credentials header for forwarded origin, got %q", got)
	}
}

func TestCORSMiddlewareRejectsUnknownOriginInProduction(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := CORSMiddleware("production", nil)(handler)

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	req.Header.Set("Origin", "https://alex.example.com")
	req.Header.Set("X-Forwarded-Host", "other.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected Access-Control-Allow-Origin to be empty, got %q", got)
	}
}

func TestObservabilityMiddlewareResolvesAnnotatedRoute(t *testing.T) {
	metrics := &observability.MetricsCollector{}
	recorded := make(chan string, 1)
	metrics.SetTestHooks(observability.MetricsTestHooks{
		HTTPServerRequest: func(method, route string, status int, duration time.Duration, responseBytes int64) {
			recorded <- route
		},
	})
	obs := &observability.Observability{Metrics: metrics}
	called := false
	handler := ObservabilityMiddleware(obs, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		annotateRequestRoute(r, "/api/sessions/:session_id/turns/:turn_id")
		w.WriteHeader(http.StatusAccepted)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/12345/turns/678", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if !called {
		t.Fatalf("expected handler to be invoked")
	}
	select {
	case route := <-recorded:
		if route != "/api/sessions/:session_id/turns/:turn_id" {
			t.Fatalf("expected annotated route, got %s", route)
		}
	default:
		t.Fatalf("expected HTTP metrics hook to be invoked")
	}
}

func TestObservabilityMiddlewareWritesLatencyLogWhenObservabilityDisabled(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ALEX_LOG_DIR", tempDir)
	utils.ResetLoggerForTests(utils.LogCategoryLatency)
	logPath := filepath.Join(tempDir, "alex-latency.log")
	_ = os.Remove(logPath)
	logger := utils.NewLatencyLogger("HTTP")

	handler := ObservabilityMiddleware(nil, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/tasks/123", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read latency log: %v", err)
	}
	contents := string(data)
	if !strings.Contains(contents, "route=/api/tasks/:id") {
		t.Fatalf("expected canonical route in latency log, got %s", contents)
	}
	if !strings.Contains(contents, "latency_ms=") {
		t.Fatalf("expected latency measurement in log: %s", contents)
	}
}

func TestAuthMiddlewareAcceptsAccessTokenQueryParameter(t *testing.T) {
	users, identities, sessions, states := authAdapters.NewMemoryStores()
	tokenManager := authAdapters.NewJWTTokenManager("secret", "test", time.Minute)
	service := authapp.NewService(users, identities, sessions, tokenManager, states, nil, authapp.Config{})

	ctx := context.Background()
	if _, err := service.RegisterLocal(ctx, "tester@example.com", "password", "Tester"); err != nil {
		t.Fatalf("register local user: %v", err)
	}

	tokens, err := service.LoginWithPassword(ctx, "tester@example.com", "password", "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("login with password: %v", err)
	}

	var called bool
	handler := AuthMiddleware(service)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		user, ok := CurrentUser(r.Context())
		if !ok {
			t.Fatalf("expected user in context")
		}
		if user.Email != "tester@example.com" {
			t.Fatalf("expected user email tester@example.com, got %s", user.Email)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/tasks?access_token="+url.QueryEscape(tokens.AccessToken), nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected OK status, got %d", rec.Code)
	}
	if !called {
		t.Fatalf("expected handler to be invoked")
	}
}

func TestAuthMiddlewareAcceptsAccessTokenCookie(t *testing.T) {
	users, identities, sessions, states := authAdapters.NewMemoryStores()
	tokenManager := authAdapters.NewJWTTokenManager("secret", "test", time.Minute)
	service := authapp.NewService(users, identities, sessions, tokenManager, states, nil, authapp.Config{})

	ctx := context.Background()
	if _, err := service.RegisterLocal(ctx, "tester@example.com", "password", "Tester"); err != nil {
		t.Fatalf("register local user: %v", err)
	}

	tokens, err := service.LoginWithPassword(ctx, "tester@example.com", "password", "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("login with password: %v", err)
	}

	var called bool
	handler := AuthMiddleware(service)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		user, ok := CurrentUser(r.Context())
		if !ok {
			t.Fatalf("expected user in context")
		}
		if user.Email != "tester@example.com" {
			t.Fatalf("expected user email tester@example.com, got %s", user.Email)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	req.AddCookie(&http.Cookie{
		Name:  "alex_access_token",
		Value: base64.StdEncoding.EncodeToString([]byte(tokens.AccessToken)),
		Path:  "/",
	})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected OK status, got %d", rec.Code)
	}
	if !called {
		t.Fatalf("expected handler to be invoked")
	}
}

func TestStreamGuardMiddlewareLimitsConcurrentStreams(t *testing.T) {
	block := make(chan struct{})
	started := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-block
	})

	wrapped := StreamGuardMiddleware(StreamGuardConfig{MaxConcurrent: 1})(handler)

	req1 := httptest.NewRequest(http.MethodGet, "/api/sse", nil)
	req1.Header.Set("Accept", "text/event-stream")
	rec1 := httptest.NewRecorder()
	go wrapped.ServeHTTP(rec1, req1)

	<-started

	req2 := httptest.NewRequest(http.MethodGet, "/api/sse", nil)
	req2.Header.Set("Accept", "text/event-stream")
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for concurrent stream, got %d", rec2.Code)
	}

	close(block)
}

func TestStreamGuardMiddlewareCancelsOnByteLimit(t *testing.T) {
	done := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", 32)))
		<-r.Context().Done()
		close(done)
	})

	wrapped := StreamGuardMiddleware(StreamGuardConfig{MaxBytes: 8})(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/sse", nil)
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()

	go wrapped.ServeHTTP(rec, req)

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected stream to cancel after byte limit")
	}
}

func TestStreamGuardMiddlewareCancelsOnDurationLimit(t *testing.T) {
	done := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		close(done)
	})

	wrapped := StreamGuardMiddleware(StreamGuardConfig{MaxDuration: 10 * time.Millisecond})(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/sse", nil)
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()

	go wrapped.ServeHTTP(rec, req)

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected stream to cancel after duration limit")
	}
}

func TestLoggingMiddlewareUsesProvidedLogID(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Context-Log-Id", id.LogIDFromContext(r.Context()))
		w.WriteHeader(http.StatusOK)
	})

	wrapped := LoggingMiddleware(nil)(handler)

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	req.Header.Set("X-Log-Id", "log-123")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Log-Id"); got != "log-123" {
		t.Fatalf("expected response log id log-123, got %q", got)
	}
	if got := rec.Header().Get("X-Context-Log-Id"); got != "log-123" {
		t.Fatalf("expected context log id log-123, got %q", got)
	}
}

func TestLoggingMiddlewareGeneratesLogID(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Context-Log-Id", id.LogIDFromContext(r.Context()))
		w.WriteHeader(http.StatusOK)
	})

	wrapped := LoggingMiddleware(nil)(handler)

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	logID := rec.Header().Get("X-Log-Id")
	if logID == "" {
		t.Fatalf("expected generated log id in response header")
	}
	if got := rec.Header().Get("X-Context-Log-Id"); got != logID {
		t.Fatalf("expected context log id %q, got %q", logID, got)
	}
}
