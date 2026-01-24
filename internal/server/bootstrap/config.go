package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/attachments"
	runtimeconfig "alex/internal/config"
	configadmin "alex/internal/config/admin"
)

// Config holds server configuration.
type Config struct {
	Runtime                    runtimeconfig.RuntimeConfig
	RuntimeMeta                runtimeconfig.Metadata
	Port                       string
	EnableMCP                  bool
	EnvironmentSummary         string
	Auth                       runtimeconfig.AuthConfig
	Session                    runtimeconfig.SessionConfig
	Analytics                  runtimeconfig.AnalyticsConfig
	AllowedOrigins             []string
	MaxTaskBodyBytes           int64
	StreamMaxDuration          time.Duration
	StreamMaxBytes             int64
	StreamMaxConcurrent        int
	RateLimitRequestsPerMinute int
	RateLimitBurst             int
	NonStreamTimeout           time.Duration
	Attachment                 attachments.StoreConfig
}

var defaultAllowedOrigins = []string{
	"http://localhost:3000",
	"http://localhost:3001",
	"https://alex.yourdomain.com",
}

func LoadConfig() (Config, *configadmin.Manager, func(context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error), error) {
	envLookup := runtimeconfig.DefaultEnvLookup

	storePath := configadmin.ResolveStorePath(envLookup)
	cacheTTL := 30 * time.Second
	if ttlValue, ok := envLookup("CONFIG_ADMIN_CACHE_TTL"); ok && strings.TrimSpace(ttlValue) != "" {
		if parsed, err := time.ParseDuration(strings.TrimSpace(ttlValue)); err == nil && parsed > 0 {
			cacheTTL = parsed
		}
	}
	ctx := context.Background()
	store := configadmin.NewFileStore(storePath)
	managedOverrides, err := store.LoadOverrides(ctx)
	if err != nil {
		return Config{}, nil, nil, err
	}
	manager := configadmin.NewManager(store, managedOverrides, configadmin.WithCacheTTL(cacheTTL))

	runtimeCfg, runtimeMeta, err := runtimeconfig.Load(
		runtimeconfig.WithEnv(envLookup),
		runtimeconfig.WithOverrides(managedOverrides),
	)
	if err != nil {
		return Config{}, nil, nil, err
	}

	cfg := Config{
		Runtime:                    runtimeCfg,
		RuntimeMeta:                runtimeMeta,
		Port:                       "8080",
		EnableMCP:                  true, // Default: enabled
		AllowedOrigins:             append([]string(nil), defaultAllowedOrigins...),
		StreamMaxDuration:          2 * time.Hour,
		StreamMaxBytes:             64 * 1024 * 1024,
		StreamMaxConcurrent:        128,
		RateLimitRequestsPerMinute: 600,
		RateLimitBurst:             120,
		NonStreamTimeout:           30 * time.Second,
		Session: runtimeconfig.SessionConfig{
			Dir: "~/.alex-web-sessions",
		},
		Attachment: attachments.StoreConfig{
			Provider: attachments.ProviderLocal,
			Dir:      "~/.alex-web-attachments",
		},
	}

	fileCfg, _, err := runtimeconfig.LoadFileConfig(runtimeconfig.WithEnv(envLookup))
	if err != nil {
		return Config{}, nil, nil, err
	}
	applyServerFileConfig(&cfg, fileCfg)

	if cfg.Runtime.APIKey == "" && cfg.Runtime.LLMProvider != "ollama" && cfg.Runtime.LLMProvider != "mock" {
		return Config{}, nil, nil, fmt.Errorf("API key required for provider '%s'", cfg.Runtime.LLMProvider)
	}

	resolver := func(ctx context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error) {
		if ctx == nil {
			ctx = context.Background()
		}
		overrides, err := manager.CurrentOverrides(ctx)
		if err != nil {
			return runtimeconfig.RuntimeConfig{}, runtimeconfig.Metadata{}, err
		}
		return runtimeconfig.Load(
			runtimeconfig.WithEnv(envLookup),
			runtimeconfig.WithOverrides(overrides),
		)
	}

	return cfg, manager, resolver, nil
}

func applyServerFileConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if cfg == nil {
		return
	}

	if file.Server != nil {
		if port := strings.TrimSpace(file.Server.Port); port != "" {
			cfg.Port = port
		}
		if file.Server.EnableMCP != nil {
			cfg.EnableMCP = *file.Server.EnableMCP
		}
		if file.Server.MaxTaskBodyBytes != nil && *file.Server.MaxTaskBodyBytes > 0 {
			cfg.MaxTaskBodyBytes = *file.Server.MaxTaskBodyBytes
		}
		if file.Server.StreamMaxDurationSeconds != nil && *file.Server.StreamMaxDurationSeconds > 0 {
			cfg.StreamMaxDuration = time.Duration(*file.Server.StreamMaxDurationSeconds) * time.Second
		}
		if file.Server.StreamMaxBytes != nil && *file.Server.StreamMaxBytes > 0 {
			cfg.StreamMaxBytes = *file.Server.StreamMaxBytes
		}
		if file.Server.StreamMaxConcurrent != nil && *file.Server.StreamMaxConcurrent > 0 {
			cfg.StreamMaxConcurrent = *file.Server.StreamMaxConcurrent
		}
		if file.Server.RateLimitRequestsPerMinute != nil && *file.Server.RateLimitRequestsPerMinute > 0 {
			cfg.RateLimitRequestsPerMinute = *file.Server.RateLimitRequestsPerMinute
		}
		if file.Server.RateLimitBurst != nil && *file.Server.RateLimitBurst > 0 {
			cfg.RateLimitBurst = *file.Server.RateLimitBurst
		}
		if file.Server.NonStreamTimeoutSeconds != nil && *file.Server.NonStreamTimeoutSeconds > 0 {
			cfg.NonStreamTimeout = time.Duration(*file.Server.NonStreamTimeoutSeconds) * time.Second
		}
		if file.Server.AllowedOrigins != nil {
			cfg.AllowedOrigins = normalizeAllowedOrigins(file.Server.AllowedOrigins)
		}
	}

	if file.Auth != nil {
		cfg.Auth = runtimeconfig.AuthConfig{
			JWTSecret:             strings.TrimSpace(file.Auth.JWTSecret),
			AccessTokenTTLMinutes: strings.TrimSpace(file.Auth.AccessTokenTTLMinutes),
			RefreshTokenTTLDays:   strings.TrimSpace(file.Auth.RefreshTokenTTLDays),
			StateTTLMinutes:       strings.TrimSpace(file.Auth.StateTTLMinutes),
			RedirectBaseURL:       strings.TrimSpace(file.Auth.RedirectBaseURL),
			GoogleClientID:        strings.TrimSpace(file.Auth.GoogleClientID),
			GoogleClientSecret:    strings.TrimSpace(file.Auth.GoogleClientSecret),
			GoogleAuthURL:         strings.TrimSpace(file.Auth.GoogleAuthURL),
			GoogleTokenURL:        strings.TrimSpace(file.Auth.GoogleTokenURL),
			GoogleUserInfoURL:     strings.TrimSpace(file.Auth.GoogleUserInfoURL),
			WeChatAppID:           strings.TrimSpace(file.Auth.WeChatAppID),
			WeChatAuthURL:         strings.TrimSpace(file.Auth.WeChatAuthURL),
			DatabaseURL:           strings.TrimSpace(file.Auth.DatabaseURL),
			BootstrapEmail:        strings.TrimSpace(file.Auth.BootstrapEmail),
			BootstrapPassword:     file.Auth.BootstrapPassword,
			BootstrapDisplayName:  strings.TrimSpace(file.Auth.BootstrapDisplayName),
		}
	}

	if file.Session != nil {
		if dbURL := strings.TrimSpace(file.Session.DatabaseURL); dbURL != "" {
			cfg.Session.DatabaseURL = dbURL
		}
		if dir := strings.TrimSpace(file.Session.Dir); dir != "" {
			cfg.Session.Dir = dir
		}
		if file.Session.PoolMaxConns != nil {
			cfg.Session.PoolMaxConns = file.Session.PoolMaxConns
		}
		if file.Session.PoolMinConns != nil {
			cfg.Session.PoolMinConns = file.Session.PoolMinConns
		}
		if file.Session.PoolMaxConnLifetimeSeconds != nil {
			cfg.Session.PoolMaxConnLifetimeSeconds = file.Session.PoolMaxConnLifetimeSeconds
		}
		if file.Session.PoolMaxConnIdleSeconds != nil {
			cfg.Session.PoolMaxConnIdleSeconds = file.Session.PoolMaxConnIdleSeconds
		}
		if file.Session.PoolHealthCheckSeconds != nil {
			cfg.Session.PoolHealthCheckSeconds = file.Session.PoolHealthCheckSeconds
		}
		if file.Session.PoolConnectTimeoutSeconds != nil {
			cfg.Session.PoolConnectTimeoutSeconds = file.Session.PoolConnectTimeoutSeconds
		}
	}

	if file.Analytics != nil {
		cfg.Analytics = runtimeconfig.AnalyticsConfig{
			PostHogAPIKey: strings.TrimSpace(file.Analytics.PostHogAPIKey),
			PostHogHost:   strings.TrimSpace(file.Analytics.PostHogHost),
		}
	}

	if file.Attachments != nil {
		if provider := strings.TrimSpace(file.Attachments.Provider); provider != "" {
			cfg.Attachment.Provider = provider
		}
		if dir := strings.TrimSpace(file.Attachments.Dir); dir != "" {
			cfg.Attachment.Dir = dir
		}
		if accountID := strings.TrimSpace(file.Attachments.CloudflareAccountID); accountID != "" {
			cfg.Attachment.CloudflareAccountID = accountID
		}
		if accessKey := strings.TrimSpace(file.Attachments.CloudflareAccessKeyID); accessKey != "" {
			cfg.Attachment.CloudflareAccessKeyID = accessKey
		}
		if secret := strings.TrimSpace(file.Attachments.CloudflareSecretAccessKey); secret != "" {
			cfg.Attachment.CloudflareSecretAccessKey = secret
		}
		if bucket := strings.TrimSpace(file.Attachments.CloudflareBucket); bucket != "" {
			cfg.Attachment.CloudflareBucket = bucket
		}
		if base := strings.TrimSpace(file.Attachments.CloudflarePublicBaseURL); base != "" {
			cfg.Attachment.CloudflarePublicBaseURL = base
		}
		if prefix := strings.TrimSpace(file.Attachments.CloudflareKeyPrefix); prefix != "" {
			cfg.Attachment.CloudflareKeyPrefix = prefix
		}
		if ttlRaw := strings.TrimSpace(file.Attachments.PresignTTL); ttlRaw != "" {
			if parsed, err := time.ParseDuration(ttlRaw); err == nil && parsed > 0 {
				cfg.Attachment.PresignTTL = parsed
			}
		}
	}
}

func normalizeAllowedOrigins(values []string) []string {
	if values == nil {
		return nil
	}
	origins := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		origin := strings.TrimSpace(value)
		if origin == "" {
			continue
		}
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	return origins
}
