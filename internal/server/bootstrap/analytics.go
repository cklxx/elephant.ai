package bootstrap

import (
	"strings"

	"alex/internal/analytics"
	"alex/internal/utils"
)

func BuildAnalyticsClient(cfg AnalyticsConfig, logger *utils.Logger) (analytics.Client, func()) {
	client := analytics.NewNoopClient()

	if apiKey := strings.TrimSpace(cfg.PostHogAPIKey); apiKey != "" {
		posthogClient, err := analytics.NewPostHogClient(apiKey, strings.TrimSpace(cfg.PostHogHost))
		if err != nil {
			if logger != nil {
				logger.Warn("Analytics disabled: %v", err)
			}
		} else {
			client = posthogClient
			if logger != nil {
				logger.Info("Analytics client initialized (PostHog)")
			}
		}
	} else if logger != nil {
		logger.Info("Analytics client disabled: POSTHOG_API_KEY not provided")
	}

	cleanup := func() {
		if client == nil {
			return
		}
		if err := client.Close(); err != nil && logger != nil {
			logger.Warn("Failed to close analytics client: %v", err)
		}
	}

	return client, cleanup
}
