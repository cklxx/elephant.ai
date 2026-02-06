package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"alex/internal/auth/domain"
	"alex/internal/auth/ports"
	"alex/internal/httpclient"
)

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
	HTTPClient   *http.Client
}

type GoogleOAuthProvider struct {
	cfg        GoogleOAuthConfig
	httpClient *http.Client
	scopes     []string
}

const (
	defaultGoogleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultGoogleTokenURL    = "https://oauth2.googleapis.com/token"
	defaultGoogleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
	googleHTTPTimeout        = 10 * time.Second
	maxGoogleErrorBody       = int64(1 << 20) // 1 MiB
)

func NewGoogleOAuthProvider(cfg GoogleOAuthConfig) *GoogleOAuthProvider {
	client := cfg.HTTPClient
	if client == nil {
		client = httpclient.New(googleHTTPTimeout, nil)
	}

	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "email", "profile"}
	}

	normalized := GoogleOAuthConfig{
		ClientID:     strings.TrimSpace(cfg.ClientID),
		ClientSecret: strings.TrimSpace(cfg.ClientSecret),
		RedirectURL:  strings.TrimSpace(cfg.RedirectURL),
		AuthURL:      strings.TrimSpace(cfg.AuthURL),
		TokenURL:     strings.TrimSpace(cfg.TokenURL),
		UserInfoURL:  strings.TrimSpace(cfg.UserInfoURL),
	}

	if normalized.AuthURL == "" {
		normalized.AuthURL = defaultGoogleAuthURL
	}
	if normalized.TokenURL == "" {
		normalized.TokenURL = defaultGoogleTokenURL
	}
	if normalized.UserInfoURL == "" {
		normalized.UserInfoURL = defaultGoogleUserInfoURL
	}

	return &GoogleOAuthProvider{cfg: normalized, httpClient: client, scopes: scopes}
}

func (p *GoogleOAuthProvider) Provider() domain.ProviderType {
	return domain.ProviderGoogle
}

func (p *GoogleOAuthProvider) BuildAuthURL(state string) (string, error) {
	if p.cfg.ClientID == "" || p.cfg.RedirectURL == "" {
		return "", fmt.Errorf("google oauth provider is not configured")
	}
	u, err := url.Parse(p.cfg.AuthURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", p.cfg.ClientID)
	q.Set("redirect_uri", p.cfg.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(p.scopes, " "))
	q.Set("state", state)
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

type googleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
	IDToken      string `json:"id_token"`
}

type googleUserInfoResponse struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
}

func (p *GoogleOAuthProvider) Exchange(ctx context.Context, code string) (ports.OAuthUserInfo, error) {
	if p.cfg.ClientID == "" || p.cfg.ClientSecret == "" || p.cfg.RedirectURL == "" {
		return ports.OAuthUserInfo{}, fmt.Errorf("google oauth provider is not configured")
	}
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", p.cfg.ClientID)
	form.Set("client_secret", p.cfg.ClientSecret)
	form.Set("redirect_uri", p.cfg.RedirectURL)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return ports.OAuthUserInfo{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return ports.OAuthUserInfo{}, fmt.Errorf("google: exchange token request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxGoogleErrorBody))
		return ports.OAuthUserInfo{}, fmt.Errorf("google: token exchange failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var token googleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return ports.OAuthUserInfo{}, fmt.Errorf("google: decode token response: %w", err)
	}

	if token.AccessToken == "" {
		return ports.OAuthUserInfo{}, fmt.Errorf("google: token response missing access_token")
	}

	infoReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.UserInfoURL, nil)
	if err != nil {
		return ports.OAuthUserInfo{}, err
	}
	infoReq.Header.Set("Authorization", "Bearer "+token.AccessToken)

	infoResp, err := p.httpClient.Do(infoReq)
	if err != nil {
		return ports.OAuthUserInfo{}, fmt.Errorf("google: user info request failed: %w", err)
	}
	defer func() {
		_ = infoResp.Body.Close()
	}()

	if infoResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(infoResp.Body, maxGoogleErrorBody))
		return ports.OAuthUserInfo{}, fmt.Errorf("google: user info failed: status=%d body=%s", infoResp.StatusCode, strings.TrimSpace(string(body)))
	}

	var profile googleUserInfoResponse
	if err := json.NewDecoder(infoResp.Body).Decode(&profile); err != nil {
		return ports.OAuthUserInfo{}, fmt.Errorf("google: decode user info response: %w", err)
	}

	if strings.TrimSpace(profile.Sub) == "" {
		return ports.OAuthUserInfo{}, fmt.Errorf("google: user info missing subject")
	}

	email := strings.TrimSpace(profile.Email)
	if email == "" {
		return ports.OAuthUserInfo{}, fmt.Errorf("google: user info missing email")
	}
	displayName := strings.TrimSpace(profile.Name)
	if displayName == "" {
		displayName = email
	}

	scopes := []string(nil)
	if token.Scope != "" {
		scopes = strings.Fields(token.Scope)
	}
	expiresIn := time.Duration(token.ExpiresIn) * time.Second
	if expiresIn <= 0 {
		expiresIn = time.Hour
	}

	return ports.OAuthUserInfo{
		ProviderID:   profile.Sub,
		Email:        email,
		DisplayName:  displayName,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expiry:       time.Now().Add(expiresIn),
		Scopes:       scopes,
	}, nil
}

var _ ports.OAuthProvider = (*GoogleOAuthProvider)(nil)
