package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkauthen "github.com/larksuite/oapi-sdk-go/v3/service/authen/v1"
)

type ServiceConfig struct {
	AppID         string
	AppSecret     string
	BaseDomain    string
	RedirectBase  string // e.g. https://your-host
	StateTTL      time.Duration
	RefreshLeeway time.Duration
}

type Service struct {
	cfg        ServiceConfig
	client     *lark.Client
	tokens     TokenStore
	states     StateStore
	now        func() time.Time
	stateBytes int
}

func NewService(cfg ServiceConfig, tokens TokenStore, states StateStore) (*Service, error) {
	cfg.AppID = strings.TrimSpace(cfg.AppID)
	cfg.AppSecret = strings.TrimSpace(cfg.AppSecret)
	cfg.BaseDomain = strings.TrimSpace(cfg.BaseDomain)
	cfg.RedirectBase = strings.TrimRight(strings.TrimSpace(cfg.RedirectBase), "/")

	if cfg.AppID == "" || cfg.AppSecret == "" {
		return nil, fmt.Errorf("app_id/app_secret required")
	}
	if cfg.BaseDomain == "" {
		cfg.BaseDomain = "https://open.feishu.cn"
	}
	if cfg.RedirectBase == "" {
		return nil, fmt.Errorf("redirect_base required")
	}
	if cfg.StateTTL <= 0 {
		cfg.StateTTL = 10 * time.Minute
	}
	if cfg.RefreshLeeway <= 0 {
		cfg.RefreshLeeway = 5 * time.Minute
	}
	if tokens == nil {
		return nil, fmt.Errorf("token store required")
	}
	if states == nil {
		states = NewMemoryStateStore()
	}

	var clientOpts []lark.ClientOptionFunc
	if cfg.BaseDomain != "" {
		clientOpts = append(clientOpts, lark.WithOpenBaseUrl(cfg.BaseDomain))
	}
	svc := &Service{
		cfg:        cfg,
		client:     lark.NewClient(cfg.AppID, cfg.AppSecret, clientOpts...),
		tokens:     tokens,
		states:     states,
		now:        time.Now,
		stateBytes: 24,
	}
	return svc, nil
}

func (s *Service) StartURL() string {
	if s == nil {
		return ""
	}
	if s.cfg.RedirectBase == "" {
		return ""
	}
	return s.cfg.RedirectBase + "/api/lark/oauth/start"
}

func (s *Service) callbackURL() string {
	if s == nil || s.cfg.RedirectBase == "" {
		return ""
	}
	return s.cfg.RedirectBase + "/api/lark/oauth/callback"
}

// AuthorizeURL returns the Lark OAuth authorization page URL for the given state.
func (s *Service) AuthorizeURL(state string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("service nil")
	}
	cb := s.callbackURL()
	if cb == "" {
		return "", fmt.Errorf("callback url not configured")
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return "", fmt.Errorf("state required")
	}

	base := strings.TrimRight(strings.TrimSpace(s.cfg.BaseDomain), "/")
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "https://" + base
	}

	u, err := url.Parse(base + "/open-apis/authen/v1/index")
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("app_id", s.cfg.AppID)
	q.Set("redirect_uri", cb)
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// StartAuth creates a short-lived state and returns the Lark authorization URL.
func (s *Service) StartAuth(ctx context.Context) (string, string, error) {
	if s == nil {
		return "", "", fmt.Errorf("service nil")
	}
	state, err := randomState(s.stateBytes)
	if err != nil {
		return "", "", err
	}
	if err := s.states.Save(ctx, state, s.now().Add(s.cfg.StateTTL)); err != nil {
		return "", "", err
	}
	authURL, err := s.AuthorizeURL(state)
	if err != nil {
		return "", "", err
	}
	return state, authURL, nil
}

// HandleCallback consumes state, exchanges code for tokens and stores them.
func (s *Service) HandleCallback(ctx context.Context, code, state string) (Token, error) {
	if s == nil {
		return Token{}, fmt.Errorf("service nil")
	}
	code = strings.TrimSpace(code)
	state = strings.TrimSpace(state)
	if code == "" {
		return Token{}, fmt.Errorf("code required")
	}
	if state == "" {
		return Token{}, fmt.Errorf("state required")
	}
	if err := s.states.Consume(ctx, state); err != nil {
		return Token{}, err
	}

	req := larkauthen.NewCreateAccessTokenReqBuilder().Body(
		larkauthen.NewCreateAccessTokenReqBodyBuilder().
			GrantType("authorization_code").
			Code(code).
			Build(),
	).Build()
	resp, err := s.client.Authen.AccessToken.Create(ctx, req)
	if err != nil {
		return Token{}, fmt.Errorf("lark authen access_token create: %w", err)
	}
	if !resp.Success() {
		return Token{}, fmt.Errorf("lark authen access_token create failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.OpenId == nil || strings.TrimSpace(*resp.Data.OpenId) == "" {
		return Token{}, fmt.Errorf("lark authen access_token response missing open_id")
	}
	openID := strings.TrimSpace(*resp.Data.OpenId)
	accessToken := derefString(resp.Data.AccessToken)
	refreshToken := derefString(resp.Data.RefreshToken)
	expiresIn := derefInt(resp.Data.ExpiresIn)
	refreshExpiresIn := derefInt(resp.Data.RefreshExpiresIn)

	now := s.now()
	token := Token{
		OpenID:       openID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    now.Add(time.Duration(expiresIn) * time.Second),
		TokenType:    derefString(resp.Data.TokenType),
		UpdatedAt:    now,
	}
	if refreshExpiresIn > 0 {
		token.RefreshExpiresAt = now.Add(time.Duration(refreshExpiresIn) * time.Second)
	}
	if err := s.tokens.Upsert(ctx, token); err != nil {
		return Token{}, fmt.Errorf("store token: %w", err)
	}
	return token, nil
}

// UserAccessToken returns a valid user_access_token for the provided open_id.
// If none exists, returns *NeedUserAuthError containing the OAuth start URL.
func (s *Service) UserAccessToken(ctx context.Context, openID string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("service nil")
	}
	openID = strings.TrimSpace(openID)
	if openID == "" {
		return "", fmt.Errorf("open_id required")
	}

	token, err := s.tokens.Get(ctx, openID)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return "", &NeedUserAuthError{AuthURL: s.StartURL()}
		}
		return "", err
	}

	now := s.now()
	if token.AccessValidAt(now, s.cfg.RefreshLeeway) {
		return token.AccessToken, nil
	}

	refreshed, err := s.refresh(ctx, token.RefreshToken)
	if err != nil {
		return "", err
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = token.RefreshToken
		refreshed.RefreshExpiresAt = token.RefreshExpiresAt
	}
	if refreshed.OpenID == "" {
		refreshed.OpenID = token.OpenID
	}
	// Store refreshed token under its own open_id (may differ from the requested open_id).
	if err := s.tokens.Upsert(ctx, refreshed); err != nil {
		return "", fmt.Errorf("store refreshed token: %w", err)
	}
	return refreshed.AccessToken, nil
}

func (s *Service) refresh(ctx context.Context, refreshToken string) (Token, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return Token{}, fmt.Errorf("refresh_token required")
	}
	req := larkauthen.NewCreateRefreshAccessTokenReqBuilder().Body(
		larkauthen.NewCreateRefreshAccessTokenReqBodyBuilder().
			GrantType("refresh_token").
			RefreshToken(refreshToken).
			Build(),
	).Build()
	resp, err := s.client.Authen.RefreshAccessToken.Create(ctx, req)
	if err != nil {
		return Token{}, fmt.Errorf("lark authen refresh_access_token create: %w", err)
	}
	if !resp.Success() {
		return Token{}, fmt.Errorf("lark authen refresh_access_token create failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.OpenId == nil || strings.TrimSpace(*resp.Data.OpenId) == "" {
		return Token{}, fmt.Errorf("lark authen refresh response missing open_id")
	}

	now := s.now()
	expiresIn := derefInt(resp.Data.ExpiresIn)
	refreshExpiresIn := derefInt(resp.Data.RefreshExpiresIn)
	token := Token{
		OpenID:       strings.TrimSpace(*resp.Data.OpenId),
		AccessToken:  derefString(resp.Data.AccessToken),
		RefreshToken: derefString(resp.Data.RefreshToken),
		ExpiresAt:    now.Add(time.Duration(expiresIn) * time.Second),
		UpdatedAt:    now,
		TokenType:    derefString(resp.Data.TokenType),
	}
	if refreshExpiresIn > 0 {
		token.RefreshExpiresAt = now.Add(time.Duration(refreshExpiresIn) * time.Second)
	}
	return token, nil
}

func randomState(n int) (string, error) {
	if n <= 0 {
		n = 24
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
