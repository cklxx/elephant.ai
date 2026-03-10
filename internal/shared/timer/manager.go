package timer

import (
	"context"
	"fmt"
	"sync"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/async"
	"alex/internal/shared/logging"
	"alex/internal/shared/notification"
	id "alex/internal/shared/utils/id"

	"github.com/robfig/cron/v3"
)

// AgentCoordinator is the subset of the coordinator interface needed by the timer manager.
type AgentCoordinator interface {
	ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error)
}

// Notifier routes timer results to external channels.
type Notifier = notification.Notifier

// Config holds TimerManager runtime configuration.
type Config struct {
	Enabled     bool
	StorePath   string
	MaxTimers   int
	TaskTimeout time.Duration
}

// TimerManager manages the lifecycle of agent-initiated timers.
// It handles scheduling, persistence, firing, and restart recovery.
type TimerManager struct {
	coordinator AgentCoordinator
	notifier    Notifier
	store       *Store
	config      Config
	logger      logging.Logger
	cron        *cron.Cron

	mu        sync.Mutex
	timers    map[string]*Timer
	cronIDs   map[string]cron.EntryID
	goTimers  map[string]*time.Timer
	stopWatch func() bool
	isStopped bool
	stopped   chan struct{}
	stopOnce  sync.Once
}

// ToolTimerManagerServiceMarker lets TimerManager cross the tool context
// boundary without the shared context package importing timer types.
func (*TimerManager) ToolTimerManagerServiceMarker() {}

// NewTimerManager creates a new TimerManager.
func NewTimerManager(cfg Config, coordinator AgentCoordinator, notifier Notifier, logger logging.Logger) (*TimerManager, error) {
	logger = logging.OrNop(logger)

	store, err := NewStore(cfg.StorePath)
	if err != nil {
		return nil, fmt.Errorf("create timer store: %w", err)
	}

	cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	cronInstance := cron.New(
		cron.WithParser(cronParser),
		cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)),
	)

	return &TimerManager{
		coordinator: coordinator,
		notifier:    notifier,
		store:       store,
		config:      cfg,
		logger:      logger,
		cron:        cronInstance,
		timers:      make(map[string]*Timer),
		cronIDs:     make(map[string]cron.EntryID),
		goTimers:    make(map[string]*time.Timer),
		stopped:     make(chan struct{}),
	}, nil
}

// Start loads persisted timers, re-schedules active ones, and starts the cron engine.
func (m *TimerManager) Start(ctx context.Context) error {
	if !m.config.Enabled {
		m.logger.Info("TimerManager disabled by config")
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Load persisted timers.
	timers, err := m.store.LoadAll()
	if err != nil {
		return fmt.Errorf("load timers: %w", err)
	}

	now := time.Now()
	for i := range timers {
		timer := timers[i]
		if !timer.IsActive() {
			continue
		}
		m.timers[timer.ID] = &timer
		if err := m.scheduleLocked(m.timers[timer.ID]); err != nil {
			m.logger.Warn("TimerManager: failed to re-schedule timer %q: %v", timer.Name, err)
		}
	}

	m.stopWatch = context.AfterFunc(ctx, m.Stop)
	m.cron.Start()

	m.logger.Info("TimerManager started with %d active timers (recovered at %s)", len(m.timers), now.Format(time.RFC3339))

	return nil
}

// Stop gracefully stops the manager. Safe to call multiple times.
func (m *TimerManager) Stop() {
	m.stopOnce.Do(func() {
		m.logger.Info("TimerManager stopping...")

		m.mu.Lock()
		m.isStopped = true
		if m.stopWatch != nil {
			m.stopWatch()
			m.stopWatch = nil
		}
		// Cancel all pending one-shot timers.
		for timerID, goTimer := range m.goTimers {
			goTimer.Stop()
			delete(m.goTimers, timerID)
		}
		m.mu.Unlock()

		// Stop cron (waits for running jobs to finish).
		stopCtx := m.cron.Stop()
		<-stopCtx.Done()

		close(m.stopped)
		m.logger.Info("TimerManager stopped")
	})
}

// Done returns a channel that is closed when the manager has fully stopped.
func (m *TimerManager) Done() <-chan struct{} {
	return m.stopped
}

// Add validates, persists, and schedules a new timer.
func (m *TimerManager) Add(t *Timer) error {
	if err := t.Validate(); err != nil {
		return fmt.Errorf("invalid timer: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.isStopped {
		return fmt.Errorf("timer manager stopped")
	}

	// Enforce max timer limit (count only active timers).
	// Fast path: if total entries (including inactive) are below the limit,
	// the active count is guaranteed to be below it too — skip the scan.
	if m.config.MaxTimers > 0 && len(m.timers) >= m.config.MaxTimers {
		activeCount := 0
		for _, existing := range m.timers {
			if existing.IsActive() {
				activeCount++
			}
		}
		if activeCount >= m.config.MaxTimers {
			return fmt.Errorf("maximum active timer limit reached (%d)", m.config.MaxTimers)
		}
	}

	// Persist first so we survive crashes between persist and schedule.
	if err := m.store.Save(*t); err != nil {
		return fmt.Errorf("persist timer: %w", err)
	}

	m.timers[t.ID] = t

	if err := m.scheduleLocked(t); err != nil {
		return fmt.Errorf("schedule timer: %w", err)
	}

	m.logger.Info("TimerManager: added timer %q (%s)", t.Name, t.ID)
	return nil
}

// Cancel marks a timer as cancelled, stops its scheduling, and persists the change.
func (m *TimerManager) Cancel(timerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.timers[timerID]
	if !ok {
		return fmt.Errorf("timer not found: %s", timerID)
	}
	if !t.IsActive() {
		return fmt.Errorf("timer %s is not active (status=%s)", timerID, t.Status)
	}

	m.unscheduleLocked(timerID)
	t.Status = StatusCancelled
	if err := m.store.Save(*t); err != nil {
		m.logger.Warn("TimerManager: failed to persist cancellation for %s: %v", timerID, err)
	}

	m.logger.Info("TimerManager: cancelled timer %q (%s)", t.Name, timerID)
	return nil
}

// List returns timers filtered by user ID. If userID is empty, returns all.
func (m *TimerManager) List(userID string) []Timer {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []Timer
	for _, t := range m.timers {
		if userID != "" && t.UserID != userID {
			continue
		}
		result = append(result, *t)
	}
	return result
}

// Get returns a single timer by ID.
func (m *TimerManager) Get(timerID string) (Timer, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.timers[timerID]
	if !ok {
		return Timer{}, false
	}
	return *t, true
}

// scheduleOneShotLocked creates a Go timer that fires after the given duration.
// Must be called with m.mu held.
func (m *TimerManager) scheduleOneShotLocked(t *Timer, delay time.Duration) {
	goTimer := time.AfterFunc(delay, func() {
		async.Run(m.logger, "timer.fire", func() {
			m.fireTimer(t.ID)
		})
	})
	m.goTimers[t.ID] = goTimer
}

// scheduleRecurringLocked registers a timer with the cron engine.
// Must be called with m.mu held.
func (m *TimerManager) scheduleRecurringLocked(t *Timer) error {
	entryID, err := m.cron.AddFunc(t.Schedule, func() {
		async.Run(m.logger, "timer.fire", func() {
			m.fireTimer(t.ID)
		})
	})
	if err != nil {
		return fmt.Errorf("invalid cron expression for %q: %w", t.Name, err)
	}
	m.cronIDs[t.ID] = entryID
	return nil
}

func (m *TimerManager) scheduleLocked(t *Timer) error {
	switch t.Type {
	case TimerTypeOnce:
		if delay := time.Until(t.FireAt); delay <= 0 {
			m.logger.Info("TimerManager: timer %q past due, firing immediately", t.Name)
			m.fireAsync(t.ID)
			return nil
		} else {
			m.scheduleOneShotLocked(t, delay)
			return nil
		}
	case TimerTypeRecurring:
		return m.scheduleRecurringLocked(t)
	default:
		return fmt.Errorf("invalid timer type: %q", t.Type)
	}
}

func (m *TimerManager) unscheduleLocked(timerID string) {
	if goTimer, ok := m.goTimers[timerID]; ok {
		goTimer.Stop()
		delete(m.goTimers, timerID)
	}
	if cronID, ok := m.cronIDs[timerID]; ok {
		m.cron.Remove(cronID)
		delete(m.cronIDs, timerID)
	}
}

func (m *TimerManager) fireAsync(timerID string) {
	async.Go(m.logger, "timer.fire", func() {
		m.fireTimer(timerID)
	})
}

func (m *TimerManager) timerForFire(timerID string) (*Timer, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isStopped {
		return nil, false
	}
	t, ok := m.timers[timerID]
	if !ok || !t.IsActive() {
		return nil, false
	}
	if t.Type == TimerTypeOnce {
		delete(m.goTimers, timerID)
	}
	return t, true
}

// fireTimer executes the timer's task within the originating session context.
func (m *TimerManager) fireTimer(timerID string) {
	t, ok := m.timerForFire(timerID)
	if !ok {
		return
	}

	ctx := context.Background()
	ctx = id.MarkUnattendedContext(ctx)
	if t.UserID != "" {
		ctx = id.WithUserID(ctx, t.UserID)
	}

	runID := id.NewRunID()
	ctx = id.WithRunID(ctx, runID)

	// Resume originating session context.
	sessionID := t.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("timer-%s-%s", t.ID, runID)
	}
	ctx = id.WithSessionID(ctx, sessionID)

	if m.config.TaskTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.config.TaskTimeout)
		defer cancel()
	}

	m.logger.Info("TimerManager: firing timer %q (%s) in session %s", t.Name, t.ID, sessionID)

	result, err := m.coordinator.ExecuteTask(ctx, t.Task, sessionID, nil)
	content := formatTimerResult(t, result, err)

	// Notify.
	m.notify(ctx, t, content)

	// Mark one-shot as fired.
	if t.Type == TimerTypeOnce {
		m.mu.Lock()
		if t.IsActive() {
			t.Status = StatusFired
		}
		m.mu.Unlock()
		if saveErr := m.store.Save(*t); saveErr != nil {
			m.logger.Warn("TimerManager: failed to persist fired status for %s: %v", t.ID, saveErr)
		}
	}
}

func (m *TimerManager) notify(ctx context.Context, t *Timer, content string) {
	if m.notifier == nil || t.Channel == "" {
		return
	}
	target := notification.Target{Channel: t.Channel, ChatID: t.ChatID}
	if err := m.notifier.Send(ctx, target, content); err != nil {
		m.logger.Warn("TimerManager: notification failed for %q: %v", t.Name, err)
	}
}

func formatTimerResult(t *Timer, result *agent.TaskResult, err error) string {
	if err != nil {
		return fmt.Sprintf("Timer '%s' task failed: %v", t.Name, err)
	}
	if result == nil {
		return fmt.Sprintf("Timer '%s' task completed (no result).", t.Name)
	}
	return result.Answer
}
