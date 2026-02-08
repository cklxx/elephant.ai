package bootstrap

import (
	"errors"
	"os"
	"strings"
	"time"

	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
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
	logger.Debug("LLM Small Provider: %s (source=%s)", runtimeCfg.LLMSmallProvider, config.RuntimeMeta.Source("llm_small_provider"))
	logger.Debug("LLM Small Model: %s (source=%s)", runtimeCfg.LLMSmallModel, config.RuntimeMeta.Source("llm_small_model"))
	logger.Debug("Base URL: %s (source=%s)", runtimeCfg.BaseURL, config.RuntimeMeta.Source("base_url"))
	if strings.TrimSpace(runtimeCfg.APIKey) != "" {
		logger.Debug("API Key: (set; source=%s)", config.RuntimeMeta.Source("api_key"))
	} else {
		logger.Debug("API Key: (not set; source=%s)", config.RuntimeMeta.Source("api_key"))
	}

	sessionDBURL := strings.TrimSpace(config.Session.DatabaseURL)
	authDBURL := strings.TrimSpace(config.Auth.DatabaseURL)
	switch {
	case sessionDBURL != "":
		logger.Debug("Session DB: (set; source=session.database_url)")
	case authDBURL != "":
		logger.Debug("Session DB: (fallback to auth.database_url)")
	default:
		logger.Debug("Session DB: (not set)")
	}

	logger.Debug("Max Tokens: %d (source=%s)", runtimeCfg.MaxTokens, config.RuntimeMeta.Source("max_tokens"))
	logger.Debug("Max Iterations: %d (source=%s)", runtimeCfg.MaxIterations, config.RuntimeMeta.Source("max_iterations"))
	logger.Debug("Temperature: %.2f (provided=%t; source=%s)", runtimeCfg.Temperature, runtimeCfg.TemperatureProvided, config.RuntimeMeta.Source("temperature"))
	logger.Info("Environment: %s (source=%s)", runtimeCfg.Environment, config.RuntimeMeta.Source("environment"))
	logger.Info("Port: %s", config.Port)
	logger.Debug("HTTP Rate Limit: %d rpm (burst=%d)", config.RateLimit.RequestsPerMinute, config.RateLimit.Burst)
	logger.Debug("HTTP Non-Stream Timeout: %s", config.NonStreamTimeout)
	logger.Debug("Event History Retention: %s", config.EventHistory.Retention)
	logger.Debug("Event History Max Sessions: %d", config.EventHistory.MaxSessions)
	logger.Debug("Event History Session TTL: %s", config.EventHistory.SessionTTL)
	logger.Debug("Event History Max Events: %d", config.EventHistory.MaxEvents)
	logger.Debug("Event History Async Batch Size: %d", config.EventHistory.AsyncBatchSize)
	logger.Debug("Event History Async Flush Interval: %s", config.EventHistory.AsyncFlushInterval)
	logger.Debug("Event History Async Append Timeout: %s", config.EventHistory.AsyncAppendTimeout)
	logger.Debug("Event History Async Queue Capacity: %d", config.EventHistory.AsyncQueueCapacity)
	larkCfg := config.Channels.Lark
	if larkCfg.Enabled {
		logger.Info(
			"Lark Gateway: enabled (tool_mode=%s, tool_preset=%s, allow_groups=%t)",
			larkCfg.ToolMode,
			larkCfg.ToolPreset,
			larkCfg.AllowGroups,
		)
		logger.Debug("Lark Base Domain: %s", larkCfg.BaseDomain)
		logger.Debug("Lark Tenant Token: auto (app_id/app_secret)")
		if strings.TrimSpace(larkCfg.TenantCalendarID) != "" {
			logger.Debug("Lark Tenant Calendar ID: (set)")
		} else {
			logger.Debug("Lark Tenant Calendar ID: (not set)")
		}
		cardPort := strings.TrimSpace(larkCfg.CardCallbackPort)
		if cardPort == "" {
			cardPort = "9292"
		}
		logger.Info(
			"Lark Cards: enabled=%t plan_review=%t results=%t errors=%t callback_port=%s callback_configured=%t",
			larkCfg.CardsEnabled,
			larkCfg.CardsPlanReview,
			larkCfg.CardsResults,
			larkCfg.CardsErrors,
			cardPort,
			strings.TrimSpace(larkCfg.CardCallbackVerificationToken) != "",
		)
	} else {
		logger.Info("Lark Gateway: disabled")
	}
	logger.Info("===========================")
}
