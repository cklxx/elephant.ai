package bootstrap

import (
	"alex/internal/app/scheduler"
	"alex/internal/infra/moltbook"
	"alex/internal/shared/logging"
)

// BuildNotifiers constructs notification channels (Lark, Moltbook) from config.
// The label parameter is used for log messages (e.g. "Scheduler", "TimerManager").
func BuildNotifiers(cfg Config, label string, logger logging.Logger) scheduler.Notifier {
	logger = logging.OrNop(logger)
	var notifiers []scheduler.Notifier

	larkCfg := cfg.Channels.Lark
	if larkCfg.Enabled && larkCfg.AppID != "" && larkCfg.AppSecret != "" {
		notifiers = append(notifiers, scheduler.NewLarkNotifier(larkCfg.AppID, larkCfg.AppSecret, logger))
		logger.Info("%s: Lark notifier initialized", label)
	}

	if cfg.Runtime.MoltbookAPIKey != "" {
		moltbookClient := moltbook.NewRateLimitedClient(moltbook.Config{
			BaseURL: cfg.Runtime.MoltbookBaseURL,
			APIKey:  cfg.Runtime.MoltbookAPIKey,
		})
		notifiers = append(notifiers, scheduler.NewMoltbookNotifier(moltbookClient, logger))
		logger.Info("%s: Moltbook notifier initialized", label)
	}

	switch len(notifiers) {
	case 0:
		logger.Info("%s: notifications disabled (no channel config)", label)
		return scheduler.NopNotifier{}
	case 1:
		return notifiers[0]
	default:
		return scheduler.NewCompositeNotifier(notifiers...)
	}
}
