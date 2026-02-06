package scheduler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"alex/internal/delivery/schedulerapi"
	"alex/internal/infra/tools/builtin/okr"
	"alex/internal/shared/config"
	"alex/internal/shared/logging"

	"github.com/robfig/cron/v3"
)

// Config holds scheduler configuration.
type Config struct {
	Enabled            bool
	StaticTriggers     []config.SchedulerTriggerConfig
	OKRGoalsRoot       string // path to scan for OKR-derived triggers
	CalendarReminder   config.CalendarReminderConfig
	TriggerTimeout     time.Duration
	ConcurrencyPolicy  string
	JobStore           JobStore
	Cooldown           time.Duration
	MaxConcurrent      int
	RecoveryMaxRetries int
	RecoveryBackoff    time.Duration
}

// Scheduler manages time-based proactive triggers using robfig/cron.
type Scheduler struct {
	cron           *cron.Cron
	parser         cron.Parser
	coordinator    AgentCoordinator
	notifier       Notifier
	goalStore      *okr.GoalStore
	jobStore       JobStore
	config         Config
	logger         logging.Logger
	mu             sync.Mutex
	entryIDs       map[string]cron.EntryID // trigger name → cron entry
	jobs           map[string]*Job
	inFlight       map[string]int
	recoveryTimers map[string]*time.Timer
	stopped        chan struct{}
	stopOnce       sync.Once
}

// New creates a new Scheduler.
func New(cfg Config, coordinator AgentCoordinator, notifier Notifier, logger logging.Logger) *Scheduler {
	logger = logging.OrNop(logger)

	var goalStore *okr.GoalStore
	if cfg.OKRGoalsRoot != "" {
		goalStore = okr.NewGoalStore(okr.OKRConfig{GoalsRoot: cfg.OKRGoalsRoot})
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	return &Scheduler{
		cron:           newCron(cfg, logger, parser),
		parser:         parser,
		coordinator:    coordinator,
		notifier:       notifier,
		goalStore:      goalStore,
		jobStore:       cfg.JobStore,
		config:         cfg,
		logger:         logger,
		entryIDs:       make(map[string]cron.EntryID),
		jobs:           make(map[string]*Job),
		inFlight:       make(map[string]int),
		recoveryTimers: make(map[string]*time.Timer),
		stopped:        make(chan struct{}),
	}
}

func newCron(cfg Config, logger logging.Logger, parser cron.Parser) *cron.Cron {
	options := []cron.Option{cron.WithParser(parser)}
	policy := strings.ToLower(strings.TrimSpace(cfg.ConcurrencyPolicy))
	var wrapper cron.JobWrapper
	switch policy {
	case "delay":
		wrapper = cron.DelayIfStillRunning(cron.DefaultLogger)
	case "skip", "":
		wrapper = cron.SkipIfStillRunning(cron.DefaultLogger)
	default:
		logger.Warn("Scheduler: unknown concurrency policy %q, defaulting to skip", policy)
		wrapper = cron.SkipIfStillRunning(cron.DefaultLogger)
	}
	if wrapper != nil {
		options = append(options, cron.WithChain(wrapper))
	}
	return cron.New(options...)
}

// Start registers all triggers and starts the cron scheduler.
func (s *Scheduler) Start(ctx context.Context) error {
	if !s.config.Enabled {
		s.logger.Info("Scheduler disabled by config")
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 0. Load persisted jobs (if configured)
	if s.jobStore != nil {
		if err := s.loadPersistedJobsLocked(ctx); err != nil {
			s.logger.Warn("Scheduler: failed to load persisted jobs: %v", err)
		}
	}

	// 1. Register static triggers from config
	for _, t := range s.config.StaticTriggers {
		trigger := Trigger{
			Name:     t.Name,
			Schedule: t.Schedule,
			Task:     t.Task,
			Channel:  t.Channel,
			UserID:   t.UserID,
			ChatID:   t.ChatID,
		}
		if err := s.registerTriggerLocked(ctx, trigger); err != nil {
			s.logger.Warn("Scheduler: failed to register static trigger %q: %v", t.Name, err)
		}
	}

	// 2. Scan OKR goals and register dynamic triggers
	s.syncOKRTriggersLocked(ctx)

	// 3. Register calendar reminder trigger
	s.registerCalendarTrigger(ctx)

	// 4. Start periodic OKR trigger sync (every 5 min)
	if s.goalStore != nil {
		syncEntryID, err := s.cron.AddFunc("*/5 * * * *", func() {
			s.syncOKRTriggers()
		})
		if err != nil {
			s.logger.Warn("Scheduler: failed to register OKR sync job: %v", err)
		} else {
			s.entryIDs["_okr_sync"] = syncEntryID
		}
	}

	// 5. Start cron
	s.cron.Start()
	s.logger.Info("Scheduler started with %d triggers", len(s.entryIDs))

	// Wait for context cancellation
	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	return nil
}

// Stop gracefully stops the scheduler. Safe to call multiple times.
func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		s.logger.Info("Scheduler stopping...")
		s.mu.Lock()
		s.stopRecoveryTimersLocked()
		s.mu.Unlock()
		stopCtx := s.cron.Stop()
		<-stopCtx.Done()
		close(s.stopped)
		s.logger.Info("Scheduler stopped")
	})
}

// Done returns a channel that is closed when the scheduler has fully stopped.
func (s *Scheduler) Done() <-chan struct{} {
	return s.stopped
}

// Name returns the subsystem name for lifecycle management.
func (s *Scheduler) Name() string {
	return "scheduler"
}

// Drain gracefully stops the scheduler, waiting for in-flight triggers to
// complete. The provided context carries a deadline that Drain respects:
// if the deadline expires before all triggers finish, Drain returns a
// context error.
func (s *Scheduler) Drain(ctx context.Context) error {
	s.logger.Info("Scheduler draining...")

	// Stop the cron runner; this prevents new triggers from firing and
	// returns a context that completes when all running jobs finish.
	s.mu.Lock()
	s.stopRecoveryTimersLocked()
	s.mu.Unlock()
	cronDone := s.cron.Stop()

	select {
	case <-cronDone.Done():
		// All in-flight triggers completed.
		s.stopOnce.Do(func() {
			close(s.stopped)
		})
		s.logger.Info("Scheduler drained successfully")
		return nil
	case <-ctx.Done():
		// Deadline exceeded while waiting for in-flight triggers.
		s.stopOnce.Do(func() {
			close(s.stopped)
		})
		s.logger.Warn("Scheduler drain timed out: %v", ctx.Err())
		return fmt.Errorf("scheduler drain: %w", ctx.Err())
	}
}

// registerTriggerLocked adds a single trigger to the cron scheduler.
// Must be called with s.mu held.
func (s *Scheduler) registerTriggerLocked(ctx context.Context, trigger Trigger) error {
	if trigger.Name == "" {
		return fmt.Errorf("trigger has no name")
	}
	if trigger.Schedule == "" {
		return fmt.Errorf("trigger %q has no schedule", trigger.Name)
	}

	job, err := s.ensureJobForTriggerLocked(ctx, trigger)
	if err != nil {
		return err
	}
	if job.Status == JobStatusPaused || job.Status == JobStatusCompleted {
		s.logger.Info("Scheduler: job %q is %s, not scheduling", job.ID, job.Status)
		return nil
	}

	return s.registerJobLocked(ctx, job)
}

// syncOKRTriggers scans OKR goals and registers/prunes triggers.
func (s *Scheduler) syncOKRTriggers() {
	s.syncOKRTriggersWithContext(context.Background())
}

func (s *Scheduler) syncOKRTriggersWithContext(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncOKRTriggersLocked(ctx)
}

// syncOKRTriggersLocked performs the OKR trigger sync while s.mu is held.
func (s *Scheduler) syncOKRTriggersLocked(ctx context.Context) {
	if s.goalStore == nil {
		return
	}

	goals, err := s.goalStore.ListGoals()
	if err != nil {
		s.logger.Warn("Scheduler: failed to list OKR goals: %v", err)
		return
	}

	activeGoalIDs := make(map[string]bool)

	for _, goalID := range goals {
		goal, err := s.goalStore.ReadGoal(goalID)
		if err != nil || goal.Meta.Status != "active" {
			continue
		}
		if goal.Meta.ReviewCadence == "" {
			continue
		}

		triggerName := "okr:" + goalID
		activeGoalIDs[triggerName] = true

		// Already registered? Skip.
		if _, exists := s.entryIDs[triggerName]; exists {
			continue
		}

		trigger := Trigger{
			Name:     triggerName,
			Schedule: goal.Meta.ReviewCadence,
			Task:     fmt.Sprintf("Run OKR review tick for goal %s. Read the goal with okr_read, evaluate KR progress, and produce a status dashboard.", goalID),
			Channel:  goal.Meta.Notifications.Channel,
			UserID:   goal.Meta.Owner,
			ChatID:   goal.Meta.Notifications.LarkChatID,
			GoalID:   goalID,
		}

		if err := s.registerTriggerLocked(ctx, trigger); err != nil {
			s.logger.Warn("Scheduler: failed to register OKR trigger %q: %v", triggerName, err)
		}
	}

	// Prune stale OKR triggers
	s.pruneStaleOKRTriggers(ctx, activeGoalIDs)
}

// pruneStaleOKRTriggers removes triggers for goals that no longer exist or are no longer active.
func (s *Scheduler) pruneStaleOKRTriggers(ctx context.Context, activeGoalIDs map[string]bool) {
	for name, entryID := range s.entryIDs {
		if len(name) <= 4 || name[:4] != "okr:" {
			continue // not an OKR trigger
		}
		if activeGoalIDs[name] {
			continue // still active
		}
		s.cron.Remove(entryID)
		delete(s.entryIDs, name)
		if s.jobStore != nil {
			if err := s.jobStore.Delete(ctx, name); err != nil {
				s.logger.Warn("Scheduler: failed to delete OKR job %q: %v", name, err)
			}
		}
		delete(s.jobs, name)
		if timer := s.recoveryTimers[name]; timer != nil {
			timer.Stop()
			delete(s.recoveryTimers, name)
		}
		s.logger.Info("Scheduler: pruned stale OKR trigger %q", name)
	}
}

// RegisterDynamicTrigger registers a new trigger at runtime (e.g. from an
// agent tool call). It creates the corresponding Job, persists it, and
// schedules it in the cron runner. Returns a schedulerapi.Job DTO so callers
// can inspect the computed NextRun time without importing this package.
func (s *Scheduler) RegisterDynamicTrigger(ctx context.Context, name, schedule, task, channel string) (*schedulerapi.Job, error) {
	trigger := Trigger{
		Name:     name,
		Schedule: schedule,
		Task:     task,
		Channel:  channel,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if trigger.Name == "" {
		return nil, fmt.Errorf("trigger has no name")
	}
	if trigger.Schedule == "" {
		return nil, fmt.Errorf("trigger %q has no schedule", trigger.Name)
	}

	job, err := s.ensureJobForTriggerLocked(ctx, trigger)
	if err != nil {
		return nil, err
	}
	if err := s.registerJobLocked(ctx, job); err != nil {
		return nil, err
	}
	return jobToDTO(job), nil
}

// UnregisterTrigger removes a trigger (and its associated Job) by name. The
// cron entry is removed, the in-memory job map is cleaned, any pending
// recovery timer is cancelled, and the persisted job is deleted from the
// store.
func (s *Scheduler) UnregisterTrigger(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.entryIDs[name]; ok {
		s.cron.Remove(entryID)
		delete(s.entryIDs, name)
	}

	delete(s.jobs, name)

	if timer := s.recoveryTimers[name]; timer != nil {
		timer.Stop()
		delete(s.recoveryTimers, name)
	}

	if s.jobStore != nil {
		if err := s.jobStore.Delete(ctx, name); err != nil {
			return err
		}
	}

	s.logger.Info("Scheduler: unregistered trigger %q", name)
	return nil
}

// ListJobs returns all persisted jobs as DTOs. Returns an error if no store
// is configured.
func (s *Scheduler) ListJobs(ctx context.Context) ([]schedulerapi.Job, error) {
	if s.jobStore == nil {
		return nil, fmt.Errorf("job store not configured")
	}
	jobs, err := s.jobStore.List(ctx)
	if err != nil {
		return nil, err
	}
	dtos := make([]schedulerapi.Job, len(jobs))
	for i := range jobs {
		dtos[i] = *jobToDTO(&jobs[i])
	}
	return dtos, nil
}

// LoadJob loads a single job by ID from the job store as a DTO.
func (s *Scheduler) LoadJob(ctx context.Context, id string) (*schedulerapi.Job, error) {
	if s.jobStore == nil {
		return nil, fmt.Errorf("job store not configured")
	}
	job, err := s.jobStore.Load(ctx, id)
	if err != nil {
		return nil, err
	}
	return jobToDTO(job), nil
}

// CronParser returns the scheduler's cron parser for external validation.
func (s *Scheduler) CronParser() cron.Parser {
	return s.parser
}

// jobToDTO converts an internal Job to a schedulerapi.Job DTO.
func jobToDTO(j *Job) *schedulerapi.Job {
	return &schedulerapi.Job{
		ID:           j.ID,
		Name:         j.Name,
		CronExpr:     j.CronExpr,
		Trigger:      j.Trigger,
		Payload:      j.Payload,
		Status:       string(j.Status),
		LastRun:      j.LastRun,
		NextRun:      j.NextRun,
		FailureCount: j.FailureCount,
		LastFailure:  j.LastFailure,
		LastError:    j.LastError,
		CreatedAt:    j.CreatedAt,
		UpdatedAt:    j.UpdatedAt,
	}
}

// TriggerCount returns the number of registered triggers.
func (s *Scheduler) TriggerCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entryIDs)
}

// TriggerNames returns the names of all registered triggers.
func (s *Scheduler) TriggerNames() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	names := make([]string, 0, len(s.entryIDs))
	for name := range s.entryIDs {
		names = append(names, name)
	}
	return names
}
