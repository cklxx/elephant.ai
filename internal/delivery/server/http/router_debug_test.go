package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"alex/internal/delivery/server/app"
)

func TestNewDebugRouter_RegistersDebugEndpoints(t *testing.T) {
	broadcaster := app.NewEventBroadcaster()
	healthChecker := app.NewHealthChecker()

	router := NewDebugRouter(DebugRouterDeps{
		Broadcaster:   broadcaster,
		HealthChecker: healthChecker,
	})

	// Endpoints that SHOULD be routed (non-404 response).
	// Some may return 503/400 at the handler level — that's fine, as long as
	// they don't return a mux-level 404.
	debugEndpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/health"},
		{"GET", "/api/sse"},
		{"GET", "/api/dev/logs"},
		{"GET", "/api/dev/logs/structured"},
		{"GET", "/api/dev/logs/index"},
		{"GET", "/api/dev/context-config"},
	}

	for _, ep := range debugEndpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusNotFound {
				t.Errorf("%s %s returned 404 — endpoint not registered", ep.method, ep.path)
			}
		})
	}
}

func TestNewDebugRouter_ContextSnapshotsReturns503(t *testing.T) {
	broadcaster := app.NewEventBroadcaster()
	healthChecker := app.NewHealthChecker()

	router := NewDebugRouter(DebugRouterDeps{
		Broadcaster:   broadcaster,
		HealthChecker: healthChecker,
	})

	req := httptest.NewRequest("GET", "/api/internal/sessions/test-session/context", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 503 (no coordinator), not panic or 404.
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("GET /api/internal/sessions/.../context returned %d; expected 503", w.Code)
	}
}

func TestNewDebugRouter_ContextWindowReturns503(t *testing.T) {
	broadcaster := app.NewEventBroadcaster()
	healthChecker := app.NewHealthChecker()

	router := NewDebugRouter(DebugRouterDeps{
		Broadcaster:   broadcaster,
		HealthChecker: healthChecker,
	})

	req := httptest.NewRequest("GET", "/api/dev/sessions/test-session/context-window", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 503 (no coordinator), not panic or 404.
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("GET /api/dev/sessions/.../context-window returned %d; expected 503", w.Code)
	}
}

func TestNewDebugRouter_MemoryEndpointRouted(t *testing.T) {
	broadcaster := app.NewEventBroadcaster()
	healthChecker := app.NewHealthChecker()

	// Without memory engine, the handler returns 404 (handler-level, not mux-level).
	// This test just ensures the route is wired and the handler runs.
	router := NewDebugRouter(DebugRouterDeps{
		Broadcaster:   broadcaster,
		HealthChecker: healthChecker,
	})

	req := httptest.NewRequest("GET", "/api/dev/memory", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 404 from handler (no memory engine) is acceptable — the route IS registered.
	// We verify it doesn't panic and returns a JSON error.
	if w.Code != http.StatusNotFound {
		t.Logf("GET /api/dev/memory returned %d (expected 404 from handler, no memory engine)", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		// Handler-level 404 returns JSON error; mux-level 404 returns text/plain.
		t.Errorf("GET /api/dev/memory Content-Type=%q; expected application/json (handler-level response)", ct)
	}
}

func TestNewDebugRouter_ExcludesWebEndpoints(t *testing.T) {
	broadcaster := app.NewEventBroadcaster()
	healthChecker := app.NewHealthChecker()

	router := NewDebugRouter(DebugRouterDeps{
		Broadcaster:   broadcaster,
		HealthChecker: healthChecker,
	})

	// Endpoints that MUST NOT be reachable (should 404 or 405).
	excludedEndpoints := []struct {
		method string
		path   string
	}{
		// Auth
		{"POST", "/api/auth/register"},
		{"POST", "/api/auth/login"},
		{"POST", "/api/auth/logout"},
		{"POST", "/api/auth/refresh"},
		{"GET", "/api/auth/me"},
		{"GET", "/api/auth/plans"},
		// Tasks
		{"POST", "/api/tasks"},
		{"GET", "/api/tasks"},
		{"GET", "/api/tasks/active"},
		{"GET", "/api/tasks/stats"},
		{"GET", "/api/tasks/test-task"},
		// Evaluations
		{"GET", "/api/evaluations"},
		{"POST", "/api/evaluations"},
		// Sessions
		{"GET", "/api/sessions"},
		{"POST", "/api/sessions"},
		{"GET", "/api/sessions/test-session"},
		{"DELETE", "/api/sessions/test-session"},
		// Agents
		{"GET", "/api/agents"},
		// Lark OAuth
		{"GET", "/api/lark/oauth/start"},
		{"GET", "/api/lark/oauth/callback"},
		// Share
		{"GET", "/api/share/sessions/test-session"},
	}

	for _, ep := range excludedEndpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// 404 or 405 means the endpoint is not registered, which is correct.
			if w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s returned %d — expected 404/405 (endpoint should not be registered in debug router)",
					ep.method, ep.path, w.Code)
			}
		})
	}
}

func TestNewDebugRouter_HooksBridgeOptional(t *testing.T) {
	broadcaster := app.NewEventBroadcaster()
	healthChecker := app.NewHealthChecker()

	// Without hooks bridge — endpoint should 404.
	router := NewDebugRouter(DebugRouterDeps{
		Broadcaster:   broadcaster,
		HealthChecker: healthChecker,
	})

	req := httptest.NewRequest("POST", "/api/hooks/claude-code", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/hooks/claude-code without bridge returned %d; expected 404/405", w.Code)
	}

	// With hooks bridge — endpoint should be reachable (non-404).
	stubBridge := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	routerWithBridge := NewDebugRouter(DebugRouterDeps{
		Broadcaster:   broadcaster,
		HealthChecker: healthChecker,
		HooksBridge:   stubBridge,
	})

	req = httptest.NewRequest("POST", "/api/hooks/claude-code", strings.NewReader("{}"))
	w = httptest.NewRecorder()
	routerWithBridge.ServeHTTP(w, req)
	if w.Code == http.StatusNotFound {
		t.Errorf("POST /api/hooks/claude-code with bridge returned 404")
	}
}

func TestNewDebugRouter_HealthReturns200(t *testing.T) {
	broadcaster := app.NewEventBroadcaster()
	healthChecker := app.NewHealthChecker()

	router := NewDebugRouter(DebugRouterDeps{
		Broadcaster:   broadcaster,
		HealthChecker: healthChecker,
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /health returned %d; expected 200", w.Code)
	}
}
