package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/app/di"
	"alex/internal/app/scheduler"
	"alex/internal/infra/moltbook"
	okrtools "alex/internal/infra/tools/builtin/okr"
	"alex/internal/shared/async"
	"alex/internal/shared/logging"
)

// startScheduler creates and starts the proactive scheduler.
// Returns the scheduler instance or nil if initialization fails.
func startScheduler(ctx context.Context, cfg Config, container *di.Container, logger logging.Logger) *scheduler.Scheduler {
	logger = logging.OrNop(logger)

	// Resolve OKR goals root
	goalsRoot := resolveGoalsRoot(cfg)

	// Build notifier(s)
	var notifiers []scheduler.Notifier

	larkCfg := cfg.Channels.Lark
	if larkCfg.Enabled && larkCfg.AppID != "" && larkCfg.AppSecret != "" {
		notifiers = append(notifiers, scheduler.NewLarkNotifier(larkCfg.AppID, larkCfg.AppSecret, logger))
		logger.Info("Scheduler: Lark notifier initialized")
	}

	if cfg.Runtime.MoltbookAPIKey != "" {
		moltbookClient := moltbook.NewRateLimitedClient(moltbook.Config{
			BaseURL: cfg.Runtime.MoltbookBaseURL,
			APIKey:  cfg.Runtime.MoltbookAPIKey,
		})
		notifiers = append(notifiers, scheduler.NewMoltbookNotifier(moltbookClient, logger))
		logger.Info("Scheduler: Moltbook notifier initialized")
	}

	var notifier scheduler.Notifier
	switch len(notifiers) {
	case 0:
		notifier = scheduler.NopNotifier{}
		logger.Info("Scheduler: notifications disabled (no channel config)")
	case 1:
		notifier = notifiers[0]
	default:
		notifier = scheduler.NewCompositeNotifier(notifiers...)
	}

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

	schedCfg := scheduler.Config{
		Enabled:            true,
		StaticTriggers:     cfg.Runtime.Proactive.Scheduler.Triggers,
		OKRGoalsRoot:       goalsRoot,
		CalendarReminder:   cfg.Runtime.Proactive.Scheduler.CalendarReminder,
		TriggerTimeout:     time.Duration(cfg.Runtime.Proactive.Scheduler.TriggerTimeoutSeconds) * time.Second,
		ConcurrencyPolicy:  cfg.Runtime.Proactive.Scheduler.ConcurrencyPolicy,
		JobStore:           jobStore,
		Cooldown:           cooldown,
		MaxConcurrent:      maxConcurrent,
		RecoveryMaxRetries: recoveryMaxRetries,
		RecoveryBackoff:    recoveryBackoff,
	}

	sched := scheduler.New(schedCfg, container.AgentCoordinator, notifier, logger)

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
