package bootstrap

import (
	"errors"
	"os"
	"strings"
	"time"

	runtimeconfig "alex/internal/config"
	"alex/internal/logging"
)

// LogServerConfiguration prints a safe, redacted snapshot of the server runtime configuration.
func LogServerConfiguration(logger logging.Logger, config Config) {
	logger = logging.OrNop(logger)
	runtimeCfg := config.Runtime

	logger.Info("=== Server Configuration ===")

	configPath, configPathSource := runtimeconfig.ResolveConfigPath(runtimeconfig.DefaultEnvLookup, nil)
	if configPath != "" {
		logger.Info("Config file: %s (%s)", configPath, configPathSource)
		if info, err := os.Stat(configPath); err == nil {
			logger.Info("Config mtime: %s", info.ModTime().UTC().Format(time.RFC3339))
		} else if errors.Is(err, os.ErrNotExist) {
			logger.Warn("Config file missing: %s", configPath)
		} else {
			logger.Warn("Config file stat failed: %v", err)
		}
	} else {
		logger.Warn("Config file path unavailable (source=%s)", configPathSource)
	}

	logger.Info("LLM Provider: %s (source=%s)", runtimeCfg.LLMProvider, config.RuntimeMeta.Source("llm_provider"))
	logger.Info("LLM Model: %s (source=%s)", runtimeCfg.LLMModel, config.RuntimeMeta.Source("llm_model"))
	logger.Info("LLM Small Provider: %s (source=%s)", runtimeCfg.LLMSmallProvider, config.RuntimeMeta.Source("llm_small_provider"))
	logger.Info("LLM Small Model: %s (source=%s)", runtimeCfg.LLMSmallModel, config.RuntimeMeta.Source("llm_small_model"))
	logger.Info("Base URL: %s (source=%s)", runtimeCfg.BaseURL, config.RuntimeMeta.Source("base_url"))
	if strings.TrimSpace(runtimeCfg.APIKey) != "" {
		logger.Info("API Key: (set; source=%s)", config.RuntimeMeta.Source("api_key"))
	} else {
		logger.Info("API Key: (not set; source=%s)", config.RuntimeMeta.Source("api_key"))
	}

	sessionDBURL := strings.TrimSpace(config.Session.DatabaseURL)
	authDBURL := strings.TrimSpace(config.Auth.DatabaseURL)
	switch {
	case sessionDBURL != "":
		logger.Info("Session DB: (set; source=session.database_url)")
	case authDBURL != "":
		logger.Info("Session DB: (fallback to auth.database_url)")
	default:
		logger.Info("Session DB: (not set)")
	}

	logger.Info("Max Tokens: %d (source=%s)", runtimeCfg.MaxTokens, config.RuntimeMeta.Source("max_tokens"))
	logger.Info("Max Iterations: %d (source=%s)", runtimeCfg.MaxIterations, config.RuntimeMeta.Source("max_iterations"))
	logger.Info("Temperature: %.2f (provided=%t; source=%s)", runtimeCfg.Temperature, runtimeCfg.TemperatureProvided, config.RuntimeMeta.Source("temperature"))
	logger.Info("Environment: %s (source=%s)", runtimeCfg.Environment, config.RuntimeMeta.Source("environment"))
	logger.Info("Port: %s", config.Port)
	logger.Info("HTTP Rate Limit: %d rpm (burst=%d)", config.RateLimitRequestsPerMinute, config.RateLimitBurst)
	logger.Info("HTTP Non-Stream Timeout: %s", config.NonStreamTimeout)
	logger.Info("Event History Retention: %s", config.EventHistoryRetention)
	logger.Info("Event History Max Sessions: %d", config.EventHistoryMaxSessions)
	logger.Info("Event History Session TTL: %s", config.EventHistorySessionTTL)
	logger.Info("Event History Max Events: %d", config.EventHistoryMaxEvents)
	larkCfg := config.Channels.Lark
	if larkCfg.Enabled {
		logger.Info(
			"Lark Gateway: enabled (tool_mode=%s, tool_preset=%s, allow_groups=%t)",
			larkCfg.ToolMode,
			larkCfg.ToolPreset,
			larkCfg.AllowGroups,
		)
		logger.Info("Lark Base Domain: %s", larkCfg.BaseDomain)
		logger.Info("Lark Tenant Token Mode: %s", larkCfg.TenantTokenMode)
		if strings.TrimSpace(larkCfg.TenantCalendarID) != "" {
			logger.Info("Lark Tenant Calendar ID: (set)")
		} else {
			logger.Info("Lark Tenant Calendar ID: (not set)")
		}
		if strings.TrimSpace(larkCfg.TenantAccessToken) != "" {
			logger.Info("Lark Tenant Access Token: (set)")
		} else {
			logger.Info("Lark Tenant Access Token: (not set)")
		}
		logger.Info(
			"Lark Cards: enabled=%t plan_review=%t results=%t errors=%t callback_configured=%t",
			larkCfg.CardsEnabled,
			larkCfg.CardsPlanReview,
			larkCfg.CardsResults,
			larkCfg.CardsErrors,
			strings.TrimSpace(larkCfg.CardCallbackVerificationToken) != "",
		)
	} else {
		logger.Info("Lark Gateway: disabled")
	}
	logger.Info("===========================")
}
