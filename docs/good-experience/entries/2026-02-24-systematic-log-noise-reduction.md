# 2026-02-24 — Systematic log noise reduction for hot paths

Impact: Reduced runtime log noise in ReAct/task execution hot paths while preserving key failure diagnostics and lifecycle visibility.

## What changed

- Removed redundant per-event debug logs in `internal/domain/agent/react/events.go`.
- Downgraded high-frequency runtime transitions from `INFO` to `DEBUG` in `internal/domain/agent/react/runtime.go`.
- Simplified and deduplicated task execution logs in `internal/delivery/server/app/task_execution_service.go`:
  - removed repeated component prefixes in message bodies,
  - removed duplicate cancellation/event-emission logs,
  - kept warning/error paths and completion summary logs.
- Reduced broadcaster noise in `internal/delivery/server/app/event_broadcaster.go`:
  - trimmed oversized “available sessions” payload from no-client warnings,
  - downgraded register/unregister/clear-history messages to `DEBUG`.
- Reduced coordinator/session persistence chatter:
  - task-start log now avoids full task body, logs only session + `task_chars`,
  - removed redundant save-success debug lines.
- Added governance and tooling:
  - `docs/reference/LOGGING_STANDARD.md` new noise-control policy and review checklist.
  - `scripts/analysis/log_audit.sh` for recurring hotspot and prefix scans.

## Why this worked

- Focused on high-frequency hotspots first (`runtime`, `task execution`, `broadcaster`) where each log line has multiplicative cost.
- Applied clear level boundaries (`INFO` for milestones, `DEBUG` for loop internals).
- Removed duplicated context in log messages when component logger already provides scope.
- Preserved warning/error signals so incident triage remained intact.

## Validation

- `./scripts/pre-push.sh` passed (full chain including `go test -race`, lint, architecture checks, web lint/build).
- `scripts/go-with-toolchain.sh test ./internal/domain/agent/react ./internal/delivery/server/app ./internal/app/agent/coordinator` passed.
- `scripts/analysis/log_audit.sh 20` produced updated hotspot and prefix visibility report for follow-up governance.
