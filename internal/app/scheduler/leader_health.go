package scheduler

import (
	"fmt"
	"time"
)

// leaderJobNames lists the well-known leader agent cron jobs.
var leaderJobNames = []string{
	blockerRadarTriggerName,
	weeklyPulseTriggerName,
	milestoneTriggerName,
	prepBriefTriggerName,
}

// leaderJobResult tracks the outcome of the most recent execution of a leader job.
type leaderJobResult struct {
	lastRun time.Time
	lastErr string // empty on success
}

// recordLeaderResult records the outcome of a leader job execution.
// Safe to call concurrently — acquires s.mu internally.
func (s *Scheduler) recordLeaderResult(name string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := &leaderJobResult{lastRun: time.Now()}
	if err != nil {
		r.lastErr = err.Error()
	}
	s.leaderResults[name] = r
}

// LeaderJobStatus describes the health of a single leader agent cron job.
type LeaderJobStatus struct {
	Name       string    `json:"name"`
	Registered bool      `json:"registered"`
	LastRun    time.Time `json:"last_run,omitempty"`
	NextRun    time.Time `json:"next_run,omitempty"`
	LastError  string    `json:"last_error,omitempty"`
	Healthy    bool      `json:"healthy"`
}

// LeaderJobsHealth returns the health status of all leader agent cron jobs.
func (s *Scheduler) LeaderJobsHealth() []LeaderJobStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	statuses := make([]LeaderJobStatus, 0, len(leaderJobNames))
	for _, name := range leaderJobNames {
		st := LeaderJobStatus{Name: name}

		entryID, registered := s.entryIDs[name]
		st.Registered = registered

		if registered {
			entry := s.cron.Entry(entryID)
			st.NextRun = entry.Next
			st.LastRun = entry.Prev
		}

		// Overlay recorded result if available (more accurate than cron.Prev).
		if r := s.leaderResults[name]; r != nil {
			if r.lastRun.After(st.LastRun) {
				st.LastRun = r.lastRun
			}
			st.LastError = r.lastErr
		}

		// Healthy = registered AND last execution (if any) succeeded.
		st.Healthy = registered && st.LastError == ""

		statuses = append(statuses, st)
	}
	return statuses
}

// Running reports whether the scheduler's cron runner is active.
func (s *Scheduler) Running() bool {
	select {
	case <-s.stopped:
		return false
	default:
		return s.config.Enabled
	}
}

// displayName converts internal trigger names to human-readable labels.
func DisplayName(name string) string {
	switch name {
	case blockerRadarTriggerName:
		return "blocker_radar"
	case weeklyPulseTriggerName:
		return "weekly_pulse"
	case milestoneTriggerName:
		return "milestone_checkin"
	case prepBriefTriggerName:
		return "prep_brief"
	default:
		return name
	}
}

// HealthSummary returns a single-line summary of leader job health.
func HealthSummary(statuses []LeaderJobStatus) string {
	total := len(statuses)
	healthy := 0
	registered := 0
	for _, s := range statuses {
		if s.Registered {
			registered++
		}
		if s.Healthy {
			healthy++
		}
	}
	return fmt.Sprintf("%d/%d leader jobs healthy (%d registered)", healthy, total, registered)
}
