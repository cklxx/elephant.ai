package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	authAdapters "alex/internal/auth/adapters"
	authapp "alex/internal/auth/app"
	"alex/internal/observability"
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

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.dev" {
		t.Fatalf("expected origin echoed in non-production, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected credentials header even in non-production, got %q", got)
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
	handler := ObservabilityMiddleware(obs)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
