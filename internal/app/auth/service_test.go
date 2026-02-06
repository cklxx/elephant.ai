package app_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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

func TestCompleteOAuthBlocksDisabledUser(t *testing.T) {
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
	now := time.Now()

	disabledUser := domain.User{
		ID:               "user-disabled",
		Email:            "disabled@example.com",
		DisplayName:      "Disabled User",
		Status:           domain.UserStatusDisabled,
		PointsBalance:    0,
		SubscriptionTier: domain.SubscriptionTierFree,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if _, err := users.Create(ctx, disabledUser); err != nil {
		t.Fatalf("create disabled user: %v", err)
	}

	identity := domain.Identity{
		ID:         "identity-disabled",
		UserID:     disabledUser.ID,
		Provider:   domain.ProviderGoogle,
		ProviderID: "google-123",
		Tokens: domain.OAuthTokens{
			AccessToken:  "third-party-access",
			RefreshToken: "third-party-refresh",
			Expiry:       now.Add(time.Hour),
			Scopes:       []string{"openid"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := identities.Create(ctx, identity); err != nil {
		t.Fatalf("create identity: %v", err)
	}

	_, state, err := service.StartOAuth(ctx, domain.ProviderGoogle)
	if err != nil {
		t.Fatalf("start oauth: %v", err)
	}

	code := encodeOAuthCode(t, map[string]any{
		"provider_id":   "google-123",
		"email":         disabledUser.Email,
		"display_name":  disabledUser.DisplayName,
		"access_token":  "third-party-access",
		"refresh_token": "third-party-refresh",
		"expires_in":    3600,
		"scopes":        []string{"openid"},
	})

	if _, err := service.CompleteOAuth(ctx, domain.ProviderGoogle, code, state, "agent", "127.0.0.1"); err == nil || err.Error() != "user disabled" {
		t.Fatalf("expected disabled user error, got: %v", err)
	}
}

func TestAdjustPoints(t *testing.T) {
	users, identities, sessions, states := adapters.NewMemoryStores()
	tokenManager := adapters.NewJWTTokenManager("secret", "test", 15*time.Minute)
	service := authapp.NewService(users, identities, sessions, tokenManager, states, nil, authapp.Config{})

	ctx := context.Background()
	user, err := service.RegisterLocal(ctx, "points@example.com", "password", "Points User")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	updated, err := service.AdjustPoints(ctx, user.ID, 50)
	if err != nil {
		t.Fatalf("adjust points add: %v", err)
	}
	if updated.PointsBalance != 50 {
		t.Fatalf("expected 50 points, got %d", updated.PointsBalance)
	}
	updated, err = service.AdjustPoints(ctx, user.ID, -20)
	if err != nil {
		t.Fatalf("adjust points subtract: %v", err)
	}
	if updated.PointsBalance != 30 {
		t.Fatalf("expected 30 points after subtract, got %d", updated.PointsBalance)
	}
	if _, err := service.AdjustPoints(ctx, user.ID, -100); !errors.Is(err, domain.ErrInsufficientPoints) {
		t.Fatalf("expected insufficient points error, got %v", err)
	}
	final, err := service.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if final.PointsBalance != 30 {
		t.Fatalf("expected balance to remain 30, got %d", final.PointsBalance)
	}
}

func TestUpdateSubscription(t *testing.T) {
	users, identities, sessions, states := adapters.NewMemoryStores()
	tokenManager := adapters.NewJWTTokenManager("secret", "test", 15*time.Minute)
	service := authapp.NewService(users, identities, sessions, tokenManager, states, nil, authapp.Config{})

	ctx := context.Background()
	user, err := service.RegisterLocal(ctx, "subscription@example.com", "password", "Subscriber")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	supporterExpiry := time.Now().Add(30 * 24 * time.Hour)
	updated, err := service.UpdateSubscription(ctx, user.ID, domain.SubscriptionTierSupporter, &supporterExpiry)
	if err != nil {
		t.Fatalf("update subscription supporter: %v", err)
	}
	if updated.SubscriptionTier != domain.SubscriptionTierSupporter {
		t.Fatalf("expected supporter tier, got %s", updated.SubscriptionTier)
	}
	if updated.SubscriptionExpiresAt == nil || !updated.SubscriptionExpiresAt.Equal(supporterExpiry) {
		t.Fatalf("expected expiry to be %v, got %v", supporterExpiry, updated.SubscriptionExpiresAt)
	}
	if _, err := service.UpdateSubscription(ctx, user.ID, domain.SubscriptionTier("invalid"), nil); !errors.Is(err, domain.ErrInvalidSubscriptionTier) {
		t.Fatalf("expected invalid tier error, got %v", err)
	}
	if _, err := service.UpdateSubscription(ctx, user.ID, domain.SubscriptionTierSupporter, nil); !errors.Is(err, domain.ErrSubscriptionExpiryRequired) {
		t.Fatalf("expected expiry required error, got %v", err)
	}
	past := time.Now().Add(-time.Hour)
	if _, err := service.UpdateSubscription(ctx, user.ID, domain.SubscriptionTierSupporter, &past); !errors.Is(err, domain.ErrSubscriptionExpiryInPast) {
		t.Fatalf("expected expiry in past error, got %v", err)
	}
	free, err := service.UpdateSubscription(ctx, user.ID, domain.SubscriptionTierFree, nil)
	if err != nil {
		t.Fatalf("revert to free tier: %v", err)
	}
	if free.SubscriptionTier != domain.SubscriptionTierFree {
		t.Fatalf("expected free tier, got %s", free.SubscriptionTier)
	}
	if free.SubscriptionExpiresAt != nil {
		t.Fatalf("expected expiry to be cleared, got %v", free.SubscriptionExpiresAt)
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
