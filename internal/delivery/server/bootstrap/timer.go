package bootstrap

import (
	"context"
	"time"

	"alex/internal/app/di"
	"alex/internal/shared/async"
	"alex/internal/shared/logging"
	"alex/internal/shared/timer"
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
	storePath = expandHome(storePath)

	maxTimers := timerCfg.MaxTimers
	if maxTimers <= 0 {
		maxTimers = 100
	}

	taskTimeout := time.Duration(timerCfg.TaskTimeoutSeconds) * time.Second
	if taskTimeout <= 0 {
		taskTimeout = 15 * time.Minute
	}

	notifier := BuildNotifiers(cfg, "TimerManager", logger)

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
