## Goal

Simplify Wave 2 error handling in a narrow, safe slice by removing redundant wrapping and redundant local `err` plumbing where callers already receive sufficiently typed/descriptive errors.

## Scope

- `internal/runtime/runtime.go`
  - Stop re-wrapping session state transition errors with the generic `runtime:` prefix.
  - Stop re-wrapping constructor errors already labeled by `panel`/`store`.
- `evaluation/task_mgmt/task_manager.go`
  - Remove duplicated "task not found" wrapping in `RecordRun`.
  - Collapse the temporary `err` binding used only for immediate passthrough.
- Add focused tests for the simplified error paths.

## Safety Constraints

- No cross-layer API changes.
- No sentinel or error type changes.
- Keep operation-specific wrapping where it still adds distinct context.

## Verification

- `go test ./internal/runtime ./evaluation/task_mgmt`
- `python3 skills/code-review/run.py review`
