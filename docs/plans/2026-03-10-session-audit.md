## Goal

Audit the effective session-management code paths, remove dead helpers, simplify lifecycle transitions, and reduce complexity in session persistence.

## Scope

- `internal/app/agent/coordinator/session_manager.go`
- `internal/app/agent/preparation/session.go`
- `internal/domain/agent/ports/storage/session.go`
- `internal/infra/session/filestore/store.go`

## Findings

- `internal/app/session/` does not exist; the real session lifecycle is split across coordinator, preparation, storage, and session filestore.
- Empty session creation was duplicated in multiple packages.
- Resetting a session's persisted state was implemented inline instead of through an explicit lifecycle helper.
- `SaveSessionAfterExecution` mixed history append, content sanitization, metadata state transitions, and persistence in one long function.

## Plan

1. Introduce explicit session lifecycle helpers for create/reset at the storage boundary.
2. Reuse the create helper across coordinator, preparation, and file-backed session store.
3. Split `SaveSessionAfterExecution` into focused helpers for history, content, and metadata updates.
4. Run focused tests/lint/review, then commit and fast-forward merge to `main` without pushing.
