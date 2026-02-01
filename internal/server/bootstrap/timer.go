package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"alex/internal/async"
	"alex/internal/di"
	"alex/internal/logging"
	"alex/internal/moltbook"
	"alex/internal/scheduler"
	"alex/internal/timer"
)

// startTimerManager creates and starts the agent timer manager.
// Returns the timer manager instance or nil if initialization fails.
func startTimerManager(ctx context.Context, cfg Config, container *di.Container, logger logging.Logger) *timer.TimerManager {
	logger = logging.OrNop(logger)

	timerCfg := cfg.Runtime.Proactive.Timer

	storePath := timerCfg.StorePath
	if storePath == "" {
		storePath = "~/.alex/timers"
	}
	storePath = resolveTimerStorePath(storePath)

	maxTimers := timerCfg.MaxTimers
	if maxTimers <= 0 {
		maxTimers = 100
	}

	taskTimeout := time.Duration(timerCfg.TaskTimeoutSeconds) * time.Second
	if taskTimeout <= 0 {
		taskTimeout = 15 * time.Minute
	}

	// Build notifier(s) — reuse same pattern as scheduler.
	var notifiers []scheduler.Notifier

	larkCfg := cfg.Channels.Lark
	if larkCfg.Enabled && larkCfg.AppID != "" && larkCfg.AppSecret != "" {
		notifiers = append(notifiers, scheduler.NewLarkNotifier(larkCfg.AppID, larkCfg.AppSecret, logger))
		logger.Info("TimerManager: Lark notifier initialized")
	}

	if cfg.Runtime.MoltbookAPIKey != "" {
		moltbookClient := moltbook.NewRateLimitedClient(moltbook.Config{
			BaseURL: cfg.Runtime.MoltbookBaseURL,
			APIKey:  cfg.Runtime.MoltbookAPIKey,
		})
		notifiers = append(notifiers, scheduler.NewMoltbookNotifier(moltbookClient, logger))
		logger.Info("TimerManager: Moltbook notifier initialized")
	}

	var notifier timer.Notifier
	switch len(notifiers) {
	case 0:
		notifier = scheduler.NopNotifier{}
		logger.Info("TimerManager: notifications disabled (no channel config)")
	case 1:
		notifier = notifiers[0]
	default:
		notifier = scheduler.NewCompositeNotifier(notifiers...)
	}

	mgrCfg := timer.Config{
		Enabled:     true,
		StorePath:   storePath,
		MaxTimers:   maxTimers,
		TaskTimeout: taskTimeout,
	}

	mgr, err := timer.NewTimerManager(mgrCfg, container.AgentCoordinator, notifier, logger)
	if err != nil {
		logger.Warn("TimerManager: failed to create: %v", err)
		return nil
	}

	// Wire the timer manager into the coordinator so tools can access it via context.
	container.AgentCoordinator.SetTimerManager(mgr)

	async.Go(logger, "timer-manager", func() {
		if err := mgr.Start(ctx); err != nil {
			logger.Warn("TimerManager start failed: %v", err)
		}
	})

	logger.Info("TimerManager started (store=%s, max_timers=%d, timeout=%s)", storePath, maxTimers, taskTimeout)
	return mgr
}

// resolveTimerStorePath expands ~ and ensures the directory exists.
func resolveTimerStorePath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		if len(path) > 1 && path[1] == '/' {
			return filepath.Join(home, path[2:])
		}
		return home
	}
	return path
}
