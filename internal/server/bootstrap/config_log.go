package bootstrap

import (
	"errors"
	"os"
	"path/filepath"
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

	configPath, configPathSource := resolveRuntimeConfigPath(runtimeconfig.DefaultEnvLookup)
	if configPath != "" {
		logger.Info("Runtime config file: %s (%s)", configPath, configPathSource)
		if info, err := os.Stat(configPath); err == nil {
			logger.Info("Runtime config mtime: %s", info.ModTime().UTC().Format(time.RFC3339))
		} else if errors.Is(err, os.ErrNotExist) {
			logger.Warn("Runtime config file missing: %s", configPath)
		} else {
			logger.Warn("Runtime config file stat failed: %v", err)
		}
	} else {
		logger.Warn("Runtime config file path unavailable (source=%s)", configPathSource)
	}

	logger.Info("LLM Provider: %s (source=%s)", runtimeCfg.LLMProvider, config.RuntimeMeta.Source("llm_provider"))
	logger.Info("LLM Model: %s (source=%s)", runtimeCfg.LLMModel, config.RuntimeMeta.Source("llm_model"))
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
		logger.Info("Session DB: (set; source=ALEX_SESSION_DATABASE_URL)")
	case authDBURL != "":
		logger.Info("Session DB: (fallback to AUTH_DATABASE_URL)")
	default:
		logger.Info("Session DB: (not set)")
	}

	logger.Info("Max Tokens: %d (source=%s)", runtimeCfg.MaxTokens, config.RuntimeMeta.Source("max_tokens"))
	logger.Info("Max Iterations: %d (source=%s)", runtimeCfg.MaxIterations, config.RuntimeMeta.Source("max_iterations"))
	logger.Info("Temperature: %.2f (provided=%t; source=%s)", runtimeCfg.Temperature, runtimeCfg.TemperatureProvided, config.RuntimeMeta.Source("temperature"))
	logger.Info("Environment: %s (source=%s)", runtimeCfg.Environment, config.RuntimeMeta.Source("environment"))
	logger.Info("Port: %s", config.Port)
	logger.Info("===========================")
}

func resolveRuntimeConfigPath(lookup runtimeconfig.EnvLookup) (path string, source string) {
	if lookup == nil {
		lookup = runtimeconfig.DefaultEnvLookup
	}
	if value, ok := lookup("ALEX_CONFIG_PATH"); ok {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed, "ALEX_CONFIG_PATH"
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", "unavailable"
	}
	home = strings.TrimSpace(home)
	if home == "" {
		return "", "unavailable"
	}
	return filepath.Join(home, ".alex-config.json"), "default"
}
