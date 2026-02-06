package scheduler

import (
	"context"
	"fmt"
	"time"
)

type jobRunOptions struct {
	bypassCooldown bool
}

func (s *Scheduler) loadPersistedJobsLocked(ctx context.Context) error {
	jobs, err := s.jobStore.List(ctx)
	if err != nil {
		return fmt.Errorf("list jobs: %w", err)
	}
	for i := range jobs {
		job := jobs[i]
		if !job.Status.IsValid() {
			job.Status = JobStatusActive
		}
		s.jobs[job.ID] = &job
		if job.Status == JobStatusPaused || job.Status == JobStatusCompleted {
			continue
		}
		if err := s.registerJobLocked(ctx, &job); err != nil {
			s.logger.Warn("Scheduler: failed to register persisted job %q: %v", job.ID, err)
		}
		s.scheduleRecoveryFromJobLocked(ctx, &job)
	}
	return nil
}

func (s *Scheduler) ensureJobForTriggerLocked(ctx context.Context, trigger Trigger) (*Job, error) {
	job, err := s.buildJobFromTrigger(trigger)
	if err != nil {
		return nil, err
	}
	if existing, ok := s.jobs[job.ID]; ok && existing != nil {
		job = mergeJobRuntime(existing, job)
	}
	if !job.Status.IsValid() {
		job.Status = JobStatusActive
	}
	s.jobs[job.ID] = &job
	s.persistJobLocked(ctx, &job)
	return &job, nil
}

func (s *Scheduler) buildJobFromTrigger(trigger Trigger) (Job, error) {
	payload, err := payloadFromTrigger(trigger)
	if err != nil {
		return Job{}, err
	}
	return Job{
		ID:       trigger.Name,
		Name:     trigger.Name,
		CronExpr: trigger.Schedule,
		Trigger:  trigger.Task,
		Payload:  payload,
		Status:   JobStatusActive,
	}, nil
}

func mergeJobRuntime(existing *Job, desired Job) Job {
	if existing == nil {
		return desired
	}
	desired.CreatedAt = existing.CreatedAt
	desired.UpdatedAt = existing.UpdatedAt
	desired.LastRun = existing.LastRun
	desired.NextRun = existing.NextRun
	desired.Status = existing.Status
	desired.FailureCount = existing.FailureCount
	desired.LastFailure = existing.LastFailure
	desired.LastError = existing.LastError
	return desired
}

func (s *Scheduler) registerJobLocked(ctx context.Context, job *Job) error {
	if _, exists := s.entryIDs[job.ID]; exists {
		return nil
	}
	if job.CronExpr == "" {
		return fmt.Errorf("job %q has no schedule", job.ID)
	}
	if _, err := s.parser.Parse(job.CronExpr); err != nil {
		return fmt.Errorf("invalid cron expression for %q: %w", job.ID, err)
	}

	jobID := job.ID
	entryID, err := s.cron.AddFunc(job.CronExpr, func() {
		s.runJob(jobID, jobRunOptions{})
	})
	if err != nil {
		return fmt.Errorf("register cron for %q: %w", job.ID, err)
	}
	s.entryIDs[job.ID] = entryID

	nextRun, err := s.nextRun(job.CronExpr, time.Now().UTC())
	if err == nil {
		job.NextRun = nextRun
		s.persistJobLocked(ctx, job)
	}

	s.logger.Info("Scheduler: registered trigger %q (schedule=%s)", job.ID, job.CronExpr)
	return nil
}

func (s *Scheduler) persistJobLocked(ctx context.Context, job *Job) {
	if s.jobStore == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.jobStore.Save(ctx, *job); err != nil {
		s.logger.Warn("Scheduler: failed to persist job %q: %v", job.ID, err)
	}
}

func (s *Scheduler) runJob(jobID string, opts jobRunOptions) bool {
	select {
	case <-s.stopped:
		return false
	default:
	}

	_, trigger, ok := s.startJob(jobID, opts)
	if !ok {
		return false
	}

	err := s.executeTrigger(trigger)
	s.finishJob(jobID, err)
	return true
}

func (s *Scheduler) startJob(jobID string, opts jobRunOptions) (*Job, Trigger, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job := s.jobs[jobID]
	if job == nil {
		return nil, Trigger{}, false
	}
	if job.Status == JobStatusPaused || job.Status == JobStatusCompleted {
		return nil, Trigger{}, false
	}

	if !opts.bypassCooldown && s.config.Cooldown > 0 && !job.LastRun.IsZero() {
		if time.Since(job.LastRun) < s.config.Cooldown {
			s.logger.Debug("Scheduler: skipping %q due to cooldown", jobID)
			return nil, Trigger{}, false
		}
	}

	maxConcurrent := s.config.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}
	if s.inFlight[jobID] >= maxConcurrent {
		s.logger.Debug("Scheduler: skipping %q due to concurrency limit", jobID)
		return nil, Trigger{}, false
	}

	trigger, err := triggerFromJob(*job)
	if err != nil {
		s.logger.Warn("Scheduler: failed to decode job %q payload: %v", jobID, err)
		return nil, Trigger{}, false
	}

	s.inFlight[jobID]++
	now := time.Now().UTC()
	job.LastRun = now
	job.Status = JobStatusActive
	job.UpdatedAt = now
	if nextRun, err := s.nextRun(job.CronExpr, now); err == nil {
		job.NextRun = nextRun
	}
	s.persistJobLocked(context.Background(), job)

	jobCopy := *job
	return &jobCopy, trigger, true
}

func (s *Scheduler) finishJob(jobID string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.inFlight[jobID] > 0 {
		s.inFlight[jobID]--
	}

	job := s.jobs[jobID]
	if job == nil {
		return
	}

	now := time.Now().UTC()
	job.UpdatedAt = now

	if err != nil {
		job.FailureCount++
		job.LastFailure = now
		job.LastError = err.Error()
		s.scheduleRecoveryLocked(context.Background(), job)
	} else {
		job.FailureCount = 0
		job.LastFailure = time.Time{}
		job.LastError = ""
		if timer := s.recoveryTimers[jobID]; timer != nil {
			timer.Stop()
			delete(s.recoveryTimers, jobID)
		}
	}

	s.persistJobLocked(context.Background(), job)
}

func (s *Scheduler) scheduleRecoveryLocked(ctx context.Context, job *Job) {
	if s.config.RecoveryMaxRetries <= 0 {
		return
	}
	if job.FailureCount > s.config.RecoveryMaxRetries {
		job.Status = JobStatusPaused
		s.persistJobLocked(ctx, job)
		s.logger.Warn("Scheduler: job %q paused after %d failures", job.ID, job.FailureCount)
		return
	}
	if job.LastFailure.IsZero() {
		return
	}
	delay := s.recoveryDelay(job.FailureCount)
	s.scheduleRecoveryTimerLocked(job, delay)
}

func (s *Scheduler) scheduleRecoveryFromJobLocked(ctx context.Context, job *Job) {
	if s.config.RecoveryMaxRetries <= 0 || job.FailureCount <= 0 {
		return
	}
	if job.FailureCount > s.config.RecoveryMaxRetries {
		job.Status = JobStatusPaused
		s.persistJobLocked(ctx, job)
		s.logger.Warn("Scheduler: job %q paused after %d failures", job.ID, job.FailureCount)
		return
	}
	if job.LastFailure.IsZero() {
		return
	}
	elapsed := time.Since(job.LastFailure)
	remaining := s.recoveryDelay(job.FailureCount) - elapsed
	if remaining < 0 {
		remaining = 0
	}
	s.scheduleRecoveryTimerLocked(job, remaining)
}

func (s *Scheduler) scheduleRecoveryTimerLocked(job *Job, delay time.Duration) {
	if timer := s.recoveryTimers[job.ID]; timer != nil {
		timer.Stop()
	}

	jobID := job.ID
	timer := time.AfterFunc(delay, func() {
		select {
		case <-s.stopped:
			return
		default:
		}
		s.runJob(jobID, jobRunOptions{bypassCooldown: true})
	})
	s.recoveryTimers[job.ID] = timer
}

func (s *Scheduler) stopRecoveryTimersLocked() {
	for id, timer := range s.recoveryTimers {
		timer.Stop()
		delete(s.recoveryTimers, id)
	}
}

func (s *Scheduler) recoveryDelay(failureCount int) time.Duration {
	backoff := s.config.RecoveryBackoff
	if backoff <= 0 {
		backoff = time.Minute
	}
	if failureCount < 1 {
		failureCount = 1
	}
	return time.Duration(failureCount) * backoff
}

func (s *Scheduler) nextRun(expr string, now time.Time) (time.Time, error) {
	schedule, err := s.parser.Parse(expr)
	if err != nil {
		return time.Time{}, err
	}
	return schedule.Next(now), nil
}
