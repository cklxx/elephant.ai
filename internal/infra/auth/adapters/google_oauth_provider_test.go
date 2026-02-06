package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestGoogleOAuthProviderBuildAuthURL(t *testing.T) {
	provider := NewGoogleOAuthProvider(GoogleOAuthConfig{
		ClientID:    "client-123",
		RedirectURL: "https://example.com/callback",
		AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
		Scopes:      []string{"openid", "email"},
	})

	urlStr, err := provider.BuildAuthURL("state-xyz")
	if err != nil {
		t.Fatalf("BuildAuthURL returned error: %v", err)
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		t.Fatalf("failed to parse auth url: %v", err)
	}

	q := parsed.Query()
	if got := q.Get("client_id"); got != "client-123" {
		t.Errorf("client_id = %s, want client-123", got)
	}
	if got := q.Get("redirect_uri"); got != "https://example.com/callback" {
		t.Errorf("redirect_uri = %s, want callback", got)
	}
	if got := q.Get("scope"); got != "openid email" {
		t.Errorf("scope = %s, want openid email", got)
	}
	if got := q.Get("state"); got != "state-xyz" {
		t.Errorf("state = %s, want state-xyz", got)
	}
}

func TestGoogleOAuthProviderExchange(t *testing.T) {
	tokenCalled := false
	userInfoCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			tokenCalled = true
			if err := r.ParseForm(); err != nil {
				t.Fatalf("failed to parse form: %v", err)
			}
			if got := r.FormValue("code"); got != "auth-code" {
				t.Fatalf("code = %s, want auth-code", got)
			}
			resp := map[string]any{
				"access_token":  "access-xyz",
				"refresh_token": "refresh-xyz",
				"expires_in":    3600,
				"scope":         "openid email",
				"token_type":    "Bearer",
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/userinfo":
			userInfoCalled = true
			authHeader := r.Header.Get("Authorization")
			if authHeader != "Bearer access-xyz" {
				t.Fatalf("Authorization header = %s", authHeader)
			}
			resp := map[string]any{
				"sub":   "google-user",
				"email": "user@example.com",
				"name":  "Example User",
			}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	provider := NewGoogleOAuthProvider(GoogleOAuthConfig{
		ClientID:     "client-123",
		ClientSecret: "secret-456",
		RedirectURL:  "https://example.com/callback",
		TokenURL:     server.URL + "/token",
		UserInfoURL:  server.URL + "/userinfo",
		HTTPClient:   server.Client(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	info, err := provider.Exchange(ctx, "auth-code")
	if err != nil {
		t.Fatalf("Exchange returned error: %v", err)
	}

	if !tokenCalled {
		t.Fatal("token endpoint not called")
	}
	if !userInfoCalled {
		t.Fatal("userinfo endpoint not called")
	}
	if info.ProviderID != "google-user" {
		t.Fatalf("ProviderID = %s", info.ProviderID)
	}
	if info.Email != "user@example.com" {
		t.Fatalf("Email = %s", info.Email)
	}
	if info.DisplayName != "Example User" {
		t.Fatalf("DisplayName = %s", info.DisplayName)
	}
	if info.AccessToken != "access-xyz" {
		t.Fatalf("AccessToken = %s", info.AccessToken)
	}
	if info.RefreshToken != "refresh-xyz" {
		t.Fatalf("RefreshToken = %s", info.RefreshToken)
	}
	if len(info.Scopes) != 2 || info.Scopes[0] != "openid" || info.Scopes[1] != "email" {
		t.Fatalf("unexpected scopes: %#v", info.Scopes)
	}
}

func TestGoogleOAuthProviderExchangeHandlesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid grant"))
	}))
	defer server.Close()

	provider := NewGoogleOAuthProvider(GoogleOAuthConfig{
		ClientID:     "client-123",
		ClientSecret: "secret-456",
		RedirectURL:  "https://example.com/callback",
		TokenURL:     server.URL + "/token",
		HTTPClient:   server.Client(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, err := provider.Exchange(ctx, "auth-code"); err == nil || !strings.Contains(err.Error(), "token exchange failed") {
		t.Fatalf("expected token exchange error, got %v", err)
	}
}
