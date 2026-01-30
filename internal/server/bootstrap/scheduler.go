package bootstrap

import (
	"context"
	"os"
	"path/filepath"

	"alex/internal/async"
	"alex/internal/di"
	"alex/internal/logging"
	"alex/internal/scheduler"
	okrtools "alex/internal/tools/builtin/okr"
)

// startScheduler creates and starts the proactive scheduler.
// Returns the scheduler instance or nil if initialization fails.
func startScheduler(ctx context.Context, cfg Config, container *di.Container, logger logging.Logger) *scheduler.Scheduler {
	logger = logging.OrNop(logger)

	// Resolve OKR goals root
	goalsRoot := resolveGoalsRoot(cfg)

	// Build notifier
	var notifier scheduler.Notifier
	larkCfg := cfg.Channels.Lark
	if larkCfg.Enabled && larkCfg.AppID != "" && larkCfg.AppSecret != "" {
		notifier = scheduler.NewLarkNotifier(larkCfg.AppID, larkCfg.AppSecret, logger)
		logger.Info("Scheduler: Lark notifier initialized")
	} else {
		notifier = scheduler.NopNotifier{}
		logger.Info("Scheduler: notifications disabled (no Lark config)")
	}

	schedCfg := scheduler.Config{
		Enabled:        true,
		StaticTriggers: cfg.Runtime.Proactive.Scheduler.Triggers,
		OKRGoalsRoot:   goalsRoot,
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
