# Plan: Root Fix for sqlite-vec Darwin Deprecation Warnings

Owner: cklxx  
Date: 2026-02-11

## Goal
Eliminate macOS deprecation warnings from `sqlite3_auto_extension` / `sqlite3_cancel_auto_extension` by removing the deprecated API usage path, while keeping sqlite-vec search behavior unchanged.

## Scope
- Replace global sqlite-vec auto-extension registration with per-connection initialization.
- Keep existing memory index schema and query behavior unchanged.
- Keep CGO-required build split (`index_store.go` vs `index_store_stub.go`) unchanged.

## Non-Goals
- Replace SQLite backend or vector engine.
- Change index ranking logic or memory retrieval behavior.
- Introduce warning-suppression-only fixes as the primary solution.

## Implementation Plan
1. Vendor/fork sqlite-vec Go bindings locally and remove deprecated API references from the build path.
2. Add an explicit sqlite-vec initializer callable from our sqlite connection hook.
3. Register a dedicated sqlite driver with `ConnectHook` to initialize sqlite-vec on every new connection.
4. Verify no deprecation warning is emitted in `go test`/`go build` paths and validate memory index tests.
5. Run lint + full tests, then perform mandatory code review workflow before commit/merge.

## Best-Practice References
- SQLite extension initialization guidance: prefer connection-scoped initialization over process-global side effects.
- Go database/sql driver practice: use explicit `sql.Register` + `ConnectHook` for deterministic connection setup.
- Maintainability principle: isolate platform/toolchain quirks from domain logic.

## Progress
- [x] Locate warning source and current loading path.
- [x] Implement non-deprecated sqlite-vec initialization path.
- [x] Validate warning-free compile + memory tests.
- [x] Complete lint + full tests.
- [x] Complete mandatory code review and remediation.
