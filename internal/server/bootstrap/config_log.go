package bootstrap

import (
	"strings"

	"alex/internal/logging"
)

// LogServerConfiguration prints a safe, redacted snapshot of the server runtime configuration.
func LogServerConfiguration(logger logging.Logger, config Config) {
	logger = logging.OrNop(logger)
	runtimeCfg := config.Runtime

	logger.Info("=== Server Configuration ===")
	logger.Info("LLM Provider: %s", runtimeCfg.LLMProvider)
	logger.Info("LLM Model: %s", runtimeCfg.LLMModel)
	logger.Info("Base URL: %s", runtimeCfg.BaseURL)
	if runtimeCfg.SandboxBaseURL != "" {
		logger.Info("Sandbox Base URL: %s", runtimeCfg.SandboxBaseURL)
	} else {
		logger.Info("Sandbox Base URL: (not set)")
	}
	if strings.TrimSpace(runtimeCfg.APIKey) != "" {
		logger.Info("API Key: (set)")
	} else {
		logger.Info("API Key: (not set)")
	}
	logger.Info("Max Tokens: %d", runtimeCfg.MaxTokens)
	logger.Info("Max Iterations: %d", runtimeCfg.MaxIterations)
	logger.Info("Temperature: %.2f (provided=%t)", runtimeCfg.Temperature, runtimeCfg.TemperatureProvided)
	logger.Info("Environment: %s", runtimeCfg.Environment)
	logger.Info("Port: %s", config.Port)
	logger.Info("===========================")
}
