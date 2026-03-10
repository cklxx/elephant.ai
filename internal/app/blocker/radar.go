// Package blocker implements the Blocker Radar — proactive detection and
// notification of stuck, stalled, or blocked tasks.
//
// The Radar periodically scans active tasks and flags those that show no
// progress for a configurable duration, have repeated errors, are waiting
// on unresolved dependencies, or have been waiting for user input too long.
package blocker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"alex/internal/domain/task"
	"alex/internal/shared/logging"
	"alex/internal/shared/notification"
)

const (
	// maxHistoryPerTask is the maximum number of alert records kept per task.
	// Oldest entries are evicted when this limit is exceeded.
	maxHistoryPerTask = 100

	// defaultStaleEntryAge is the default duration after which a task entry
	// with no recent alerts is considered stale and eligible for cleanup.
	defaultStaleEntryAge = 30 * 24 * time.Hour // 30 days
)

// BlockReason classifies why a task is considered blocked.
type BlockReason string

const (
	ReasonStaleProgress  BlockReason = "stale_progress"
	ReasonHasError       BlockReason = "has_error"
	ReasonWaitingInput   BlockReason = "waiting_input"
	ReasonDepBlocked     BlockReason = "dependency_blocked"
)

// Config controls Blocker Radar behavior.
type Config struct {
	Enabled              bool          `json:"enabled" yaml:"enabled"`
	StaleThreshold       time.Duration `json:"-" yaml:"-"`                                         // derived from StaleThresholdSeconds
	StaleThresholdSeconds int          `json:"stale_threshold_seconds" yaml:"stale_threshold_seconds"` // default 1800 (30 min)
	InputWaitThreshold    time.Duration `json:"-" yaml:"-"`                                         // derived
	InputWaitSeconds      int          `json:"input_wait_seconds" yaml:"input_wait_seconds"`         // default 900 (15 min)
	Channel              string        `json:"channel" yaml:"channel"`
	ChatID               string        `json:"chat_id" yaml:"chat_id"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:               false,
		StaleThresholdSeconds: 1800,
		StaleThreshold:        30 * time.Minute,
		InputWaitSeconds:      900,
		InputWaitThreshold:    15 * time.Minute,
		Channel:               "lark",
	}
}

// Alert represents a single blocker detection for one task.
type Alert struct {
	Task   *task.Task
	Reason BlockReason
	Detail string
	Age    time.Duration // how long the blocker condition has persisted
}

// ScanResult holds all detected blockers from a single scan.
type ScanResult struct {
	Alerts      []Alert
	ScannedAt   time.Time
	TasksScanned int
}

// alertRecord is one historical alert entry for a task.
type alertRecord struct {
	Reason BlockReason
	At     time.Time
}

// taskHistory holds bounded alert history for a single task.
type taskHistory struct {
	records []alertRecord
}

// append adds a record, evicting the oldest if at capacity.
func (h *taskHistory) append(r alertRecord) {
	if len(h.records) >= maxHistoryPerTask {
		// Shift left by 1 to drop oldest.
		copy(h.records, h.records[1:])
		h.records[len(h.records)-1] = r
	} else {
		h.records = append(h.records, r)
	}
}

// lastSeen returns the timestamp of the most recent record, or zero.
func (h *taskHistory) lastSeen() time.Time {
	if len(h.records) == 0 {
		return time.Time{}
	}
	return h.records[len(h.records)-1].At
}

// Radar scans active tasks for blocker conditions.
type Radar struct {
	store    task.Store
	notifier notification.Notifier
	config   Config
	logger   logging.Logger
	nowFunc  func() time.Time // for testing

	mu           sync.Mutex
	history      map[string]*taskHistory // taskID → bounded alert history
	lastNotified map[string]time.Time    // "taskID:reason" → last notification time
}

// NewRadar creates a Blocker Radar.
func NewRadar(store task.Store, notifier notification.Notifier, cfg Config) *Radar {
	if cfg.StaleThresholdSeconds > 0 && cfg.StaleThreshold == 0 {
		cfg.StaleThreshold = time.Duration(cfg.StaleThresholdSeconds) * time.Second
	}
	if cfg.StaleThreshold == 0 {
		cfg.StaleThreshold = 30 * time.Minute
	}
	if cfg.InputWaitSeconds > 0 && cfg.InputWaitThreshold == 0 {
		cfg.InputWaitThreshold = time.Duration(cfg.InputWaitSeconds) * time.Second
	}
	if cfg.InputWaitThreshold == 0 {
		cfg.InputWaitThreshold = 15 * time.Minute
	}
	return &Radar{
		store:    store,
		notifier: notifier,
		config:   cfg,
		logger:   logging.NewComponentLogger("blocker_radar"),
		nowFunc:  time.Now,
		history:      make(map[string]*taskHistory),
		lastNotified: make(map[string]time.Time),
	}
}

// Scan inspects all active tasks and returns detected blocker alerts.
func (r *Radar) Scan(ctx context.Context) (*ScanResult, error) {
	now := r.nowFunc()
	result := &ScanResult{ScannedAt: now}

	active, err := r.store.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active tasks: %w", err)
	}
	result.TasksScanned = len(active)

	// Build a lookup of active task IDs for dependency checking.
	terminalIDs := make(map[string]bool)
	allIDs := make(map[string]*task.Task, len(active))
	for _, t := range active {
		allIDs[t.TaskID] = t
	}

	for _, t := range active {
		r.checkStaleProgress(t, now, &result.Alerts)
		r.checkWaitingInput(t, now, &result.Alerts)
		r.checkError(t, &result.Alerts)
		r.checkDependencies(ctx, t, allIDs, terminalIDs, &result.Alerts)
	}

	// Record alerts into bounded per-task history.
	r.mu.Lock()
	for _, a := range result.Alerts {
		h := r.history[a.Task.TaskID]
		if h == nil {
			h = &taskHistory{}
			r.history[a.Task.TaskID] = h
		}
		h.append(alertRecord{Reason: a.Reason, At: now})
	}
	r.mu.Unlock()

	return result, nil
}

func (r *Radar) checkStaleProgress(t *task.Task, now time.Time, alerts *[]Alert) {
	if t.Status != task.StatusRunning {
		return
	}
	age := now.Sub(t.UpdatedAt)
	if age < r.config.StaleThreshold {
		return
	}
	*alerts = append(*alerts, Alert{
		Task:   t,
		Reason: ReasonStaleProgress,
		Detail: fmt.Sprintf("no progress update for %s (last updated %s ago)", formatDuration(r.config.StaleThreshold), formatDuration(age)),
		Age:    age,
	})
}

func (r *Radar) checkWaitingInput(t *task.Task, now time.Time, alerts *[]Alert) {
	if t.Status != task.StatusWaitingInput {
		return
	}
	age := now.Sub(t.UpdatedAt)
	if age < r.config.InputWaitThreshold {
		return
	}
	*alerts = append(*alerts, Alert{
		Task:   t,
		Reason: ReasonWaitingInput,
		Detail: fmt.Sprintf("waiting for user input for %s", formatDuration(age)),
		Age:    age,
	})
}

func (r *Radar) checkError(t *task.Task, alerts *[]Alert) {
	if t.Error == "" {
		return
	}
	// Only flag running/pending tasks with errors (active but erroring).
	if t.Status != task.StatusRunning && t.Status != task.StatusPending {
		return
	}
	*alerts = append(*alerts, Alert{
		Task:   t,
		Reason: ReasonHasError,
		Detail: fmt.Sprintf("task has error: %s", truncate(t.Error, 150)),
	})
}

func (r *Radar) checkDependencies(ctx context.Context, t *task.Task, activeIDs map[string]*task.Task, terminalCache map[string]bool, alerts *[]Alert) {
	if len(t.DependsOn) == 0 {
		return
	}

	var blockedBy []string
	for _, depID := range t.DependsOn {
		// If dep is in the active set, it hasn't completed yet — blocker.
		if _, active := activeIDs[depID]; active {
			blockedBy = append(blockedBy, depID)
			continue
		}
		// Check terminal cache to avoid repeated store lookups.
		if done, cached := terminalCache[depID]; cached {
			if !done {
				blockedBy = append(blockedBy, depID)
			}
			continue
		}
		// Look up the dependency in the store.
		dep, err := r.store.Get(ctx, depID)
		if err != nil {
			// Dependency not found — treat as blocking (missing task).
			terminalCache[depID] = false
			blockedBy = append(blockedBy, depID)
			continue
		}
		if dep.Status == task.StatusCompleted {
			terminalCache[depID] = true
		} else {
			terminalCache[depID] = false
			blockedBy = append(blockedBy, depID)
		}
	}

	if len(blockedBy) > 0 {
		*alerts = append(*alerts, Alert{
			Task:   t,
			Reason: ReasonDepBlocked,
			Detail: fmt.Sprintf("blocked by %d unresolved dependency(ies): %s", len(blockedBy), strings.Join(blockedBy, ", ")),
		})
	}
}

// FormatAlerts renders scan results as a human-readable Markdown string.
func FormatAlerts(result *ScanResult) string {
	if len(result.Alerts) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Blocker Radar — %d alert(s)\n\n", len(result.Alerts)))
	b.WriteString(fmt.Sprintf("Scanned %d active task(s).\n\n", result.TasksScanned))

	for i, a := range result.Alerts {
		desc := truncate(taskLabel(a.Task), 80)
		icon := reasonIcon(a.Reason)
		b.WriteString(fmt.Sprintf("%d. %s **%s** [%s]\n", i+1, icon, desc, a.Task.Status))
		b.WriteString(fmt.Sprintf("   %s\n", a.Detail))
	}

	return b.String()
}

// SendAlerts scans for blockers and sends a notification if any are found.
// Returns the scan result regardless of whether a notification was sent.
func (r *Radar) SendAlerts(ctx context.Context) (*ScanResult, error) {
	result, err := r.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	if len(result.Alerts) == 0 {
		r.logger.Debug("Blocker Radar: no alerts (%d tasks scanned)", result.TasksScanned)
		return result, nil
	}

	content := FormatAlerts(result)

	if r.notifier == nil {
		r.logger.Info("Blocker Radar (%d alerts, no notifier):\n%s", len(result.Alerts), content)
		return result, nil
	}

	target := notification.Target{
		Channel: r.config.Channel,
		ChatID:  r.config.ChatID,
	}
	if err := r.notifier.Send(ctx, target, content); err != nil {
		return result, fmt.Errorf("send alerts: %w", err)
	}

	r.logger.Info("Blocker Radar: sent %d alert(s) to %s/%s", len(result.Alerts), r.config.Channel, r.config.ChatID)
	return result, nil
}

// notifyCooldown is the minimum interval between notifications for the same
// task+reason pair. Alerts within this window are suppressed.
const notifyCooldown = 24 * time.Hour

// NotifyResult holds the outcome of a NotifyBlockedTasks call.
type NotifyResult struct {
	Scanned    int
	Detected   int
	Notified   int
	Suppressed int
}

// NotifyBlockedTasks runs a scan, then sends a per-task Lark notification
// for each blocked task that hasn't been notified in the last 24 hours.
// Each notification includes task details and a suggested action.
func (r *Radar) NotifyBlockedTasks(ctx context.Context) (*NotifyResult, error) {
	result, err := r.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	nr := &NotifyResult{
		Scanned:  result.TasksScanned,
		Detected: len(result.Alerts),
	}

	if len(result.Alerts) == 0 {
		return nr, nil
	}

	now := r.nowFunc()

	for _, a := range result.Alerts {
		key := notifyKey(a.Task.TaskID, a.Reason)
		if r.isRecentlyNotified(key, now) {
			nr.Suppressed++
			continue
		}

		content := FormatTaskNotification(a)

		if r.notifier == nil {
			r.logger.Info("NotifyBlockedTasks (no notifier): %s — %s", a.Task.TaskID, a.Reason)
			r.recordNotified(key, now)
			nr.Notified++
			continue
		}

		target := notification.Target{
			Channel: r.config.Channel,
			ChatID:  r.config.ChatID,
		}
		if err := r.notifier.Send(ctx, target, content); err != nil {
			r.logger.Warn("NotifyBlockedTasks: send failed for task %s: %v", a.Task.TaskID, err)
			continue
		}

		r.recordNotified(key, now)
		nr.Notified++
	}

	r.logger.Info("NotifyBlockedTasks: detected=%d notified=%d suppressed=%d",
		nr.Detected, nr.Notified, nr.Suppressed)
	return nr, nil
}

func notifyKey(taskID string, reason BlockReason) string {
	return taskID + ":" + string(reason)
}

func (r *Radar) isRecentlyNotified(key string, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	ts, ok := r.lastNotified[key]
	return ok && now.Sub(ts) < notifyCooldown
}

func (r *Radar) recordNotified(key string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastNotified[key] = now
}

// FormatTaskNotification renders a single blocked-task notification with
// task details and a suggested action.
func FormatTaskNotification(a Alert) string {
	var b strings.Builder
	icon := reasonIcon(a.Reason)
	desc := truncate(taskLabel(a.Task), 80)

	b.WriteString(fmt.Sprintf("%s **Blocked Task Alert**\n\n", icon))
	b.WriteString(fmt.Sprintf("**Task:** %s\n", desc))
	b.WriteString(fmt.Sprintf("**ID:** %s\n", a.Task.TaskID))
	b.WriteString(fmt.Sprintf("**Status:** %s\n", a.Task.Status))
	b.WriteString(fmt.Sprintf("**Reason:** %s\n", a.Detail))
	if a.Age > 0 {
		b.WriteString(fmt.Sprintf("**Duration:** %s\n", formatDuration(a.Age)))
	}
	b.WriteString(fmt.Sprintf("\n**Suggested action:** %s\n", suggestAction(a.Reason)))

	return b.String()
}

func suggestAction(reason BlockReason) string {
	switch reason {
	case ReasonStaleProgress:
		return "Check if the task is stuck or waiting on an external resource. Consider restarting or reassigning."
	case ReasonHasError:
		return "Review the error details and fix the root cause, then retry the task."
	case ReasonWaitingInput:
		return "Provide the requested input or clarification so the task can proceed."
	case ReasonDepBlocked:
		return "Unblock or complete the dependency tasks, or remove the dependency if no longer needed."
	default:
		return "Investigate the task and take appropriate action."
	}
}

// ReapStaleNotifications removes dedup entries older than the given duration.
func (r *Radar) ReapStaleNotifications(maxAge time.Duration) int {
	if maxAge <= 0 {
		maxAge = 7 * 24 * time.Hour
	}
	now := r.nowFunc()
	cutoff := now.Add(-maxAge)

	r.mu.Lock()
	defer r.mu.Unlock()

	reaped := 0
	for key, ts := range r.lastNotified {
		if ts.Before(cutoff) {
			delete(r.lastNotified, key)
			reaped++
		}
	}
	return reaped
}

// ReapStale removes per-task history entries that have had no alert activity
// for longer than the given duration. This prevents unbounded growth of the
// history map for tasks that have long since completed or been abandoned.
func (r *Radar) ReapStale(maxAge time.Duration) int {
	if maxAge <= 0 {
		maxAge = defaultStaleEntryAge
	}
	now := r.nowFunc()
	cutoff := now.Add(-maxAge)

	r.mu.Lock()
	defer r.mu.Unlock()

	reaped := 0
	for taskID, h := range r.history {
		if h.lastSeen().Before(cutoff) {
			delete(r.history, taskID)
			reaped++
		}
	}
	if reaped > 0 {
		r.logger.Info("Blocker Radar: reaped %d stale task history entries (cutoff=%s)", reaped, maxAge)
	}
	return reaped
}

// HistoryLen returns the number of alert records stored for a task.
// Returns 0 if no history exists for the task.
func (r *Radar) HistoryLen(taskID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.history[taskID]; ok {
		return len(h.records)
	}
	return 0
}

// HistoryTaskCount returns the number of tasks with alert history.
func (r *Radar) HistoryTaskCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.history)
}

func taskLabel(t *task.Task) string {
	if t.Description != "" {
		return t.Description
	}
	return t.TaskID
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatDuration(d time.Duration) string {
	if d >= 24*time.Hour {
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", days)
	}
	if d >= time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	mins := int(d.Minutes())
	if mins <= 1 {
		return "1 minute"
	}
	return fmt.Sprintf("%d minutes", mins)
}

func reasonIcon(r BlockReason) string {
	switch r {
	case ReasonStaleProgress:
		return "⏱"
	case ReasonHasError:
		return "⚠"
	case ReasonWaitingInput:
		return "⏳"
	case ReasonDepBlocked:
		return "🔗"
	default:
		return "!"
	}
}
