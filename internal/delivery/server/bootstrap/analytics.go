package bootstrap

import (
	"strings"

	"alex/internal/infra/analytics"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
)

func BuildAnalyticsClient(cfg runtimeconfig.AnalyticsConfig, logger logging.Logger) (analytics.Client, func()) {
	logger = logging.OrNop(logger)
	client := analytics.NewNoopClient()

	if apiKey := strings.TrimSpace(cfg.PostHogAPIKey); apiKey != "" {
		posthogClient, err := analytics.NewPostHogClient(apiKey, strings.TrimSpace(cfg.PostHogHost))
		if err != nil {
			logger.Warn("Analytics disabled: %v", err)
		} else {
			client = posthogClient
			logger.Info("Analytics client initialized (PostHog)")
		}
	} else {
		logger.Info("Analytics client disabled: analytics.posthog_api_key not configured")
	}

	cleanup := func() {
		if client == nil {
			return
		}
		if err := client.Close(); err != nil {
			logger.Warn("Failed to close analytics client: %v", err)
		}
	}

	return client, cleanup
}
