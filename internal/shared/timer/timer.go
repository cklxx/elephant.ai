// Package timer provides agent-initiated, runtime-created timers with session
// context resume. Unlike the scheduler package (admin-configured, YAML-based),
// timers are created dynamically by the agent during conversations and fire
// within the originating session context.
package timer

import (
	"fmt"
	"time"

	"alex/internal/shared/utils/id"
)

// TimerType distinguishes one-shot from recurring timers.
type TimerType string

const (
	TimerTypeOnce      TimerType = "once"
	TimerTypeRecurring TimerType = "recurring"
)

// TimerStatus tracks the lifecycle of a timer.
type TimerStatus string

const (
	StatusActive    TimerStatus = "active"
	StatusFired     TimerStatus = "fired"
	StatusCancelled TimerStatus = "cancelled"
)

// Timer represents a scheduled agent task that fires at a specific time or on
// a recurring schedule. When fired, the agent resumes the originating session
// context (full conversation history) and executes the task prompt.
type Timer struct {
	ID        string      `yaml:"id"`
	Name      string      `yaml:"name"`
	Type      TimerType   `yaml:"type"`
	Schedule  string      `yaml:"schedule,omitempty"`
	Delay     string      `yaml:"delay,omitempty"`
	FireAt    time.Time   `yaml:"fire_at"`
	Task      string      `yaml:"task"`
	SessionID string      `yaml:"session_id"`
	Channel   string      `yaml:"channel,omitempty"`
	UserID    string      `yaml:"user_id,omitempty"`
	ChatID    string      `yaml:"chat_id,omitempty"`
	CreatedAt time.Time   `yaml:"created_at"`
	Status    TimerStatus `yaml:"status"`
}

// NewTimerID generates a unique timer identifier with "tmr-" prefix.
func NewTimerID() string {
	return fmt.Sprintf("tmr-%s", id.NewKSUID())
}

// IsActive reports whether the timer is in the active state.
func (t *Timer) IsActive() bool {
	return t.Status == StatusActive
}

// Validate checks that a timer has the required fields and consistent configuration.
func (t *Timer) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("timer ID is required")
	}
	if t.Name == "" {
		return fmt.Errorf("timer name is required")
	}
	if t.Task == "" {
		return fmt.Errorf("timer task is required")
	}
	switch t.Type {
	case TimerTypeOnce:
		if t.FireAt.IsZero() {
			return fmt.Errorf("one-shot timer requires fire_at")
		}
	case TimerTypeRecurring:
		if t.Schedule == "" {
			return fmt.Errorf("recurring timer requires schedule (cron expression)")
		}
	default:
		return fmt.Errorf("invalid timer type: %q", t.Type)
	}
	return nil
}
