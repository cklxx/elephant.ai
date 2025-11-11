package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"alex/internal/auth/domain"
	"alex/internal/auth/ports"
)

// Config controls token expirations and OAuth behaviour.
type Config struct {
	AccessTokenTTL        time.Duration
	RefreshTokenTTL       time.Duration
	StateTTL              time.Duration
	RedirectBaseURL       string
	SecureCookies         bool
	AllowedCallbackDomain string
}

// Service orchestrates authentication workflows.
type Service struct {
	users      ports.UserRepository
	identities ports.IdentityRepository
	sessions   ports.SessionRepository
	tokens     ports.TokenManager
	states     ports.StateStore
	providers  map[domain.ProviderType]ports.OAuthProvider
	config     Config
	now        func() time.Time
}

// NewService constructs a Service instance.
func NewService(users ports.UserRepository, identities ports.IdentityRepository, sessions ports.SessionRepository, tokens ports.TokenManager, states ports.StateStore, providers []ports.OAuthProvider, cfg Config) *Service {
	providerMap := map[domain.ProviderType]ports.OAuthProvider{}
	for _, p := range providers {
		if p == nil {
			continue
		}
		providerMap[p.Provider()] = p
	}
	if sessions != nil && tokens != nil {
		type refreshVerifier interface {
			SetVerifier(func(string, string) (bool, error))
		}
		if verifier, ok := sessions.(refreshVerifier); ok {
			verifier.SetVerifier(func(plain, encoded string) (bool, error) {
				return tokens.VerifyRefreshToken(plain, encoded)
			})
		}
	}
	if cfg.AccessTokenTTL == 0 {
		cfg.AccessTokenTTL = 15 * time.Minute
	}
	if cfg.RefreshTokenTTL == 0 {
		cfg.RefreshTokenTTL = 30 * 24 * time.Hour
	}
	if cfg.StateTTL == 0 {
		cfg.StateTTL = 10 * time.Minute
	}
	return &Service{
		users:      users,
		identities: identities,
		sessions:   sessions,
		tokens:     tokens,
		states:     states,
		providers:  providerMap,
		config:     cfg,
		now:        time.Now,
	}
}

// WithNow allows tests to control the clock.
func (s *Service) WithNow(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

// RegisterLocal registers a new local user with username/password.
func (s *Service) RegisterLocal(ctx context.Context, email, password, displayName string) (domain.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return domain.User{}, fmt.Errorf("email is required")
	}
	if password == "" {
		return domain.User{}, fmt.Errorf("password is required")
	}

	if _, err := s.users.FindByEmail(ctx, email); err == nil {
		return domain.User{}, domain.ErrUserExists
	}

	hashed, err := s.tokens.HashRefreshToken(password)
	if err != nil {
		return domain.User{}, fmt.Errorf("hash password: %w", err)
	}

	now := s.now()
	user := domain.User{
		ID:           uuid.NewString(),
		Email:        email,
		DisplayName:  displayName,
		Status:       domain.UserStatusActive,
		PasswordHash: hashed,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	created, err := s.users.Create(ctx, user)
	if err != nil {
		return domain.User{}, err
	}
	return created, nil
}

// LoginWithPassword authenticates a user using email/password.
func (s *Service) LoginWithPassword(ctx context.Context, email, password, userAgent, ip string) (domain.TokenPair, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return domain.TokenPair{}, domain.ErrInvalidCredentials
	}
	ok, err := s.tokens.VerifyRefreshToken(password, user.PasswordHash)
	if err != nil || !ok {
		return domain.TokenPair{}, domain.ErrInvalidCredentials
	}
	if user.Status != domain.UserStatusActive {
		return domain.TokenPair{}, fmt.Errorf("user disabled")
	}

	plainRefresh, hashedRefresh, err := s.tokens.GenerateRefreshToken(ctx)
	if err != nil {
		return domain.TokenPair{}, err
	}
	session := domain.Session{
		ID:               uuid.NewString(),
		UserID:           user.ID,
		RefreshTokenHash: hashedRefresh,
		UserAgent:        userAgent,
		IP:               ip,
		CreatedAt:        s.now(),
		ExpiresAt:        s.now().Add(s.config.RefreshTokenTTL),
	}
	if _, err := s.sessions.Create(ctx, session); err != nil {
		return domain.TokenPair{}, err
	}
	accessToken, expiresAt, err := s.tokens.GenerateAccessToken(ctx, user, session.ID)
	if err != nil {
		return domain.TokenPair{}, err
	}
	return domain.TokenPair{
		AccessToken:   accessToken,
		AccessExpiry:  expiresAt,
		RefreshToken:  plainRefresh,
		RefreshExpiry: session.ExpiresAt,
	}, nil
}

// RefreshAccessToken rotates refresh tokens and issues a new access token.
func (s *Service) RefreshAccessToken(ctx context.Context, refreshToken, userAgent, ip string) (domain.TokenPair, error) {
	session, err := s.sessions.FindByRefreshToken(ctx, refreshToken)
	if err != nil {
		return domain.TokenPair{}, err
	}
	if session.ExpiresAt.Before(s.now()) {
		return domain.TokenPair{}, domain.ErrSessionExpired
	}
	user, err := s.users.FindByID(ctx, session.UserID)
	if err != nil {
		return domain.TokenPair{}, err
	}

	// rotate refresh token
	plainRefresh, hashedRefresh, err := s.tokens.GenerateRefreshToken(ctx)
	if err != nil {
		return domain.TokenPair{}, err
	}
	_ = s.sessions.DeleteByID(ctx, session.ID)
	session.ID = uuid.NewString()
	session.RefreshTokenHash = hashedRefresh
	session.UserAgent = userAgent
	session.IP = ip
	session.CreatedAt = s.now()
	session.ExpiresAt = s.now().Add(s.config.RefreshTokenTTL)
	if _, err := s.sessions.Create(ctx, session); err != nil {
		return domain.TokenPair{}, err
	}
	access, exp, err := s.tokens.GenerateAccessToken(ctx, user, session.ID)
	if err != nil {
		return domain.TokenPair{}, err
	}
	return domain.TokenPair{
		AccessToken:   access,
		AccessExpiry:  exp,
		RefreshToken:  plainRefresh,
		RefreshExpiry: session.ExpiresAt,
	}, nil
}

// Logout invalidates refresh token sessions.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	session, err := s.sessions.FindByRefreshToken(ctx, refreshToken)
	if err != nil {
		return err
	}
	return s.sessions.DeleteByID(ctx, session.ID)
}

// StartOAuth begins the OAuth flow returning an authorization URL.
func (s *Service) StartOAuth(ctx context.Context, provider domain.ProviderType) (string, string, error) {
	p, ok := s.providers[provider]
	if !ok {
		return "", "", domain.ErrProviderNotConfigured
	}
	state := randomState()
	if err := s.states.Save(ctx, state, provider, s.now().Add(s.config.StateTTL)); err != nil {
		return "", "", err
	}
	url, err := p.BuildAuthURL(state)
	if err != nil {
		return "", "", err
	}
	return url, state, nil
}

// CompleteOAuth finalizes the OAuth flow with the returned code/state.
func (s *Service) CompleteOAuth(ctx context.Context, provider domain.ProviderType, code, state, userAgent, ip string) (domain.TokenPair, error) {
	p, ok := s.providers[provider]
	if !ok {
		return domain.TokenPair{}, domain.ErrProviderNotConfigured
	}
	if err := s.states.Consume(ctx, state, provider); err != nil {
		return domain.TokenPair{}, err
	}
	info, err := p.Exchange(ctx, code)
	if err != nil {
		return domain.TokenPair{}, err
	}
	identity, err := s.identities.FindByProvider(ctx, provider, info.ProviderID)
	var user domain.User
	if err == nil {
		user, err = s.users.FindByID(ctx, identity.UserID)
		if err != nil {
			return domain.TokenPair{}, err
		}
	} else {
		// Fallback to email lookup to support account linking
		normalizedEmail := strings.TrimSpace(strings.ToLower(info.Email))
		user, err = s.users.FindByEmail(ctx, normalizedEmail)
		if err != nil {
			now := s.now()
			user = domain.User{
				ID:          uuid.NewString(),
				Email:       normalizedEmail,
				DisplayName: info.DisplayName,
				Status:      domain.UserStatusActive,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			user, err = s.users.Create(ctx, user)
			if err != nil {
				return domain.TokenPair{}, err
			}
		}
		now := s.now()
		identity = domain.Identity{
			ID:         uuid.NewString(),
			UserID:     user.ID,
			Provider:   provider,
			ProviderID: info.ProviderID,
			Tokens: domain.OAuthTokens{
				AccessToken:  info.AccessToken,
				RefreshToken: info.RefreshToken,
				Expiry:       info.Expiry,
				Scopes:       info.Scopes,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		if _, err := s.identities.Create(ctx, identity); err != nil {
			return domain.TokenPair{}, err
		}
	}

	plainRefresh, hashedRefresh, err := s.tokens.GenerateRefreshToken(ctx)
	if err != nil {
		return domain.TokenPair{}, err
	}
	session := domain.Session{
		ID:               uuid.NewString(),
		UserID:           user.ID,
		RefreshTokenHash: hashedRefresh,
		UserAgent:        userAgent,
		IP:               ip,
		CreatedAt:        s.now(),
		ExpiresAt:        s.now().Add(s.config.RefreshTokenTTL),
	}
	if _, err := s.sessions.Create(ctx, session); err != nil {
		return domain.TokenPair{}, err
	}
	accessToken, expiresAt, err := s.tokens.GenerateAccessToken(ctx, user, session.ID)
	if err != nil {
		return domain.TokenPair{}, err
	}
	return domain.TokenPair{
		AccessToken:   accessToken,
		AccessExpiry:  expiresAt,
		RefreshToken:  plainRefresh,
		RefreshExpiry: session.ExpiresAt,
	}, nil
}

// ParseAccessToken parses an access token and returns the associated user.
func (s *Service) ParseAccessToken(ctx context.Context, token string) (domain.Claims, error) {
	return s.tokens.ParseAccessToken(ctx, token)
}

// GetUser fetches a user by ID.
func (s *Service) GetUser(ctx context.Context, id string) (domain.User, error) {
	return s.users.FindByID(ctx, id)
}

func randomState() string {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("failed to read random bytes: %w", err))
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
