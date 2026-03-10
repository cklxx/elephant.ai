package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/app/di"
	"alex/internal/app/milestone"
	"alex/internal/app/scheduler"
	okrtools "alex/internal/infra/tools/builtin/okr"
	"alex/internal/shared/async"
	"alex/internal/shared/logging"
)

// startScheduler creates and starts the proactive scheduler.
// Returns the scheduler instance or nil if initialization fails.
func startScheduler(ctx context.Context, cfg Config, container *di.Container, logger logging.Logger) *scheduler.Scheduler {
	logger = logging.OrNop(logger)

	goalsRoot := resolveGoalsRoot(cfg)
	notifier := BuildNotifiers(cfg, "Scheduler", logger)

	var jobStore scheduler.JobStore
	jobStorePath := strings.TrimSpace(cfg.Runtime.Proactive.Scheduler.JobStorePath)
	if jobStorePath != "" {
		jobStorePath = expandHome(jobStorePath)
		jobStore = scheduler.NewFileJobStore(jobStorePath)
		logger.Info("Scheduler: job store initialized at %s", jobStorePath)
	}

	cooldown := time.Duration(cfg.Runtime.Proactive.Scheduler.CooldownSeconds) * time.Second
	if cooldown < 0 {
		cooldown = 0
	}

	maxConcurrent := cfg.Runtime.Proactive.Scheduler.MaxConcurrent

	recoveryMaxRetries := cfg.Runtime.Proactive.Scheduler.RecoveryMaxRetries
	recoveryBackoff := time.Duration(cfg.Runtime.Proactive.Scheduler.RecoveryBackoffSeconds) * time.Second

	milestoneCheckinCfg := cfg.Runtime.Proactive.Scheduler.MilestoneCheckin
	var milestoneSvc scheduler.MilestoneCheckinService
	if milestoneCheckinCfg.Enabled && container != nil && container.TaskStore != nil {
		lookback := time.Duration(milestoneCheckinCfg.LookbackSeconds) * time.Second
		if lookback <= 0 {
			lookback = time.Hour
		}
		svc := milestone.NewService(container.TaskStore, notifier, milestone.Config{
			Enabled:          true,
			IntervalSeconds:  milestoneCheckinCfg.LookbackSeconds,
			LookbackDuration: lookback,
			Channel:          milestoneCheckinCfg.Channel,
			ChatID:           milestoneCheckinCfg.ChatID,
			IncludeActive:    milestoneCheckinCfg.IncludeActive,
			IncludeCompleted: milestoneCheckinCfg.IncludeCompleted,
		})
		milestoneSvc = svc
		logger.Info("Milestone check-in service created (channel=%s, chat_id=%s)", milestoneCheckinCfg.Channel, milestoneCheckinCfg.ChatID)
	}

	schedCfg := scheduler.Config{
		Enabled:            true,
		StaticTriggers:     cfg.Runtime.Proactive.Scheduler.Triggers,
		OKRGoalsRoot:       goalsRoot,
		CalendarReminder:   cfg.Runtime.Proactive.Scheduler.CalendarReminder,
		Heartbeat:          cfg.Runtime.Proactive.Scheduler.Heartbeat,
		MilestoneCheckin:   milestoneCheckinCfg,
		MilestoneService:   milestoneSvc,
		TriggerTimeout:     time.Duration(cfg.Runtime.Proactive.Scheduler.TriggerTimeoutSeconds) * time.Second,
		ConcurrencyPolicy:  cfg.Runtime.Proactive.Scheduler.ConcurrencyPolicy,
		JobStore:           jobStore,
		Cooldown:           cooldown,
		MaxConcurrent:      maxConcurrent,
		RecoveryMaxRetries: recoveryMaxRetries,
		RecoveryBackoff:    recoveryBackoff,
		LeaderLock:         nil, // distributed lock removed (local single-process)
	}

	sched := scheduler.New(schedCfg, container.AgentCoordinator, notifier, logger)
	container.AgentCoordinator.SetScheduler(sched)

	async.Go(logger, "scheduler", func() {
		if err := sched.Start(ctx); err != nil {
			logger.Warn("Scheduler start failed: %v", err)
		}
	})

	logger.Info("Scheduler started (triggers=%d, okr_goals_root=%s)", len(schedCfg.StaticTriggers), goalsRoot)
	return sched
}

// resolveGoalsRoot determines the OKR goals directory path.
func resolveGoalsRoot(cfg Config) string {
	if root := cfg.Runtime.Proactive.OKR.GoalsRoot; root != "" {
		return expandHome(root)
	}
	okrDefault := okrtools.DefaultOKRConfig()
	return okrDefault.GoalsRoot
}

// expandHome resolves ~ to the user's home directory.
func expandHome(path string) string {
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
