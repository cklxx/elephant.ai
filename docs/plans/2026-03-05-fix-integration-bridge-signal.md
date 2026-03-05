# 2026-03-05 Fix Integration Bridge Signal Failures

## Goal
- Root-cause and fix `go run ./cmd/alex dev test` failures in `internal/infra/integration` where tasks fail with `bridge terminated by signal`.

## Plan
1. Reproduce with a minimal failing integration test and capture task runtime metadata.
2. Trace binary/agent resolution path (`run_tasks` bootstrap -> taskfile runtime meta -> bridge executor).
3. Implement fix so fake bridge CLIs used by integration tests are not unintentionally overridden.
4. Run targeted integration tests, then full `go run ./cmd/alex dev test`.
5. Run mandatory code review, commit, merge, push.

## Progress
- [x] Plan created.
- [x] Reproduction + root cause confirmed.
- [x] Code fix implemented.
- [x] Validation passed (`internal/infra/integration` full package + focused race checks).
- [ ] Merged and pushed.
