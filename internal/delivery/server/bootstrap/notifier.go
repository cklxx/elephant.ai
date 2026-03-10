package bootstrap

import (
	larkpkg "alex/internal/delivery/channels/lark"
	"alex/internal/infra/moltbook"
	infranotify "alex/internal/infra/notification"
	"alex/internal/shared/logging"
	"alex/internal/shared/notification"
)

// BuildNotifiers constructs notification channels (Lark, Moltbook) from config.
// The label parameter is used for log messages (e.g. "Scheduler", "TimerManager").
func BuildNotifiers(cfg Config, label string, logger logging.Logger) notification.Notifier {
	logger = logging.OrNop(logger)
	var notifiers []notification.Notifier

	larkCfg := cfg.Channels.LarkConfig()
	if larkCfg.Enabled && larkCfg.AppID != "" && larkCfg.AppSecret != "" {
		var sender notification.Notifier = infranotify.NewLarkSender(larkCfg.AppID, larkCfg.AppSecret, logger)

		if larkCfg.RateLimiterEnabled {
			limiter := larkpkg.NewRateLimiter(larkpkg.RateLimiterConfig{
				ChatHourlyLimit: larkCfg.RateLimiterChatHourlyLimit,
				UserDailyLimit:  larkCfg.RateLimiterUserDailyLimit,
			})
			sender = larkpkg.NewRateLimitedNotifier(sender, limiter, logger)
			logger.Info("%s: Lark rate limiter enabled (chat_hourly=%d, user_daily=%d)",
				label, limiter.Config().ChatHourlyLimit, limiter.Config().UserDailyLimit)
		}

		notifiers = append(notifiers, sender)
		logger.Info("%s: Lark notifier initialized", label)
	}

	if cfg.Runtime.MoltbookAPIKey != "" {
		moltbookClient := moltbook.NewRateLimitedClient(moltbook.Config{
			BaseURL: cfg.Runtime.MoltbookBaseURL,
			APIKey:  cfg.Runtime.MoltbookAPIKey,
		})
		notifiers = append(notifiers, infranotify.NewMoltbookSender(moltbookClient, logger))
		logger.Info("%s: Moltbook notifier initialized", label)
	}

	switch len(notifiers) {
	case 0:
		logger.Info("%s: notifications disabled (no channel config)", label)
		return infranotify.NopNotifier{}
	case 1:
		return notifiers[0]
	default:
		return infranotify.NewCompositeNotifier(notifiers...)
	}
}
