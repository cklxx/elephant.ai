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

	weeklyPulseCfg := cfg.Runtime.Proactive.Scheduler.WeeklyPulse
	var weeklyPulseSvc scheduler.WeeklyPulseService
	if weeklyPulseCfg.Enabled && container != nil && container.TaskStore != nil {
		svc := pulse.NewService(container.TaskStore, notifier, weeklyPulseCfg.Channel, weeklyPulseCfg.ChatID)
		weeklyPulseSvc = svc
		logger.Info("Weekly pulse service created (channel=%s, chat_id=%s)", weeklyPulseCfg.Channel, weeklyPulseCfg.ChatID)
	}

	blockerRadarCfg := cfg.Runtime.Proactive.Scheduler.BlockerRadar
	var blockerRadarSvc scheduler.BlockerRadarService
	if blockerRadarCfg.Enabled && container != nil && container.TaskStore != nil {
		radar := blocker.NewRadar(container.TaskStore, notifier, blocker.Config{
			Enabled:               true,
			StaleThresholdSeconds: blockerRadarCfg.StaleThresholdSeconds,
			InputWaitSeconds:      blockerRadarCfg.InputWaitSeconds,
			Channel:               blockerRadarCfg.Channel,
			ChatID:                blockerRadarCfg.ChatID,
		})
		blockerRadarSvc = &blockerRadarAdapter{radar: radar}
		logger.Info("Blocker radar service created (channel=%s, chat_id=%s)", blockerRadarCfg.Channel, blockerRadarCfg.ChatID)
	}

	prepBriefCfg := cfg.Runtime.Proactive.Scheduler.PrepBrief
	var prepBriefSvc scheduler.PrepBriefService
	if prepBriefCfg.Enabled && container != nil && container.TaskStore != nil {
		lookback := time.Duration(prepBriefCfg.LookbackSeconds) * time.Second
		if lookback <= 0 {
			lookback = 7 * 24 * time.Hour
		}
		svc := prepbrief.NewService(container.TaskStore, notifier, prepbrief.Config{
			LookbackDuration: lookback,
			LookbackSeconds:  prepBriefCfg.LookbackSeconds,
			Channel:          prepBriefCfg.Channel,
			ChatID:           prepBriefCfg.ChatID,
		})
		prepBriefSvc = &prepBriefAdapter{svc: svc}
		logger.Info("Prep brief service created (channel=%s, chat_id=%s)", prepBriefCfg.Channel, prepBriefCfg.ChatID)
	}

	schedCfg := scheduler.Config{
		Enabled:             true,
		StaticTriggers:      cfg.Runtime.Proactive.Scheduler.Triggers,
		OKRGoalsRoot:        goalsRoot,
		CalendarReminder:    cfg.Runtime.Proactive.Scheduler.CalendarReminder,
		Heartbeat:           cfg.Runtime.Proactive.Scheduler.Heartbeat,
		MilestoneCheckin:    milestoneCheckinCfg,
		MilestoneService:    milestoneSvc,
		WeeklyPulse:         weeklyPulseCfg,
		WeeklyPulseService:  weeklyPulseSvc,
		BlockerRadar:        blockerRadarCfg,
		BlockerRadarService: blockerRadarSvc,
		PrepBrief:           prepBriefCfg,
		PrepBriefService:    prepBriefSvc,
		TriggerTimeout:      time.Duration(cfg.Runtime.Proactive.Scheduler.TriggerTimeoutSeconds) * time.Second,
		ConcurrencyPolicy:   cfg.Runtime.Proactive.Scheduler.ConcurrencyPolicy,
		JobStore:            jobStore,
		Cooldown:            cooldown,
		MaxConcurrent:       maxConcurrent,
		RecoveryMaxRetries:  recoveryMaxRetries,
		RecoveryBackoff:     recoveryBackoff,
		LeaderLock:          nil, // distributed lock removed (local single-process)
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
