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
	Runtime            runtimeconfig.RuntimeConfig
	RuntimeMeta        runtimeconfig.Metadata
	Port               string
	EnableMCP          bool
	EnvironmentSummary string
	Auth               AuthConfig
	Session            SessionConfig
	Analytics          AnalyticsConfig
	AllowedOrigins     []string
	Attachment         attachments.StoreConfig
}

// AuthConfig captures authentication-related environment configuration.
type AuthConfig struct {
	JWTSecret             string
	AccessTokenTTLMinutes string
	RefreshTokenTTLDays   string
	StateTTLMinutes       string
	RedirectBaseURL       string
	GoogleClientID        string
	GoogleClientSecret    string
	GoogleAuthURL         string
	GoogleTokenURL        string
	GoogleUserInfoURL     string
	WeChatAppID           string
	WeChatAuthURL         string
	DatabaseURL           string
	BootstrapEmail        string
	BootstrapPassword     string
	BootstrapDisplayName  string
}

// SessionConfig captures session persistence configuration for the server (web mode).
type SessionConfig struct {
	DatabaseURL string
	Dir         string
}

// AnalyticsConfig holds analytics configuration values.
type AnalyticsConfig struct {
	PostHogAPIKey string
	PostHogHost   string
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
		Runtime:        runtimeCfg,
		RuntimeMeta:    runtimeMeta,
		Port:           "8080",
		EnableMCP:      true, // Default: enabled
		AllowedOrigins: append([]string(nil), defaultAllowedOrigins...),
		Attachment: attachments.StoreConfig{
			Provider: attachments.ProviderLocal,
			Dir:      "~/.alex-web-attachments",
		},
	}

	if port, ok := envLookup("PORT"); ok && port != "" {
		cfg.Port = port
	}

	if enableMCP, ok := envLookup("ENABLE_MCP"); ok {
		cfg.EnableMCP = enableMCP == "true" || enableMCP == "1"
	}

	if origins, ok := envLookup("CORS_ALLOWED_ORIGINS"); ok {
		cfg.AllowedOrigins = parseAllowedOrigins(origins)
	}

	if cfg.Runtime.APIKey == "" && cfg.Runtime.LLMProvider != "ollama" && cfg.Runtime.LLMProvider != "mock" {
		return Config{}, nil, nil, fmt.Errorf("API key required for provider '%s'", cfg.Runtime.LLMProvider)
	}

	authCfg := AuthConfig{}
	if secret, ok := envLookup("AUTH_JWT_SECRET"); ok {
		authCfg.JWTSecret = strings.TrimSpace(secret)
	}
	if ttl, ok := envLookup("AUTH_ACCESS_TOKEN_TTL_MINUTES"); ok {
		authCfg.AccessTokenTTLMinutes = strings.TrimSpace(ttl)
	}
	if ttl, ok := envLookup("AUTH_REFRESH_TOKEN_TTL_DAYS"); ok {
		authCfg.RefreshTokenTTLDays = strings.TrimSpace(ttl)
	}
	if ttl, ok := envLookup("AUTH_STATE_TTL_MINUTES"); ok {
		authCfg.StateTTLMinutes = strings.TrimSpace(ttl)
	}
	if redirect, ok := envLookup("AUTH_REDIRECT_BASE_URL"); ok {
		authCfg.RedirectBaseURL = strings.TrimSpace(redirect)
	}
	if clientID, ok := envLookup("GOOGLE_CLIENT_ID"); ok {
		authCfg.GoogleClientID = strings.TrimSpace(clientID)
	}
	if clientSecret, ok := envLookup("GOOGLE_CLIENT_SECRET"); ok {
		authCfg.GoogleClientSecret = strings.TrimSpace(clientSecret)
	}
	if authURL, ok := envLookup("GOOGLE_AUTH_URL"); ok {
		authCfg.GoogleAuthURL = strings.TrimSpace(authURL)
	}
	if tokenURL, ok := envLookup("GOOGLE_TOKEN_URL"); ok {
		authCfg.GoogleTokenURL = strings.TrimSpace(tokenURL)
	}
	if userInfoURL, ok := envLookup("GOOGLE_USERINFO_URL"); ok {
		authCfg.GoogleUserInfoURL = strings.TrimSpace(userInfoURL)
	}
	if appID, ok := envLookup("WECHAT_APP_ID"); ok {
		authCfg.WeChatAppID = strings.TrimSpace(appID)
	}
	if authURL, ok := envLookup("WECHAT_AUTH_URL"); ok {
		authCfg.WeChatAuthURL = strings.TrimSpace(authURL)
	}
	if dbURL, ok := envLookup("AUTH_DATABASE_URL"); ok {
		authCfg.DatabaseURL = strings.TrimSpace(dbURL)
	}
	if email, ok := envLookup("AUTH_BOOTSTRAP_EMAIL"); ok {
		authCfg.BootstrapEmail = strings.TrimSpace(email)
	}
	if password, ok := envLookup("AUTH_BOOTSTRAP_PASSWORD"); ok {
		authCfg.BootstrapPassword = password
	}
	if name, ok := envLookup("AUTH_BOOTSTRAP_DISPLAY_NAME"); ok {
		authCfg.BootstrapDisplayName = strings.TrimSpace(name)
	}
	cfg.Auth = authCfg

	sessionCfg := SessionConfig{Dir: "~/.alex-web-sessions"}
	if dbURL, ok := envLookup("ALEX_SESSION_DATABASE_URL"); ok {
		sessionCfg.DatabaseURL = strings.TrimSpace(dbURL)
	}
	if dir, ok := envLookup("ALEX_WEB_SESSION_DIR"); ok && strings.TrimSpace(dir) != "" {
		sessionCfg.Dir = strings.TrimSpace(dir)
	} else if dir, ok := envLookup("SESSION_STORE_PATH"); ok && strings.TrimSpace(dir) != "" {
		sessionCfg.Dir = strings.TrimSpace(dir)
	}
	cfg.Session = sessionCfg

	analyticsCfg := AnalyticsConfig{}
	if apiKey, ok := envLookup("POSTHOG_API_KEY"); ok {
		analyticsCfg.PostHogAPIKey = strings.TrimSpace(apiKey)
	}
	if host, ok := envLookup("POSTHOG_HOST"); ok {
		analyticsCfg.PostHogHost = strings.TrimSpace(host)
	}
	cfg.Analytics = analyticsCfg

	if provider, ok := envLookup("ALEX_ATTACHMENT_PROVIDER"); ok {
		cfg.Attachment.Provider = strings.TrimSpace(provider)
	}
	if dir, ok := envLookup("ALEX_WEB_ATTACHMENT_DIR"); ok && strings.TrimSpace(dir) != "" {
		cfg.Attachment.Dir = strings.TrimSpace(dir)
	}
	if accountID, ok := envLookup("CLOUDFLARE_ACCOUNT_ID"); ok {
		cfg.Attachment.CloudflareAccountID = strings.TrimSpace(accountID)
	}
	if accessKey, ok := envLookup("CLOUDFLARE_ACCESS_KEY_ID"); ok {
		cfg.Attachment.CloudflareAccessKeyID = strings.TrimSpace(accessKey)
	}
	if secret, ok := envLookup("CLOUDFLARE_SECRET_ACCESS_KEY"); ok {
		cfg.Attachment.CloudflareSecretAccessKey = strings.TrimSpace(secret)
	}
	if bucket, ok := envLookup("CLOUDFLARE_BUCKET"); ok {
		cfg.Attachment.CloudflareBucket = strings.TrimSpace(bucket)
	}
	if base, ok := envLookup("CLOUDFLARE_PUBLIC_BASE_URL"); ok {
		cfg.Attachment.CloudflarePublicBaseURL = strings.TrimSpace(base)
	}
	if prefix, ok := envLookup("CLOUDFLARE_ATTACHMENT_KEY_PREFIX"); ok {
		cfg.Attachment.CloudflareKeyPrefix = strings.TrimSpace(prefix)
	}
	if ttlRaw, ok := envLookup("ALEX_ATTACHMENT_PRESIGN_TTL"); ok && strings.TrimSpace(ttlRaw) != "" {
		if parsed, err := time.ParseDuration(strings.TrimSpace(ttlRaw)); err == nil && parsed > 0 {
			cfg.Attachment.PresignTTL = parsed
		}
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

func parseAllowedOrigins(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\r', '\t':
			return true
		default:
			return false
		}
	})
	origins := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		origin := strings.TrimSpace(field)
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
