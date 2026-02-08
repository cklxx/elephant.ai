package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	authapp "alex/internal/app/auth"
	authdomain "alex/internal/domain/auth"
	authports "alex/internal/domain/auth/ports"
	authAdapters "alex/internal/infra/auth/adapters"
	"alex/internal/shared/async"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
	"github.com/jackc/pgx/v5/pgxpool"
)

func BuildAuthService(cfg Config, logger logging.Logger) (*authapp.Service, func(), error) {
	logger = logging.OrNop(logger)
	runtimeCfg := cfg.Runtime
	authCfg := cfg.Auth
	allowDevelopmentFallback := isAuthDevelopmentFallbackEnvironment(runtimeCfg.Environment)

	secret := strings.TrimSpace(authCfg.JWTSecret)
	if secret == "" {
		if allowDevelopmentFallback {
			secret = "dev-secret-change-me"
			logger.Warn("auth.jwt_secret not configured; using development fallback secret")
		} else {
			return nil, nil, fmt.Errorf("auth.jwt_secret not configured")
		}
	}

	accessTTL := 15 * time.Minute
	if minutes := strings.TrimSpace(authCfg.AccessTokenTTLMinutes); minutes != "" {
		if v, err := strconv.Atoi(minutes); err == nil && v > 0 {
			accessTTL = time.Duration(v) * time.Minute
		} else if err != nil {
			logger.Warn("Invalid auth.access_token_ttl_minutes value: %v", err)
		}
	}

	refreshTTL := 30 * 24 * time.Hour
	if days := strings.TrimSpace(authCfg.RefreshTokenTTLDays); days != "" {
		if v, err := strconv.Atoi(days); err == nil && v > 0 {
			refreshTTL = time.Duration(v) * 24 * time.Hour
		} else if err != nil {
			logger.Warn("Invalid auth.refresh_token_ttl_days value: %v", err)
		}
	}

	stateTTL := 10 * time.Minute
	if minutes := strings.TrimSpace(authCfg.StateTTLMinutes); minutes != "" {
		if v, err := strconv.Atoi(minutes); err == nil && v > 0 {
			stateTTL = time.Duration(v) * time.Minute
		} else if err != nil {
			logger.Warn("Invalid auth.state_ttl_minutes value: %v", err)
		}
	}

	memUsers, memIdentities, memSessions, memStates := authAdapters.NewMemoryStores()
	var (
		users      authports.UserRepository     = memUsers
		identities authports.IdentityRepository = memIdentities
		sessions   authports.SessionRepository  = memSessions
		states     authports.StateStore         = memStates
	)
	tokenManager := authAdapters.NewJWTTokenManager(secret, "alex-server", accessTTL)

	var cleanupFuncs []func()

	if dbURL := strings.TrimSpace(authCfg.DatabaseURL); dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		poolCfg, err := pgxpool.ParseConfig(dbURL)
		if err != nil {
			if !allowDevelopmentFallback {
				return nil, nil, fmt.Errorf("parse auth db pool config: %w", err)
			}
			logger.Warn("Auth DB pool config parse failed; falling back to memory stores: %v", err)
		} else {
			maxConns := int32(4)
			if authCfg.DatabasePoolMaxConns != nil && *authCfg.DatabasePoolMaxConns > 0 {
				maxConns = int32(*authCfg.DatabasePoolMaxConns)
			}
			poolCfg.MaxConns = maxConns

			pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
			if err != nil {
				if !allowDevelopmentFallback {
					return nil, nil, fmt.Errorf("create auth db pool: %w", err)
				}
				logger.Warn("Auth DB pool init failed; falling back to memory stores: %v", err)
			} else {
				if err := pool.Ping(ctx); err != nil {
					pool.Close()
					if !allowDevelopmentFallback {
						return nil, nil, fmt.Errorf("ping auth db: %w", err)
					}
					logger.Warn("Auth DB ping failed; falling back to memory stores: %v", err)
				} else {
					usersRepo, identitiesRepo, sessionsRepo, statesRepo := authAdapters.NewPostgresStores(pool)
					users = usersRepo
					identities = identitiesRepo
					sessions = sessionsRepo
					states = statesRepo
					cleanupFuncs = append(cleanupFuncs, func() {
						pool.Close()
					})
					logger.Info("Authentication repositories backed by Postgres (max_conns=%d)", maxConns)
				}
			}
		}
	}

	redirectBase := strings.TrimSpace(authCfg.RedirectBaseURL)
	if redirectBase == "" {
		port := strings.TrimPrefix(cfg.Port, ":")
		redirectBase = fmt.Sprintf("http://localhost:%s", port)
	}
	if !strings.HasPrefix(redirectBase, "http://") && !strings.HasPrefix(redirectBase, "https://") {
		redirectBase = "https://" + redirectBase
	}
	trimmedBase := strings.TrimRight(redirectBase, "/")

	googleAuthURL := strings.TrimSpace(authCfg.GoogleAuthURL)
	if googleAuthURL == "" {
		googleAuthURL = "https://accounts.google.com/o/oauth2/v2/auth"
	}
	googleTokenURL := strings.TrimSpace(authCfg.GoogleTokenURL)
	if googleTokenURL == "" {
		googleTokenURL = "https://oauth2.googleapis.com/token"
	}
	googleUserInfoURL := strings.TrimSpace(authCfg.GoogleUserInfoURL)
	if googleUserInfoURL == "" {
		googleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
	}
	providers := []authports.OAuthProvider{}
	if clientID := strings.TrimSpace(authCfg.GoogleClientID); clientID != "" {
		secret := strings.TrimSpace(authCfg.GoogleClientSecret)
		if secret == "" {
			logger.Warn("Google OAuth client secret not configured; Google login disabled")
		} else {
			providers = append(providers, authAdapters.NewGoogleOAuthProvider(authAdapters.GoogleOAuthConfig{
				ClientID:     clientID,
				ClientSecret: secret,
				AuthURL:      googleAuthURL,
				TokenURL:     googleTokenURL,
				UserInfoURL:  googleUserInfoURL,
				RedirectURL:  trimmedBase + "/api/auth/google/callback",
			}))
		}
	}
	service := authapp.NewService(users, identities, sessions, tokenManager, states, providers, authapp.Config{
		AccessTokenTTL:        accessTTL,
		RefreshTokenTTL:       refreshTTL,
		StateTTL:              stateTTL,
		RedirectBaseURL:       trimmedBase,
		SecureCookies:         strings.EqualFold(runtimeCfg.Environment, "production"),
		AllowedCallbackDomain: runtimeCfg.Environment,
	})

	if cleaner, ok := states.(authports.StateStoreWithCleanup); ok {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		interval := time.Minute

		runCleanup := func() {
			purgeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			removed, err := cleaner.PurgeExpired(purgeCtx, time.Now().UTC())
			if err != nil {
				logger.Warn("Failed to purge expired auth states: %v", err)
				return
			}
			if removed > 0 {
				logger.Info("Purged %d expired auth states", removed)
			} else {
				logger.Debug("No expired auth states to purge")
			}
		}

		async.Go(logger, "server.authStateCleanup", func() {
			ticker := time.NewTicker(interval)
			defer func() {
				ticker.Stop()
				close(done)
			}()
			runCleanup()
			for {
				select {
				case <-ticker.C:
					runCleanup()
				case <-ctx.Done():
					return
				}
			}
		})

		cleanupFuncs = append(cleanupFuncs, func() {
			cancel()
			<-done
		})
	}

	cleanup := func() {
		for i := len(cleanupFuncs) - 1; i >= 0; i-- {
			if cleanupFuncs[i] != nil {
				cleanupFuncs[i]()
			}
		}
	}

	if err := bootstrapAuthUser(service, authCfg, logger); err != nil {
		cleanup()
		return nil, nil, err
	}

	return service, cleanup, nil
}

func isAuthDevelopmentFallbackEnvironment(environment string) bool {
	env := strings.TrimSpace(environment)
	return strings.EqualFold(env, "development") ||
		strings.EqualFold(env, "dev") ||
		strings.EqualFold(env, "internal") ||
		strings.EqualFold(env, "evaluation") ||
		env == ""
}

func bootstrapAuthUser(service *authapp.Service, cfg runtimeconfig.AuthConfig, logger logging.Logger) error {
	logger = logging.OrNop(logger)
	email := strings.TrimSpace(cfg.BootstrapEmail)
	password := strings.TrimSpace(cfg.BootstrapPassword)
	if email == "" || password == "" {
		return nil
	}
	displayName := strings.TrimSpace(cfg.BootstrapDisplayName)
	if displayName == "" {
		displayName = "Admin"
	}

	_, err := service.RegisterLocal(context.Background(), email, password, displayName)
	if err != nil {
		if errors.Is(err, authdomain.ErrUserExists) {
			logger.Info("Bootstrap auth user already exists: %s", email)
			return nil
		}
		return fmt.Errorf("bootstrap auth user %s: %w", email, err)
	}

	logger.Info("Bootstrap auth user created: %s", email)
	return nil
}
