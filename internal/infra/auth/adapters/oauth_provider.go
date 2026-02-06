package adapters

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"alex/internal/domain/auth"
	"alex/internal/domain/auth/ports"
)

// OAuthProviderConfig configures the simple OAuth provider stub.
type OAuthProviderConfig struct {
	Provider     domain.ProviderType
	ClientID     string
	AuthURL      string
	RedirectURL  string
	DefaultScope []string
}

// PassthroughOAuthProvider is a lightweight provider useful for development and tests.
// It encodes user details in the returned OAuth "code" payload to avoid outbound calls.
type PassthroughOAuthProvider struct {
	cfg OAuthProviderConfig
}

// NewPassthroughOAuthProvider creates the stub provider.
func NewPassthroughOAuthProvider(cfg OAuthProviderConfig) *PassthroughOAuthProvider {
	return &PassthroughOAuthProvider{cfg: cfg}
}

// Provider implements ports.OAuthProvider.
func (p *PassthroughOAuthProvider) Provider() domain.ProviderType {
	return p.cfg.Provider
}

// BuildAuthURL builds a URL that includes state and static query parameters.
func (p *PassthroughOAuthProvider) BuildAuthURL(state string) (string, error) {
	u, err := url.Parse(p.cfg.AuthURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", p.cfg.ClientID)
	q.Set("redirect_uri", p.cfg.RedirectURL)
	q.Set("response_type", "code")
	if len(p.cfg.DefaultScope) > 0 {
		q.Set("scope", joinScopes(p.cfg.DefaultScope))
	}
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// Exchange decodes the base64 JSON payload carrying user information.
func (p *PassthroughOAuthProvider) Exchange(_ context.Context, code string) (ports.OAuthUserInfo, error) {
	data, err := base64.RawURLEncoding.DecodeString(code)
	if err != nil {
		return ports.OAuthUserInfo{}, fmt.Errorf("invalid authorization code payload: %w", err)
	}
	var payload struct {
		ProviderID   string   `json:"provider_id"`
		Email        string   `json:"email"`
		DisplayName  string   `json:"display_name"`
		AccessToken  string   `json:"access_token"`
		RefreshToken string   `json:"refresh_token"`
		ExpiresIn    int      `json:"expires_in"`
		Scopes       []string `json:"scopes"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return ports.OAuthUserInfo{}, err
	}
	expiry := time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	return ports.OAuthUserInfo{
		ProviderID:   payload.ProviderID,
		Email:        payload.Email,
		DisplayName:  payload.DisplayName,
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		Expiry:       expiry,
		Scopes:       payload.Scopes,
	}, nil
}

func joinScopes(scopes []string) string {
	return strings.Join(scopes, " ")
}

var _ ports.OAuthProvider = (*PassthroughOAuthProvider)(nil)
