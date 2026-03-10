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

- [x] Introduce explicit session lifecycle helpers for create/reset at the storage boundary.
- [x] Reuse the create helper across coordinator, preparation, and file-backed session store.
- [x] Split `SaveSessionAfterExecution` into focused helpers for history, content, and metadata updates. *(already done in prior work)*
- [x] Run focused tests/lint/review, then commit and fast-forward merge to `main` without pushing.

## Changes

- `storage/session.go`: Added `ClearContent()` method and `GetOrCreate()` function.
- `coordinator/session_manager.go`: Replaced `clearPersistedSessionContent` with `session.ClearContent()`, simplified `EnsureSession` to delegate to `GetOrCreate`.
- `preparation/session.go`: Simplified `loadSession` to delegate to `GetOrCreate`, removed `errors`/`time` imports.
- `storage/session_test.go`: Added 7 tests covering `ClearContent` and `GetOrCreate`.
