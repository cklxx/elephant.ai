package coordinator

import (
	"fmt"
	"strings"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

// stepRecord holds the name and duration of a single execution step.
type stepRecord struct {
	Name       string
	DurationMs float64
}

// stepTimer tracks durations for sequential execution steps.
type stepTimer struct {
	records []stepRecord
	started time.Time
}

// newStepTimer creates a timer and records the overall start time.
func newStepTimer() *stepTimer {
	return &stepTimer{started: time.Now()}
}

// track records a step that started at `start` and finished now.
func (st *stepTimer) track(name string, start time.Time) {
	st.records = append(st.records, stepRecord{
		Name:       name,
		DurationMs: float64(time.Since(start)) / float64(time.Millisecond),
	})
}

// logStep logs a single step's duration via structured logging.
func (st *stepTimer) logStep(logger agent.Logger, taskID string, rec stepRecord) {
	logger.Info("step_timing task_id=%s step=%s duration_ms=%.2f", taskID, rec.Name, rec.DurationMs)
}

// logSummary logs the overall task summary with all step durations.
func (st *stepTimer) logSummary(logger agent.Logger, taskID string) {
	totalMs := float64(time.Since(st.started)) / float64(time.Millisecond)

	parts := make([]string, 0, len(st.records))
	for _, r := range st.records {
		parts = append(parts, fmt.Sprintf("{%s,%.2f}", r.Name, r.DurationMs))
	}
	stepsStr := "[" + strings.Join(parts, ",") + "]"

	logger.Info("task_timing task_id=%s total_ms=%.2f steps=%s", taskID, totalMs, stepsStr)
}
