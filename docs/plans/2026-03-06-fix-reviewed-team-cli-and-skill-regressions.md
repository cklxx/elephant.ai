# 2026-03-06 Fix Reviewed Team CLI And Skill Regressions

## Scope
- Fix reviewed regressions in `alex team` runtime wiring and targeting.
- Fix `reminder-scheduler` Python runner contract mismatch.
- Fix reminder plan mutation semantics when both `name` and `id` are provided.

## Plan
1. Repair `alex team` runtime context, validation, and runtime selection behavior.
2. Make team inject/terminal targeting deterministic and reject ambiguous selection.
3. Align `reminder-scheduler` with the shared Python CLI contract.
4. Add/adjust focused tests for the repaired behavior.
5. Run targeted lint/test verification and commit.

## Status
- Completed.

## Verification
- `go test ./cmd/alex/... ./internal/app/context/... ./internal/app/agent/context/... ./internal/app/agent/coordinator/... ./internal/app/agent/kernel/... ./internal/domain/agent/taskfile/...`
- `python3 -m pytest skills/reminder-scheduler/tests/test_reminder_scheduler.py`
