package bootstrap

import (
	"context"
	"fmt"
	"strings"
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
			return
		}
		if timerCfg.HeartbeatEnabled {
			interval := timerCfg.HeartbeatMinutes
			if interval <= 0 {
				interval = 30
			}
			if err := ensureHeartbeatTimer(mgr, interval, logger); err != nil {
				logger.Warn("TimerManager heartbeat setup failed: %v", err)
			}
		}
	})

	logger.Info("TimerManager started (store=%s, max_timers=%d, timeout=%s)", storePath, maxTimers, taskTimeout)
	return mgr
}

func ensureHeartbeatTimer(mgr *timer.TimerManager, minutes int, logger logging.Logger) error {
	if mgr == nil {
		return nil
	}
	if existing, ok := mgr.Get("tmr-heartbeat-system"); ok && existing.IsActive() {
		logger.Info("TimerManager: heartbeat timer already active")
		return nil
	}
	if minutes <= 0 {
		minutes = 30
	}
	schedule := fmt.Sprintf("*/%d * * * *", minutes)
	entry := &timer.Timer{
		ID:        "tmr-heartbeat-system",
		Name:      "system-heartbeat",
		Type:      timer.TimerTypeRecurring,
		Schedule:  strings.TrimSpace(schedule),
		Task:      "Read HEARTBEAT.md if it exists. Follow it strictly. If nothing needs attention, reply HEARTBEAT_OK.",
		SessionID: "timer-heartbeat",
		CreatedAt: time.Now(),
		Status:    timer.StatusActive,
	}
	return mgr.Add(entry)
}
