package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestBearerAuth_EmptyToken_PassThrough(t *testing.T) {
	handler := BearerAuthMiddleware("")(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/leader/dashboard", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when token is empty, got %d", rec.Code)
	}
}

func TestBearerAuth_ValidBearerToken(t *testing.T) {
	handler := BearerAuthMiddleware("secret123")(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/leader/dashboard", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid bearer token, got %d", rec.Code)
	}
}

func TestBearerAuth_ValidAPIKey(t *testing.T) {
	handler := BearerAuthMiddleware("secret123")(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/leader/dashboard", nil)
	req.Header.Set("X-API-Key", "secret123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid X-API-Key, got %d", rec.Code)
	}
}

func TestBearerAuth_MissingHeader(t *testing.T) {
	handler := BearerAuthMiddleware("secret123")(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/leader/dashboard", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing header, got %d", rec.Code)
	}
}

func TestBearerAuth_WrongToken(t *testing.T) {
	handler := BearerAuthMiddleware("secret123")(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/leader/dashboard", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong token, got %d", rec.Code)
	}
}

func TestBearerAuth_MalformedHeader(t *testing.T) {
	handler := BearerAuthMiddleware("secret123")(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/leader/dashboard", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for malformed header, got %d", rec.Code)
	}
}

func TestBearerAuth_EmptyBearer(t *testing.T) {
	handler := BearerAuthMiddleware("secret123")(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/leader/dashboard", nil)
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for empty bearer value, got %d", rec.Code)
	}
}
