package bootstrap

import (
	"strings"

	"alex/internal/analytics"
	"alex/internal/logging"
)

func BuildAnalyticsClient(cfg AnalyticsConfig, logger logging.Logger) (analytics.Client, func()) {
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
		logger.Info("Analytics client disabled: POSTHOG_API_KEY not provided")
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
