package http_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"alex/internal/auth/adapters"
	authapp "alex/internal/auth/app"
	"alex/internal/auth/domain"
	"alex/internal/auth/ports"
	serverhttp "alex/internal/server/http"
)

type userResponse struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	DisplayName   string `json:"display_name"`
	PointsBalance int64  `json:"points_balance"`
	Subscription  struct {
		Tier              string     `json:"tier"`
		MonthlyPriceCents int        `json:"monthly_price_cents"`
		ExpiresAt         *time.Time `json:"expires_at"`
	} `json:"subscription"`
}

type plansResponse struct {
	Plans []struct {
		Tier              string `json:"tier"`
		MonthlyPriceCents int    `json:"monthly_price_cents"`
	} `json:"plans"`
}

func TestHandleAdjustPoints(t *testing.T) {
	handler, service, token, _ := newAuthHandler(t)

	reqBody := bytes.NewBufferString(`{"delta": 75}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/points", reqBody)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.HandleAdjustPoints(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp userResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.PointsBalance != 75 {
		t.Fatalf("expected points balance 75, got %d", resp.PointsBalance)
	}

	user, err := service.GetUser(context.Background(), resp.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if user.PointsBalance != 75 {
		t.Fatalf("expected stored balance 75, got %d", user.PointsBalance)
	}
}

func TestHandleUpdateSubscription(t *testing.T) {
	handler, service, token, now := newAuthHandler(t)

	expiry := now.Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	body := bytes.NewBufferString(`{"tier":"supporter","expires_at":"` + expiry + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/subscription", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.HandleUpdateSubscription(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp userResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Subscription.Tier != "supporter" {
		t.Fatalf("expected tier supporter, got %s", resp.Subscription.Tier)
	}
	if resp.Subscription.MonthlyPriceCents != 2000 {
		t.Fatalf("expected supporter price 2000, got %d", resp.Subscription.MonthlyPriceCents)
	}
	if resp.Subscription.ExpiresAt == nil || resp.Subscription.ExpiresAt.Format(time.RFC3339) != expiry {
		t.Fatalf("expected expiry %s, got %v", expiry, resp.Subscription.ExpiresAt)
	}

	user, err := service.GetUser(context.Background(), resp.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if user.SubscriptionTier != "supporter" {
		t.Fatalf("expected stored tier supporter, got %s", user.SubscriptionTier)
	}
	if user.SubscriptionExpiresAt == nil || user.SubscriptionExpiresAt.Format(time.RFC3339) != expiry {
		t.Fatalf("expected stored expiry %s, got %v", expiry, user.SubscriptionExpiresAt)
	}
}

func TestHandleListPlans(t *testing.T) {
	handler, _, _, _ := newAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/plans", nil)
	rr := httptest.NewRecorder()

	handler.HandleListPlans(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp plansResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Plans) != 3 {
		t.Fatalf("expected 3 plans, got %d", len(resp.Plans))
	}
	if resp.Plans[0].Tier != "free" || resp.Plans[0].MonthlyPriceCents != 0 {
		t.Fatalf("unexpected first plan: %+v", resp.Plans[0])
	}
	if resp.Plans[1].Tier != "supporter" || resp.Plans[1].MonthlyPriceCents != 2000 {
		t.Fatalf("unexpected supporter plan: %+v", resp.Plans[1])
	}
	if resp.Plans[2].Tier != "professional" || resp.Plans[2].MonthlyPriceCents != 10000 {
		t.Fatalf("unexpected professional plan: %+v", resp.Plans[2])
	}
}

func TestRefreshCookieSameSiteModes(t *testing.T) {
	t.Run("insecure cookies use Lax mode", func(t *testing.T) {
		handler, _, _, _ := newAuthHandler(t)
		reqBody := bytes.NewBufferString(`{"email":"handler@example.com","password":"password"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", reqBody)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.HandleLogin(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}

		refresh := findRefreshCookie(rr.Result().Cookies())
		if refresh == nil {
			t.Fatalf("expected refresh cookie in response")
		}
		access := findAccessCookie(rr.Result().Cookies())
		if access == nil {
			t.Fatalf("expected access cookie in response")
		}
		if refresh.SameSite != http.SameSiteLaxMode {
			t.Fatalf("expected SameSite=Lax, got %v", refresh.SameSite)
		}
		if refresh.Secure {
			t.Fatalf("expected insecure cookie in dev mode")
		}
		if access.SameSite != http.SameSiteLaxMode {
			t.Fatalf("expected SameSite=Lax, got %v", access.SameSite)
		}
		if access.Secure {
			t.Fatalf("expected insecure cookie in dev mode")
		}
		if !access.HttpOnly {
			t.Fatalf("expected HttpOnly access cookie")
		}
	})

	t.Run("secure cookies use SameSite=None", func(t *testing.T) {
		handler, _, _, _ := newAuthHandlerWithSecure(t, true)
		reqBody := bytes.NewBufferString(`{"email":"handler@example.com","password":"password"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", reqBody)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.HandleLogin(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}

		refresh := findRefreshCookie(rr.Result().Cookies())
		if refresh == nil {
			t.Fatalf("expected refresh cookie in response")
		}
		access := findAccessCookie(rr.Result().Cookies())
		if access == nil {
			t.Fatalf("expected access cookie in response")
		}
		if refresh.SameSite != http.SameSiteNoneMode {
			t.Fatalf("expected SameSite=None, got %v", refresh.SameSite)
		}
		if !refresh.Secure {
			t.Fatalf("expected secure cookie in production mode")
		}
		if access.SameSite != http.SameSiteNoneMode {
			t.Fatalf("expected SameSite=None, got %v", access.SameSite)
		}
		if !access.Secure {
			t.Fatalf("expected secure cookie in production mode")
		}
		if !access.HttpOnly {
			t.Fatalf("expected HttpOnly access cookie")
		}
	})
}

func TestHandleOAuthCallbackPrefersHTML(t *testing.T) {
	handler, service := newOAuthEnabledAuthHandler(t)
	ctx := context.Background()
	_, state, err := service.StartOAuth(ctx, domain.ProviderGoogle)
	if err != nil {
		t.Fatalf("start oauth: %v", err)
	}
	code := encodePassthroughCode(t, "google-user", "oauth@example.com", "OAuth User")
	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?code="+code+"&state="+state, nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	rr := httptest.NewRecorder()
	handler.HandleOAuthCallback(domain.ProviderGoogle, rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected text/html response, got %s", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Login complete") {
		t.Fatalf("expected success page, got %s", body)
	}
	if !strings.Contains(body, "window.close") {
		t.Fatalf("expected auto-close script in response")
	}
}

func TestHandleOAuthCallbackFallsBackToJSON(t *testing.T) {
	handler, service := newOAuthEnabledAuthHandler(t)
	ctx := context.Background()
	_, state, err := service.StartOAuth(ctx, domain.ProviderGoogle)
	if err != nil {
		t.Fatalf("start oauth: %v", err)
	}
	code := encodePassthroughCode(t, "google-json", "json@example.com", "JSON User")
	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?code="+code+"&state="+state, nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	handler.HandleOAuthCallback(domain.ProviderGoogle, rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected JSON response, got %s", ct)
	}
	var resp struct {
		AccessToken string       `json:"access_token"`
		User        userResponse `json:"user"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.AccessToken == "" {
		t.Fatalf("expected access token in JSON response")
	}
	if resp.User.Email != "json@example.com" {
		t.Fatalf("expected user email json@example.com, got %s", resp.User.Email)
	}
}

func findRefreshCookie(cookies []*http.Cookie) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == "alex_refresh_token" {
			return cookie
		}
	}
	return nil
}

func findAccessCookie(cookies []*http.Cookie) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == "alex_access_token" {
			return cookie
		}
	}
	return nil
}

func newAuthHandler(t *testing.T) (*serverhttp.AuthHandler, *authapp.Service, string, time.Time) {
	return newAuthHandlerWithSecure(t, false)
}

func newAuthHandlerWithSecure(t *testing.T, secure bool) (*serverhttp.AuthHandler, *authapp.Service, string, time.Time) {
	t.Helper()
	users, identities, sessions, states := adapters.NewMemoryStores()
	tokenManager := adapters.NewJWTTokenManager("secret", "test", 15*time.Minute)
	sessions.SetVerifier(func(plain, encoded string) (bool, error) {
		return tokenManager.VerifyRefreshToken(plain, encoded)
	})
	service := authapp.NewService(users, identities, sessions, tokenManager, states, nil, authapp.Config{})
	fixed := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	service.WithNow(func() time.Time { return fixed })

	ctx := context.Background()
	if _, err := service.RegisterLocal(ctx, "handler@example.com", "password", "Handler"); err != nil {
		t.Fatalf("register: %v", err)
	}
	tokens, err := service.LoginWithPassword(ctx, "handler@example.com", "password", "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	return serverhttp.NewAuthHandler(service, secure), service, tokens.AccessToken, fixed
}

func newOAuthEnabledAuthHandler(t *testing.T) (*serverhttp.AuthHandler, *authapp.Service) {
	t.Helper()
	users, identities, sessions, states := adapters.NewMemoryStores()
	tokenManager := adapters.NewJWTTokenManager("secret", "test", 15*time.Minute)
	provider := adapters.NewPassthroughOAuthProvider(adapters.OAuthProviderConfig{
		Provider:    domain.ProviderGoogle,
		ClientID:    "client-id",
		AuthURL:     "https://example.com/oauth",
		RedirectURL: "http://localhost/api/auth/google/callback",
	})
	service := authapp.NewService(
		users,
		identities,
		sessions,
		tokenManager,
		states,
		[]ports.OAuthProvider{provider},
		authapp.Config{},
	)
	return serverhttp.NewAuthHandler(service, false), service
}

func encodePassthroughCode(t *testing.T, providerID, email, displayName string) string {
	t.Helper()
	payload := map[string]any{
		"provider_id":   providerID,
		"email":         email,
		"display_name":  displayName,
		"access_token":  "remote-access-token",
		"refresh_token": "remote-refresh-token",
		"expires_in":    3600,
		"scopes":        []string{"profile"},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}
