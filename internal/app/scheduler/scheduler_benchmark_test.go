package scheduler

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

type benchmarkCoordinator struct {
	delay time.Duration
}

func (c *benchmarkCoordinator) ExecuteTask(_ context.Context, _ string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	return &agent.TaskResult{Answer: "ok"}, nil
}

func benchmarkSchedulerWithJobs(b *testing.B, jobCount int, cfg Config, coordinator AgentCoordinator) (*Scheduler, []string) {
	b.Helper()

	sched := New(cfg, coordinator, nil, nil)
	ctx := context.Background()
	jobIDs := make([]string, 0, jobCount)

	sched.mu.Lock()
	defer sched.mu.Unlock()
	for i := 0; i < jobCount; i++ {
		jobID := fmt.Sprintf("bench-job-%d", i)
		if err := sched.registerTriggerLocked(ctx, Trigger{
			Name:     jobID,
			Schedule: "* * * * *",
			Task:     "benchmark scheduler task",
		}); err != nil {
			b.Fatalf("registerTriggerLocked(%s): %v", jobID, err)
		}
		jobIDs = append(jobIDs, jobID)
	}

	return sched, jobIDs
}

func benchmarkJob(jobID string) Job {
	return Job{
		ID:       jobID,
		Name:     jobID,
		CronExpr: "* * * * *",
		Trigger:  "benchmark",
		Status:   JobStatusActive,
	}
}

func BenchmarkSchedulerTriggerEvaluation(b *testing.B) {
	for _, jobCount := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("jobs=%d", jobCount), func(b *testing.B) {
			sched, jobIDs := benchmarkSchedulerWithJobs(b, jobCount, Config{Enabled: true}, &benchmarkCoordinator{})

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				jobID := jobIDs[i%len(jobIDs)]
				if _, _, ok := sched.startJob(jobID, jobRunOptions{bypassCooldown: true}); !ok {
					b.Fatalf("startJob(%s) unexpectedly skipped", jobID)
				}
				sched.finishJob(jobID, nil)
			}
		})
	}
}

func BenchmarkSchedulerConcurrentJobExecution(b *testing.B) {
	for _, maxConcurrent := range []int{1, 4, 16} {
		b.Run(fmt.Sprintf("max_concurrent=%d", maxConcurrent), func(b *testing.B) {
			sched, jobIDs := benchmarkSchedulerWithJobs(b, 1, Config{
				Enabled:       true,
				MaxConcurrent: maxConcurrent,
			}, &benchmarkCoordinator{delay: 100 * time.Microsecond})

			jobID := jobIDs[0]
			var executed atomic.Int64
			var skipped atomic.Int64

			b.SetParallelism(8)
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if sched.runJob(jobID, jobRunOptions{bypassCooldown: true}) {
						executed.Add(1)
						continue
					}
					skipped.Add(1)
				}
			})
			b.StopTimer()

			elapsed := b.Elapsed().Seconds()
			if elapsed > 0 {
				b.ReportMetric(float64(executed.Load())/elapsed, "executed/s")
				b.ReportMetric(float64(skipped.Load())/elapsed, "skipped/s")
			}
		})
	}
}

func BenchmarkJobStorePersistence(b *testing.B) {
	ctx := context.Background()

	b.Run("save", func(b *testing.B) {
		store := NewFileJobStore(b.TempDir())

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := store.Save(ctx, benchmarkJob(fmt.Sprintf("save-job-%d", i))); err != nil {
				b.Fatalf("Save: %v", err)
			}
		}
	})

	b.Run("load", func(b *testing.B) {
		store := NewFileJobStore(b.TempDir())
		jobIDs := make([]string, 256)
		for i := range jobIDs {
			jobIDs[i] = fmt.Sprintf("load-job-%d", i)
			if err := store.Save(ctx, benchmarkJob(jobIDs[i])); err != nil {
				b.Fatalf("seed Save(%s): %v", jobIDs[i], err)
			}
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := store.Load(ctx, jobIDs[i%len(jobIDs)]); err != nil {
				b.Fatalf("Load: %v", err)
			}
		}
	})
}
