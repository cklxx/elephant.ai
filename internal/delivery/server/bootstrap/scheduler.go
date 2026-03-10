package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/app/blocker"
	"alex/internal/app/di"
	"alex/internal/app/milestone"
	"alex/internal/app/prepbrief"
	"alex/internal/app/pulse"
	"alex/internal/app/scheduler"
	"alex/internal/infra/leaderlock"
	infranotify "alex/internal/infra/notification"
	"alex/internal/infra/observability"
	okrtools "alex/internal/infra/tools/builtin/okr"
	"alex/internal/shared/async"
	"alex/internal/shared/logging"
	"alex/internal/shared/notification"
)

// instrumentedNotifier returns a notifier wrapped with alert outcome telemetry
// if metrics are available, otherwise returns the base notifier unchanged.
func instrumentedNotifier(base notification.Notifier, metrics *observability.MetricsCollector, feature string) notification.Notifier {
	if metrics == nil {
		return base
	}
	recorder := &observability.MetricsOutcomeRecorder{Metrics: metrics}
	return infranotify.NewInstrumentedNotifier(base, recorder, metrics, feature)
}

// startScheduler creates and starts the proactive scheduler.
// Returns the scheduler instance or nil if initialization fails.
func startScheduler(ctx context.Context, cfg Config, container *di.Container, metrics *observability.MetricsCollector, logger logging.Logger) *scheduler.Scheduler {
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

	svcs := buildSchedulerServices(cfg, container, notifier, metrics, logger)

	schedCfg := scheduler.Config{
		Enabled:             true,
		StaticTriggers:      cfg.Runtime.Proactive.Scheduler.Triggers,
		OKRGoalsRoot:        goalsRoot,
		CalendarReminder:    cfg.Runtime.Proactive.Scheduler.CalendarReminder,
		Heartbeat:           cfg.Runtime.Proactive.Scheduler.Heartbeat,
		MilestoneCheckin:    cfg.Runtime.Proactive.Scheduler.MilestoneCheckin,
		MilestoneService:    svcs.milestone,
		WeeklyPulse:         cfg.Runtime.Proactive.Scheduler.WeeklyPulse,
		WeeklyPulseService:  svcs.weeklyPulse,
		BlockerRadar:        cfg.Runtime.Proactive.Scheduler.BlockerRadar,
		BlockerRadarService: svcs.blockerRadar,
		PrepBrief:           cfg.Runtime.Proactive.Scheduler.PrepBrief,
		PrepBriefService:    svcs.prepBrief,
		CalendarPort:        container.CalendarPort,
		TriggerTimeout:      time.Duration(cfg.Runtime.Proactive.Scheduler.TriggerTimeoutSeconds) * time.Second,
		ConcurrencyPolicy:   cfg.Runtime.Proactive.Scheduler.ConcurrencyPolicy,
		JobStore:            jobStore,
		Cooldown:            cooldown,
		MaxConcurrent:       cfg.Runtime.Proactive.Scheduler.MaxConcurrent,
		RecoveryMaxRetries:  cfg.Runtime.Proactive.Scheduler.RecoveryMaxRetries,
		RecoveryBackoff:     time.Duration(cfg.Runtime.Proactive.Scheduler.RecoveryBackoffSeconds) * time.Second,
		LeaderLock:          buildLeaderLock(cfg, logger),
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

type schedulerServices struct {
	milestone    scheduler.MilestoneCheckinService
	weeklyPulse  scheduler.WeeklyPulseService
	blockerRadar scheduler.BlockerRadarService
	prepBrief    scheduler.PrepBriefService
}

func buildSchedulerServices(cfg Config, container *di.Container, notifier notification.Notifier, metrics *observability.MetricsCollector, logger logging.Logger) schedulerServices {
	var svcs schedulerServices
	schedCfg := cfg.Runtime.Proactive.Scheduler
	if container == nil || container.TaskStore == nil {
		return svcs
	}

	if schedCfg.MilestoneCheckin.Enabled {
		lookback := time.Duration(schedCfg.MilestoneCheckin.LookbackSeconds) * time.Second
		if lookback <= 0 {
			lookback = time.Hour
		}
		svc := milestone.NewService(container.TaskStore, instrumentedNotifier(notifier, metrics, "milestone_checkin"), milestone.Config{
			Enabled:          true,
			IntervalSeconds:  schedCfg.MilestoneCheckin.LookbackSeconds,
			LookbackDuration: lookback,
			Channel:          schedCfg.MilestoneCheckin.Channel,
			ChatID:           schedCfg.MilestoneCheckin.ChatID,
			IncludeActive:    schedCfg.MilestoneCheckin.IncludeActive,
			IncludeCompleted: schedCfg.MilestoneCheckin.IncludeCompleted,
		})
		svcs.milestone = svc
		logger.Info("Milestone check-in service created (channel=%s, chat_id=%s)", schedCfg.MilestoneCheckin.Channel, schedCfg.MilestoneCheckin.ChatID)
	}

	if schedCfg.WeeklyPulse.Enabled {
		svc := pulse.NewService(container.TaskStore, instrumentedNotifier(notifier, metrics, "weekly_pulse"), schedCfg.WeeklyPulse.Channel, schedCfg.WeeklyPulse.ChatID)
		svc.GitSignalSource = container.GitSignalProvider
		svcs.weeklyPulse = svc
		logger.Info("Weekly pulse service created (channel=%s, chat_id=%s)", schedCfg.WeeklyPulse.Channel, schedCfg.WeeklyPulse.ChatID)
	}

	if schedCfg.BlockerRadar.Enabled {
		gitSignalCfg := schedCfg.GitSignal
		gitReviewThreshold := time.Duration(gitSignalCfg.ReviewBottleneckThreshold) * time.Second
		radar := blocker.NewRadar(container.TaskStore, instrumentedNotifier(notifier, metrics, "blocker_radar"), blocker.Config{
			Enabled:               true,
			StaleThresholdSeconds: schedCfg.BlockerRadar.StaleThresholdSeconds,
			InputWaitSeconds:      schedCfg.BlockerRadar.InputWaitSeconds,
			Channel:               schedCfg.BlockerRadar.Channel,
			ChatID:                schedCfg.BlockerRadar.ChatID,
			GitRepos:              gitSignalCfg.Repos,
			GitReviewThreshold:    gitReviewThreshold,
		})
		radar.GitSignalSource = container.GitSignalProvider
		svcs.blockerRadar = &blockerRadarAdapter{radar: radar}
		logger.Info("Blocker radar service created (channel=%s, chat_id=%s)", schedCfg.BlockerRadar.Channel, schedCfg.BlockerRadar.ChatID)
	}

	if schedCfg.PrepBrief.Enabled {
		lookback := time.Duration(schedCfg.PrepBrief.LookbackSeconds) * time.Second
		if lookback <= 0 {
			lookback = 7 * 24 * time.Hour
		}
		gitSignalCfg := schedCfg.GitSignal
		svc := prepbrief.NewService(container.TaskStore, instrumentedNotifier(notifier, metrics, "prep_brief"), prepbrief.Config{
			LookbackDuration: lookback,
			LookbackSeconds:  schedCfg.PrepBrief.LookbackSeconds,
			Channel:          schedCfg.PrepBrief.Channel,
			ChatID:           schedCfg.PrepBrief.ChatID,
			GitRepos:         gitSignalCfg.Repos,
		})
		svc.GitSignalSource = container.GitSignalProvider
		svcs.prepBrief = &prepBriefAdapter{svc: svc}
		logger.Info("Prep brief service created (channel=%s, chat_id=%s)", schedCfg.PrepBrief.Channel, schedCfg.PrepBrief.ChatID)
	}

	return svcs
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

// blockerRadarAdapter adapts blocker.Radar to scheduler.BlockerRadarService.
type blockerRadarAdapter struct {
	radar *blocker.Radar
}

func (a *blockerRadarAdapter) NotifyBlockedTasks(ctx context.Context) error {
	_, err := a.radar.NotifyBlockedTasks(ctx)
	return err
}

// prepBriefAdapter adapts prepbrief.Service to scheduler.PrepBriefService.
type prepBriefAdapter struct {
	svc *prepbrief.Service
}

func (a *prepBriefAdapter) GenerateAndSend(ctx context.Context, memberID string) error {
	_, err := a.svc.SendBrief(ctx, memberID)
	return err
}

// buildLeaderLock creates a file-based leader lock if enabled in config.
// The lock file is placed next to the job store file, or in ~/.alex/ as fallback.
func buildLeaderLock(cfg Config, logger logging.Logger) scheduler.LeaderLock {
	schedCfg := cfg.Runtime.Proactive.Scheduler
	if !schedCfg.LeaderLockEnabled {
		return nil
	}

	name := schedCfg.LeaderLockName
	if name == "" {
		name = "proactive_scheduler"
	}

	// Derive lock file path from job store path or home dir.
	lockDir := ""
	if p := strings.TrimSpace(schedCfg.JobStorePath); p != "" {
		lockDir = filepath.Dir(expandHome(p))
	}
	if lockDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			logger.Warn("Scheduler: cannot determine home dir for leader lock: %v", err)
			return nil
		}
		lockDir = filepath.Join(home, ".alex")
	}

	lockPath := filepath.Join(lockDir, name+".lock")
	lock, err := leaderlock.NewFileLock(lockPath, name)
	if err != nil {
		logger.Warn("Scheduler: failed to create leader lock at %s: %v", lockPath, err)
		return nil
	}

	logger.Info("Scheduler: leader lock configured at %s (name=%s)", lockPath, name)
	return lock
}
