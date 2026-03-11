package kernel

import "time"

// Default kernel engine constants.
const (
	DefaultLeaseSeconds            = 1800 // 30 minutes
	DefaultDispatchRetentionHours  = 24
	DefaultMaxCycleHistory         = 5
	DefaultMaxConcurrentDispatches = 3
	DefaultMinRestartBackoff       = 5 * time.Second
	DefaultMaxRestartBackoff       = 5 * time.Minute
	DefaultAbsenceAlertThreshold   = 2 * time.Hour
	DefaultAlertRepeatInterval     = 10 // every N failures
)

// EngineConfig holds all configurable parameters for the kernel engine.
// Zero values fall back to defaults.
type EngineConfig struct {
	KernelID string

	// Stale recovery: dispatches still running after this duration are
	// considered stale and marked failed.
	LeaseSeconds int

	// Dispatch retention: terminal dispatches older than this are purged.
	DispatchRetentionHours int

	// Cycle history: how many recent cycles to keep in state.
	MaxCycleHistory int

	// Concurrency: max concurrent dispatch executions per cycle.
	MaxConcurrentDispatches int

	// Restart backoff: min and max backoff for engine restart after failure.
	MinRestartBackoff time.Duration
	MaxRestartBackoff time.Duration

	// Alerting: time without a successful cycle before alerting.
	AbsenceAlertThreshold time.Duration
	// Alerting: emit a repeated alert every N consecutive failures.
	AlertRepeatInterval int
}

// LeaseDuration returns the configured lease duration or the default.
func (c EngineConfig) LeaseDuration() time.Duration {
	if c.LeaseSeconds > 0 {
		return time.Duration(c.LeaseSeconds) * time.Second
	}
	return time.Duration(DefaultLeaseSeconds) * time.Second
}

// RetentionPeriod returns the configured retention period or the default.
func (c EngineConfig) RetentionPeriod() time.Duration {
	if c.DispatchRetentionHours > 0 {
		return time.Duration(c.DispatchRetentionHours) * time.Hour
	}
	return time.Duration(DefaultDispatchRetentionHours) * time.Hour
}

func (c EngineConfig) maxCycleHistory() int {
	if c.MaxCycleHistory > 0 {
		return c.MaxCycleHistory
	}
	return DefaultMaxCycleHistory
}

func (c EngineConfig) maxConcurrent() int {
	if c.MaxConcurrentDispatches > 0 {
		return c.MaxConcurrentDispatches
	}
	return DefaultMaxConcurrentDispatches
}

func (c EngineConfig) minBackoff() time.Duration {
	if c.MinRestartBackoff > 0 {
		return c.MinRestartBackoff
	}
	return DefaultMinRestartBackoff
}

func (c EngineConfig) maxBackoff() time.Duration {
	if c.MaxRestartBackoff > 0 {
		return c.MaxRestartBackoff
	}
	return DefaultMaxRestartBackoff
}

func (c EngineConfig) absenceAlert() time.Duration {
	if c.AbsenceAlertThreshold > 0 {
		return c.AbsenceAlertThreshold
	}
	return DefaultAbsenceAlertThreshold
}

func (c EngineConfig) alertRepeat() int {
	if c.AlertRepeatInterval > 0 {
		return c.AlertRepeatInterval
	}
	return DefaultAlertRepeatInterval
}
