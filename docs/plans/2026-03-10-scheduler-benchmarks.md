## Scheduler Benchmarks

### Goal
Add `internal/app/scheduler/scheduler_benchmark_test.go` with focused scheduler benchmarks for trigger evaluation, concurrent job execution under `MaxConcurrent`, and file-backed job-store persistence.

### Scope
- Benchmark trigger/job evaluation cost across `10`, `100`, and `1000` registered jobs.
- Benchmark contended `runJob` throughput with different `MaxConcurrent` limits.
- Benchmark file job-store `Save` and `Load` latency.

### Approach
1. Reuse the real scheduler internals already exercised in unit tests:
   - `registerTriggerLocked`
   - `startJob` / `finishJob`
   - `runJob`
   - `FileJobStore.Save` / `Load`
2. Keep the benchmark coordinator minimal so the benchmark stays focused on scheduler overhead and concurrency gating.
3. Use sub-benchmarks for the requested cardinalities and concurrency limits.

### Validation
- `go test ./internal/app/scheduler`
- `go test -run '^$' -bench Scheduler -benchmem ./internal/app/scheduler`
- `go test -run '^$' -bench JobStorePersistence -benchmem ./internal/app/scheduler`
- `python3 skills/code-review/run.py review`
