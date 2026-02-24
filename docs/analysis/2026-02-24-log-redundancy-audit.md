# 2026-02-24 Log Redundancy Audit

## Scope
- `internal/domain/agent/react/runtime.go`
- `internal/domain/agent/react/events.go`
- `internal/delivery/server/app/task_execution_service.go`
- `internal/delivery/server/app/event_broadcaster.go`
- `internal/app/agent/coordinator/coordinator.go`
- `internal/app/agent/coordinator/session_manager.go`

## Method
1. Scan all Go log calls with `rg` and locate high-frequency hotspots.
2. Classify each hotspot log as one of:
   - `delete`: duplicated semantics or no diagnostic value.
   - `downgrade`: useful but high-frequency, move `INFO -> DEBUG`.
   - `keep`: operationally critical (`WARN`/`ERROR`/one-shot milestones).
3. Apply only low-risk changes that do not alter control flow.

## Decisions

### React runtime and event emission
- `delete`: per-event emit logs in `events.go` (`emitting/success/no-listener`) because they fire for every event and duplicate event stream itself.
- `downgrade`: iteration and final-answer transition logs from `INFO` to `DEBUG`.
- `keep`: context overflow, tool failures, max-iteration safeguards.

### TaskExecutionService
- `delete`: repeated emission logs for cancellation/input events.
- `downgrade`: task startup/preset details and resume-per-task success logs from `INFO` to `DEBUG`.
- `normalize`: remove repeated text prefixes (`[TaskExecutionService]`, `[Background]`, `[Resume]`, `[CancelTask]`) because logger component already carries context.
- `keep`: claim/admission/lease failures, execution failures, completion summary.

### EventBroadcaster
- `downgrade`: client register/unregister and clear-history logs to `DEBUG`.
- `trim`: remove `Available sessions: %v` from no-client warnings to avoid large noisy payloads.
- `keep`: drop/critical-delivery warnings and sampled no-client/no-session diagnostics.

### Coordinator/session manager
- `trim`: task start log no longer prints full task text; keep metadata (`session`, `task_chars`).
- `delete`: "delegating to engine", "listener provided/successfully set", and session save success debug chatter.
- `keep`: execution failure, session stats summary, persistence failures.

## Expected outcome
- Lower `INFO` volume in hot-path execution loops.
- Reduced duplicated component prefixes and repeated transition logs.
- Preserved failure diagnostics and key lifecycle milestones.
