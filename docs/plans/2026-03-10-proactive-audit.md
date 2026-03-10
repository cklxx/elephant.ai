## Goal

Audit proactive application code under the effective runtime locations for proactive behavior, remove dead paths, trim stale trigger configuration, and simplify scheduler trigger dispatch.

## Scope

- `internal/app/agent/hooks`
- `internal/app/scheduler`
- `internal/shared/config` entries that still expose proactive scheduler trigger fields

## Findings

- `internal/app/proactive/` does not exist; proactive runtime behavior is implemented by hooks and scheduler packages.
- Scheduler trigger config still exposed `approval_required` and `risk`, but no runtime code consumed either field.
- Trigger registration and execution repeated basic validation, context setup, and registration flow.
- Hook injection type `skill_activation` was defined but not emitted by any production code or tests.

## Plan

1. Remove stale scheduler trigger config fields from runtime types, file config, merge logic, tests, and schema.
2. Simplify scheduler trigger registration and execution with shared helpers while preserving behavior.
3. Delete dead proactive hook constant and keep tests aligned with current runtime behavior.
4. Run focused tests/lint/review, commit in worktree, and fast-forward merge to `main` without pushing.
