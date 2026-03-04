package bootstrap

import (
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
		notifiers = append(notifiers, infranotify.NewLarkSender(larkCfg.AppID, larkCfg.AppSecret, logger))
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
