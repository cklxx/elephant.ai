package app_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"alex/internal/auth/adapters"
	authapp "alex/internal/auth/app"
	"alex/internal/auth/domain"
	"alex/internal/auth/ports"
)

func TestRegisterAndLogin(t *testing.T) {
	users, identities, sessions, states := adapters.NewMemoryStores()
	tokenManager := adapters.NewJWTTokenManager("secret", "test", 15*time.Minute)
	sessions.SetVerifier(func(plain, encoded string) (bool, error) {
		return tokenManager.VerifyRefreshToken(plain, encoded)
	})
	service := authapp.NewService(users, identities, sessions, tokenManager, states, nil, authapp.Config{})

	ctx := context.Background()
	user, err := service.RegisterLocal(ctx, "test@example.com", "password", "Tester")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	tokens, err := service.LoginWithPassword(ctx, "test@example.com", "password", "agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("expected tokens to be issued: %+v", tokens)
	}
	if tokens.RefreshExpiry.Before(time.Now()) {
		t.Fatalf("expected refresh token expiry in future")
	}
	claims, err := service.ParseAccessToken(ctx, tokens.AccessToken)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if claims.Subject != user.ID {
		t.Fatalf("expected subject %s got %s", user.ID, claims.Subject)
	}

	refreshed, err := service.RefreshAccessToken(ctx, tokens.RefreshToken, "agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if refreshed.AccessToken == tokens.AccessToken {
		t.Fatalf("expected new access token on refresh")
	}
}

func TestOAuthFlowCreatesUser(t *testing.T) {
	users, identities, sessions, states := adapters.NewMemoryStores()
	tokenManager := adapters.NewJWTTokenManager("secret", "test", 15*time.Minute)
	sessions.SetVerifier(func(plain, encoded string) (bool, error) {
		return tokenManager.VerifyRefreshToken(plain, encoded)
	})

	provider := adapters.NewPassthroughOAuthProvider(adapters.OAuthProviderConfig{
		Provider:     domain.ProviderGoogle,
		ClientID:     "client",
		AuthURL:      "https://example.com/oauth",
		RedirectURL:  "https://app.example.com/callback",
		DefaultScope: []string{"openid"},
	})

	service := authapp.NewService(users, identities, sessions, tokenManager, states, []ports.OAuthProvider{provider}, authapp.Config{})

	ctx := context.Background()
	_, state, err := service.StartOAuth(ctx, domain.ProviderGoogle)
	if err != nil {
		t.Fatalf("start oauth: %v", err)
	}
	code := encodeOAuthCode(t, map[string]any{
		"provider_id":   "google-123",
		"email":         "oauth@example.com",
		"display_name":  "OAuth User",
		"access_token":  "third-party-access",
		"refresh_token": "third-party-refresh",
		"expires_in":    3600,
		"scopes":        []string{"openid"},
	})

	tokens, err := service.CompleteOAuth(ctx, domain.ProviderGoogle, code, state, "agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("complete oauth: %v", err)
	}
	claims, err := service.ParseAccessToken(ctx, tokens.AccessToken)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.Subject == "" {
		t.Fatalf("expected subject to be set")
	}
}

func encodeOAuthCode(t *testing.T, payload map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}
