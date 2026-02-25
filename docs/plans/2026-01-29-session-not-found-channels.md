# Plan: Fix session-not-found for channel gateways + task API (2026-01-29)

- Reviewed `docs/guides/engineering-practices.md`.
- Memory load completed (latest error/good entries + summaries + long-term memory).

## Goals
- Prevent first-message failures in channel gateways when sessions are missing.
- Standardize session-not-found errors for reliable handling.
- Return 404 for stale sessions in task creation to align with frontend retry logic.

## Plan
1. Add a `storage.ErrSessionNotFound` sentinel and update session stores to return it.
2. Add `AgentCoordinator.EnsureSession` to create sessions with explicit IDs when missing.
3. Ensure channel gateways call `EnsureSession` before `ExecuteTask`.
4. Map session-not-found errors to HTTP 404 in `HandleCreateTask`.
5. Add/adjust tests (coordinator ensure, filestore not-found, API handler 404, gateway stubs).
6. Run `gofmt` if needed, then full lint + tests.
7. Update docs (plan progress, memory timestamp) and commit changes.

## Progress
- 2026-01-29: Plan created; engineering practices reviewed; starting implementation.
- 2026-01-29: Added ErrSessionNotFound sentinel + store updates; implemented EnsureSession + gateway calls; added tests for coordinator/session stores/API 404; updated long-term memory timestamp.
- 2026-01-29: Ran `./dev.sh lint` and `./dev.sh test` (pass; linker warnings about LC_DYSYMTAB observed during Go tests).
