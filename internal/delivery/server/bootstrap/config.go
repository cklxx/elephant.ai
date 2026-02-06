package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/delivery/channels"
	"alex/internal/infra/attachments"
	"alex/internal/shared/agent/presets"
	runtimeconfig "alex/internal/shared/config"
	configadmin "alex/internal/shared/config/admin"
)

// Config holds server configuration.
type Config struct {
	Runtime                        runtimeconfig.RuntimeConfig
	RuntimeMeta                    runtimeconfig.Metadata
	Port                           string
	EnableMCP                      bool
	EnvironmentSummary             string
	Auth                           runtimeconfig.AuthConfig
	Session                        runtimeconfig.SessionConfig
	Analytics                      runtimeconfig.AnalyticsConfig
	Channels                       ChannelsConfig
	AllowedOrigins                 []string
	MaxTaskBodyBytes               int64
	StreamMaxDuration              time.Duration
	StreamMaxBytes                 int64
	StreamMaxConcurrent            int
	RateLimitRequestsPerMinute     int
	RateLimitBurst                 int
	NonStreamTimeout               time.Duration
	EventHistoryRetention          time.Duration
	EventHistoryMaxSessions        int
	EventHistorySessionTTL         time.Duration
	EventHistoryMaxEvents          int
	EventHistoryAsyncBatchSize     int
	EventHistoryAsyncFlushInterval time.Duration
	EventHistoryAsyncAppendTimeout time.Duration
	EventHistoryAsyncQueueCapacity int
	Attachment                     attachments.StoreConfig
}

// ChannelsConfig captures server-side channel gateways.
type ChannelsConfig struct {
	Lark LarkGatewayConfig
}

// LarkGatewayConfig captures the resolved Lark gateway configuration.
type LarkGatewayConfig struct {
	channels.BaseConfig
	Enabled                       bool
	AppID                         string
	AppSecret                     string
	TenantCalendarID              string
	BaseDomain                    string
	WorkspaceDir                  string
	CardsEnabled                  bool
	CardsPlanReview               bool
	CardsResults                  bool
	CardsErrors                   bool
	CardCallbackVerificationToken string
	CardCallbackEncryptKey        string
	AutoUploadFiles               bool
	AutoUploadMaxBytes            int
	AutoUploadAllowExt            []string
	Browser                       LarkBrowserConfig
	ToolMode                      string
	ReactEmoji                    string
	InjectionAckReactEmoji        string
	FinalAnswerReviewReactEmoji   string
	ShowToolProgress              bool
	ShowPlanClarifyMessages       bool
	AutoChatContextSize           int
	PlanReviewEnabled             bool
	PlanReviewRequireConfirmation bool
	PlanReviewPendingTTL          time.Duration
}

// LarkBrowserConfig captures local browser settings for Lark.
type LarkBrowserConfig struct {
	CDPURL      string
	ChromePath  string
	Headless    bool
	UserDataDir string
	Timeout     time.Duration
}

var defaultAllowedOrigins = []string{
	"http://localhost:3000",
	"http://localhost:3001",
	"https://alex.yourdomain.com",
}

func LoadConfig() (Config, *configadmin.Manager, func(context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error), *runtimeconfig.RuntimeConfigCache, error) {
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
		return Config{}, nil, nil, nil, err
	}
	manager := configadmin.NewManager(store, managedOverrides, configadmin.WithCacheTTL(cacheTTL))

	loader := func(ctx context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error) {
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
	runtimeCache, err := runtimeconfig.NewRuntimeConfigCache(loader)
	if err != nil {
		return Config{}, nil, nil, nil, err
	}
	runtimeCfg, runtimeMeta, err := runtimeCache.Resolve(context.Background())
	if err != nil {
		return Config{}, nil, nil, nil, err
	}

	cfg := Config{
		Runtime:                        runtimeCfg,
		RuntimeMeta:                    runtimeMeta,
		Port:                           "8080",
		EnableMCP:                      true, // Default: enabled
		AllowedOrigins:                 append([]string(nil), defaultAllowedOrigins...),
		StreamMaxDuration:              2 * time.Hour,
		StreamMaxBytes:                 64 * 1024 * 1024,
		StreamMaxConcurrent:            128,
		RateLimitRequestsPerMinute:     600,
		RateLimitBurst:                 120,
		NonStreamTimeout:               30 * time.Second,
		EventHistoryRetention:          30 * 24 * time.Hour,
		EventHistoryMaxSessions:        100,
		EventHistorySessionTTL:         1 * time.Hour,
		EventHistoryMaxEvents:          1000,
		EventHistoryAsyncBatchSize:     200,
		EventHistoryAsyncFlushInterval: 250 * time.Millisecond,
		EventHistoryAsyncAppendTimeout: 50 * time.Millisecond,
		EventHistoryAsyncQueueCapacity: 8192,
		Session: runtimeconfig.SessionConfig{
			Dir: "~/.alex/sessions",
		},
		Channels: ChannelsConfig{
			Lark: LarkGatewayConfig{
				BaseConfig: channels.BaseConfig{
					SessionPrefix: "lark",
					AllowGroups:   true,
					AllowDirect:   true,
					AgentPreset:   string(presets.PresetDefault),
					ToolPreset:    string(presets.ToolPresetLarkLocal),
					ReplyTimeout:  3 * time.Minute,
				},
				BaseDomain:         "https://open.larkoffice.com",
				ToolMode:           "cli",
				AutoUploadFiles:    true,
				AutoUploadMaxBytes: 2 * 1024 * 1024,
				AutoUploadAllowExt: []string{".txt", ".md", ".json", ".yaml", ".yml", ".csv", ".log", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".pdf", ".docx", ".xlsx", ".pptx"},
				Browser: LarkBrowserConfig{
					Headless: true,
					Timeout:  60 * time.Second,
				},
				ReactEmoji:          "WAVE, Get, THINKING, MUSCLE, THUMBSUP, OK, THANKS, APPLAUSE, LGTM",
				AutoChatContextSize: 20,
				CardsEnabled:        true,
				CardsPlanReview:     false,
				CardsResults:        false,
				CardsErrors:         true,
			},
		},
		Attachment: attachments.StoreConfig{
			Provider: attachments.ProviderLocal,
			Dir:      "~/.alex/attachments",
		},
	}

	fileCfg, _, err := runtimeconfig.LoadFileConfig(runtimeconfig.WithEnv(envLookup))
	if err != nil {
		return Config{}, nil, nil, nil, err
	}
	applyServerFileConfig(&cfg, fileCfg)
	applyLarkEnvFallback(&cfg, envLookup)

	providerLower := strings.ToLower(strings.TrimSpace(cfg.Runtime.LLMProvider))
	if cfg.Runtime.APIKey == "" && providerLower != "ollama" && providerLower != "mock" && providerLower != "llama.cpp" && providerLower != "llamacpp" && providerLower != "llama-cpp" {
		return Config{}, nil, nil, nil, fmt.Errorf("API key required for provider '%s'", cfg.Runtime.LLMProvider)
	}

	resolver := runtimeCache.Resolve

	return cfg, manager, resolver, runtimeCache, nil
}

func applyServerFileConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if cfg == nil {
		return
	}
	applyLarkConfig(cfg, file)
	applyServerHTTPConfig(cfg, file)
	applyAuthConfig(cfg, file)
	applySessionConfig(cfg, file)
	applyAnalyticsConfig(cfg, file)
	applyAttachmentConfig(cfg, file)
}

func applyLarkEnvFallback(cfg *Config, lookup runtimeconfig.EnvLookup) {
	if cfg == nil {
		return
	}
	if lookup == nil {
		lookup = runtimeconfig.DefaultEnvLookup
	}

	if strings.TrimSpace(cfg.Channels.Lark.CardCallbackVerificationToken) == "" {
		if token := lookupFirstNonEmptyEnv(
			lookup,
			"LARK_CARD_CALLBACK_VERIFICATION_TOKEN",
			"LARK_VERIFICATION_TOKEN",
			"FEISHU_CARD_CALLBACK_VERIFICATION_TOKEN",
			"FEISHU_VERIFICATION_TOKEN",
			"CARD_CALLBACK_VERIFICATION_TOKEN",
		); token != "" {
			cfg.Channels.Lark.CardCallbackVerificationToken = token
		}
	}
	if strings.TrimSpace(cfg.Channels.Lark.CardCallbackEncryptKey) == "" {
		if key := lookupFirstNonEmptyEnv(
			lookup,
			"LARK_CARD_CALLBACK_ENCRYPT_KEY",
			"LARK_ENCRYPT_KEY",
			"FEISHU_CARD_CALLBACK_ENCRYPT_KEY",
			"FEISHU_ENCRYPT_KEY",
			"CARD_CALLBACK_ENCRYPT_KEY",
		); key != "" {
			cfg.Channels.Lark.CardCallbackEncryptKey = key
		}
	}
}

func lookupFirstNonEmptyEnv(lookup runtimeconfig.EnvLookup, keys ...string) string {
	if lookup == nil {
		return ""
	}
	for _, key := range keys {
		if value, ok := lookup(key); ok {
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func applyLarkConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Channels == nil || file.Channels.Lark == nil {
		return
	}
	larkCfg := file.Channels.Lark
	if larkCfg.Enabled != nil {
		cfg.Channels.Lark.Enabled = *larkCfg.Enabled
	}
	if appID := strings.TrimSpace(larkCfg.AppID); appID != "" {
		cfg.Channels.Lark.AppID = appID
	}
	if appSecret := strings.TrimSpace(larkCfg.AppSecret); appSecret != "" {
		cfg.Channels.Lark.AppSecret = appSecret
	}
	if calendarID := strings.TrimSpace(larkCfg.TenantCalendarID); calendarID != "" {
		cfg.Channels.Lark.TenantCalendarID = calendarID
	}
	if baseDomain := strings.TrimSpace(larkCfg.BaseDomain); baseDomain != "" {
		cfg.Channels.Lark.BaseDomain = baseDomain
	}
	if workspaceDir := strings.TrimSpace(larkCfg.WorkspaceDir); workspaceDir != "" {
		cfg.Channels.Lark.WorkspaceDir = workspaceDir
	}
	if larkCfg.CardsEnabled != nil {
		cfg.Channels.Lark.CardsEnabled = *larkCfg.CardsEnabled
	}
	if larkCfg.CardsPlanReview != nil {
		cfg.Channels.Lark.CardsPlanReview = *larkCfg.CardsPlanReview
	}
	if larkCfg.CardsResults != nil {
		cfg.Channels.Lark.CardsResults = *larkCfg.CardsResults
	}
	if larkCfg.CardsErrors != nil {
		cfg.Channels.Lark.CardsErrors = *larkCfg.CardsErrors
	}
	if token := strings.TrimSpace(larkCfg.CardCallbackVerificationToken); token != "" {
		cfg.Channels.Lark.CardCallbackVerificationToken = token
	}
	if encryptKey := strings.TrimSpace(larkCfg.CardCallbackEncryptKey); encryptKey != "" {
		cfg.Channels.Lark.CardCallbackEncryptKey = encryptKey
	}
	if larkCfg.AutoUploadFiles != nil {
		cfg.Channels.Lark.AutoUploadFiles = *larkCfg.AutoUploadFiles
	}
	if larkCfg.AutoUploadMaxBytes != nil && *larkCfg.AutoUploadMaxBytes > 0 {
		cfg.Channels.Lark.AutoUploadMaxBytes = *larkCfg.AutoUploadMaxBytes
	}
	if len(larkCfg.AutoUploadAllowExt) > 0 {
		cfg.Channels.Lark.AutoUploadAllowExt = append([]string(nil), larkCfg.AutoUploadAllowExt...)
	}
	if larkCfg.Browser != nil {
		if cdpURL := strings.TrimSpace(larkCfg.Browser.CDPURL); cdpURL != "" {
			cfg.Channels.Lark.Browser.CDPURL = cdpURL
		}
		if chromePath := strings.TrimSpace(larkCfg.Browser.ChromePath); chromePath != "" {
			cfg.Channels.Lark.Browser.ChromePath = chromePath
		}
		if larkCfg.Browser.Headless != nil {
			cfg.Channels.Lark.Browser.Headless = *larkCfg.Browser.Headless
		}
		if userDataDir := strings.TrimSpace(larkCfg.Browser.UserDataDir); userDataDir != "" {
			cfg.Channels.Lark.Browser.UserDataDir = userDataDir
		}
		if larkCfg.Browser.TimeoutSeconds != nil && *larkCfg.Browser.TimeoutSeconds > 0 {
			cfg.Channels.Lark.Browser.Timeout = time.Duration(*larkCfg.Browser.TimeoutSeconds) * time.Second
		}
	}
	if prefix := strings.TrimSpace(larkCfg.SessionPrefix); prefix != "" {
		cfg.Channels.Lark.SessionPrefix = prefix
	}
	if replyPrefix := strings.TrimSpace(larkCfg.ReplyPrefix); replyPrefix != "" {
		cfg.Channels.Lark.ReplyPrefix = replyPrefix
	}
	if larkCfg.AllowGroups != nil {
		cfg.Channels.Lark.AllowGroups = *larkCfg.AllowGroups
	}
	if larkCfg.AllowDirect != nil {
		cfg.Channels.Lark.AllowDirect = *larkCfg.AllowDirect
	}
	if agentPreset := strings.TrimSpace(larkCfg.AgentPreset); agentPreset != "" {
		cfg.Channels.Lark.AgentPreset = agentPreset
	}
	if toolPreset := strings.TrimSpace(larkCfg.ToolPreset); toolPreset != "" {
		cfg.Channels.Lark.ToolPreset = toolPreset
	}
	if toolMode := strings.TrimSpace(larkCfg.ToolMode); toolMode != "" {
		cfg.Channels.Lark.ToolMode = toolMode
	}
	if larkCfg.ReplyTimeoutSeconds != nil && *larkCfg.ReplyTimeoutSeconds > 0 {
		cfg.Channels.Lark.ReplyTimeout = time.Duration(*larkCfg.ReplyTimeoutSeconds) * time.Second
	}
	if reactEmoji := strings.TrimSpace(larkCfg.ReactEmoji); reactEmoji != "" {
		cfg.Channels.Lark.ReactEmoji = reactEmoji
	}
	if emoji := strings.TrimSpace(larkCfg.InjectionAckReactEmoji); emoji != "" {
		cfg.Channels.Lark.InjectionAckReactEmoji = emoji
	}
	if emoji := strings.TrimSpace(larkCfg.FinalAnswerReviewReactEmoji); emoji != "" {
		cfg.Channels.Lark.FinalAnswerReviewReactEmoji = emoji
	}
	if larkCfg.MemoryEnabled != nil {
		cfg.Channels.Lark.MemoryEnabled = *larkCfg.MemoryEnabled
	}
	if larkCfg.ShowToolProgress != nil {
		cfg.Channels.Lark.ShowToolProgress = *larkCfg.ShowToolProgress
	}
	if larkCfg.ShowPlanClarifyMessages != nil {
		cfg.Channels.Lark.ShowPlanClarifyMessages = *larkCfg.ShowPlanClarifyMessages
	}
	if larkCfg.AutoChatContextSize != nil && *larkCfg.AutoChatContextSize > 0 {
		cfg.Channels.Lark.AutoChatContextSize = *larkCfg.AutoChatContextSize
	}
	if larkCfg.PlanReviewEnabled != nil {
		cfg.Channels.Lark.PlanReviewEnabled = *larkCfg.PlanReviewEnabled
	}
	if larkCfg.PlanReviewRequireConfirmation != nil {
		cfg.Channels.Lark.PlanReviewRequireConfirmation = *larkCfg.PlanReviewRequireConfirmation
	}
	if larkCfg.PlanReviewPendingTTLMinutes != nil && *larkCfg.PlanReviewPendingTTLMinutes > 0 {
		cfg.Channels.Lark.PlanReviewPendingTTL = time.Duration(*larkCfg.PlanReviewPendingTTLMinutes) * time.Minute
	}
}

func applyServerHTTPConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Server == nil {
		return
	}
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
	if file.Server.EventHistoryRetentionDays != nil {
		days := *file.Server.EventHistoryRetentionDays
		if days <= 0 {
			cfg.EventHistoryRetention = 0
		} else {
			cfg.EventHistoryRetention = time.Duration(days) * 24 * time.Hour
		}
	}
	if file.Server.EventHistoryMaxSessions != nil {
		if *file.Server.EventHistoryMaxSessions <= 0 {
			cfg.EventHistoryMaxSessions = 0
		} else {
			cfg.EventHistoryMaxSessions = *file.Server.EventHistoryMaxSessions
		}
	}
	if file.Server.EventHistorySessionTTL != nil {
		if *file.Server.EventHistorySessionTTL <= 0 {
			cfg.EventHistorySessionTTL = 0
		} else {
			cfg.EventHistorySessionTTL = time.Duration(*file.Server.EventHistorySessionTTL) * time.Second
		}
	}
	if file.Server.EventHistoryMaxEvents != nil {
		if *file.Server.EventHistoryMaxEvents <= 0 {
			cfg.EventHistoryMaxEvents = 0
		} else {
			cfg.EventHistoryMaxEvents = *file.Server.EventHistoryMaxEvents
		}
	}
	if file.Server.EventHistoryAsyncBatchSize != nil && *file.Server.EventHistoryAsyncBatchSize > 0 {
		cfg.EventHistoryAsyncBatchSize = *file.Server.EventHistoryAsyncBatchSize
	}
	if file.Server.EventHistoryAsyncFlushMS != nil && *file.Server.EventHistoryAsyncFlushMS > 0 {
		cfg.EventHistoryAsyncFlushInterval = time.Duration(*file.Server.EventHistoryAsyncFlushMS) * time.Millisecond
	}
	if file.Server.EventHistoryAsyncAppendMS != nil && *file.Server.EventHistoryAsyncAppendMS > 0 {
		cfg.EventHistoryAsyncAppendTimeout = time.Duration(*file.Server.EventHistoryAsyncAppendMS) * time.Millisecond
	}
	if file.Server.EventHistoryAsyncQueueSize != nil && *file.Server.EventHistoryAsyncQueueSize > 0 {
		cfg.EventHistoryAsyncQueueCapacity = *file.Server.EventHistoryAsyncQueueSize
	}
	if file.Server.AllowedOrigins != nil {
		cfg.AllowedOrigins = normalizeAllowedOrigins(file.Server.AllowedOrigins)
	}
}

func applyAuthConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Auth == nil {
		return
	}
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
		DatabaseURL:           strings.TrimSpace(file.Auth.DatabaseURL),
		BootstrapEmail:        strings.TrimSpace(file.Auth.BootstrapEmail),
		BootstrapPassword:     file.Auth.BootstrapPassword,
		BootstrapDisplayName:  strings.TrimSpace(file.Auth.BootstrapDisplayName),
	}
}

func applySessionConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Session == nil {
		return
	}
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
	if file.Session.CacheSize != nil {
		cfg.Session.CacheSize = file.Session.CacheSize
	}
}

func applyAnalyticsConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Analytics == nil {
		return
	}
	cfg.Analytics = runtimeconfig.AnalyticsConfig{
		PostHogAPIKey: strings.TrimSpace(file.Analytics.PostHogAPIKey),
		PostHogHost:   strings.TrimSpace(file.Analytics.PostHogHost),
	}
}

func applyAttachmentConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Attachments == nil {
		return
	}
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
