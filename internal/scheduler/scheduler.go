package scheduler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"alex/internal/config"
	"alex/internal/logging"
	"alex/internal/tools/builtin/okr"

	"github.com/robfig/cron/v3"
)

// Config holds scheduler configuration.
type Config struct {
	Enabled           bool
	StaticTriggers    []config.SchedulerTriggerConfig
	OKRGoalsRoot      string // path to scan for OKR-derived triggers
	TriggerTimeout    time.Duration
	ConcurrencyPolicy string
}

// Scheduler manages time-based proactive triggers using robfig/cron.
type Scheduler struct {
	cron        *cron.Cron
	coordinator AgentCoordinator
	notifier    Notifier
	goalStore   *okr.GoalStore
	config      Config
	logger      logging.Logger
	mu          sync.Mutex
	entryIDs    map[string]cron.EntryID // trigger name → cron entry
	stopped     chan struct{}
	stopOnce    sync.Once
}

// New creates a new Scheduler.
func New(cfg Config, coordinator AgentCoordinator, notifier Notifier, logger logging.Logger) *Scheduler {
	logger = logging.OrNop(logger)

	var goalStore *okr.GoalStore
	if cfg.OKRGoalsRoot != "" {
		goalStore = okr.NewGoalStore(okr.OKRConfig{GoalsRoot: cfg.OKRGoalsRoot})
	}

	return &Scheduler{
		cron:        newCron(cfg, logger),
		coordinator: coordinator,
		notifier:    notifier,
		goalStore:   goalStore,
		config:      cfg,
		logger:      logger,
		entryIDs:    make(map[string]cron.EntryID),
		stopped:     make(chan struct{}),
	}
}

func newCron(cfg Config, logger logging.Logger) *cron.Cron {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
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
		if err := s.registerTrigger(trigger); err != nil {
			s.logger.Warn("Scheduler: failed to register static trigger %q: %v", t.Name, err)
		}
	}

	// 2. Scan OKR goals and register dynamic triggers
	s.syncOKRTriggersLocked()

	// 3. Start periodic OKR trigger sync (every 5 min)
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

	// 4. Start cron
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

// registerTrigger adds a single trigger to the cron scheduler.
// Must be called with s.mu held.
func (s *Scheduler) registerTrigger(trigger Trigger) error {
	if _, exists := s.entryIDs[trigger.Name]; exists {
		return nil // already registered
	}

	if trigger.Schedule == "" {
		return fmt.Errorf("trigger %q has no schedule", trigger.Name)
	}

	// Capture trigger for closure
	t := trigger
	entryID, err := s.cron.AddFunc(t.Schedule, func() {
		s.executeTrigger(t)
	})
	if err != nil {
		return fmt.Errorf("invalid cron expression for %q: %w", trigger.Name, err)
	}

	s.entryIDs[trigger.Name] = entryID
	s.logger.Info("Scheduler: registered trigger %q (schedule=%s)", trigger.Name, trigger.Schedule)
	return nil
}

// syncOKRTriggers scans OKR goals and registers/prunes triggers.
func (s *Scheduler) syncOKRTriggers() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncOKRTriggersLocked()
}

// syncOKRTriggersLocked performs the OKR trigger sync while s.mu is held.
func (s *Scheduler) syncOKRTriggersLocked() {
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

		if err := s.registerTrigger(trigger); err != nil {
			s.logger.Warn("Scheduler: failed to register OKR trigger %q: %v", triggerName, err)
		}
	}

	// Prune stale OKR triggers
	s.pruneStaleOKRTriggers(activeGoalIDs)
}

// pruneStaleOKRTriggers removes triggers for goals that no longer exist or are no longer active.
func (s *Scheduler) pruneStaleOKRTriggers(activeGoalIDs map[string]bool) {
	for name, entryID := range s.entryIDs {
		if len(name) <= 4 || name[:4] != "okr:" {
			continue // not an OKR trigger
		}
		if activeGoalIDs[name] {
			continue // still active
		}
		s.cron.Remove(entryID)
		delete(s.entryIDs, name)
		s.logger.Info("Scheduler: pruned stale OKR trigger %q", name)
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
