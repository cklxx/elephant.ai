# Server Alignment & Reliability Audit

_Last updated: 2024-xx-xx_

## Overview

This document captures the current gaps between the server implementation and the intended multi-surface experience (CLI, SSE server, and web UI). It drills into root causes, impact, and remediation plans so the team can prioritize and tackle the problems systematically.

## Critical Findings

### 1. Request-Scoped Context Cancels Background Work

- **Where**: `internal/server/http/api_handler.go:53-60`
- **What happens**: `HandleCreateTask` launches a goroutine with `r.Context()`. When the HTTP handler exits, that context cancels immediately. `ServerCoordinator.ExecuteTaskAsync` propagates the cancelled context down to `AgentCoordinator`, so the task halts almost instantly.
- **Impact**: No SSE stream is delivered; long-running tasks die silently. Web UI experiences “task accepted” followed by silence.
- **Fix plan**:
  1. Create a detached context: `ctx, cancel := context.WithCancelCause(context.Background())` before launching work.
  2. Persist the cancel handle alongside a generated task ID (for future cancel endpoint).
  3. Ensure downstream code receives the detached context.
  4. Emit an error event and fail the task if execution still aborts unexpectedly.
- **Tests**: Add integration test that posts a task, waits for SSE events, and verifies completion while request returns immediately.

### 2. Session ID Never Propagates Back to Clients

- **Where**: `HandleCreateTask` returns `req.SessionID`, but `AgentCoordinator` creates a new session when the field is empty.
- **Impact**: Web UI cannot subscribe to SSE or resume the session because it never learns the real ID. CLI also loses continuity when relying on API.
- **Fix plan**:
  1. Enhance `ServerCoordinator.ExecuteTaskAsync` to report the effective session ID (existing or newly created).
  2. Modify response schema to include both `task_id` and `session_id`.
  3. When the client omits `session_id`, persist the new ID in session history and return it.
- **Tests**: Extend handler tests to assert the response echoes the stored session and that SSE stream uses the same ID.

### 3. SSE Broadcasts Leak Across Sessions

- **Where**: `internal/server/app/event_broadcaster.go` – `extractSessionID` always returns empty string, so events broadcast to every client.
- **Impact**: Multiple users connected concurrently receive each other’s agent output; privacy and UX regression.
- **Fix plan**:
  1. Enrich `domain.AgentEvent` with a `SessionID` or metadata map.
  2. When a task is prepared, inject the session ID into the context and ensure every emitted event carries it.
  3. Update broadcaster to filter by `SessionID`. Fall back to broadcast-all only when ID is absent.
  4. Add tests with two registered channels to confirm isolation.

### 4. API Surface Diverges From Web Contract

- **Where**: Types defined in `web/lib/types.ts` expect task IDs, status routes, session detail/fork APIs. Router currently exposes only bare-bones endpoints.
- **Impact**: Web UI cannot function against the shipped Go server without mock implementations. Documentation promises features that do not exist on the backend.
- **Fix plan**:
  1. Introduce task lifecycle storage (task ID, status, timestamps, errors).
  2. Implement endpoints: `GET /api/tasks/{id}`, `POST /api/tasks/{id}/cancel`, `POST /api/sessions/{id}/fork`, etc.
  3. Return payloads matching TypeScript contracts.
  4. Document any deviations and update `docs/` and README for parity.
- **Tests**: Add REST contract tests plus Playwright/Next.js integration smoke test hitting the real server.

### 5. Session Storage Collisions

- **Where**: `internal/session/filestore/store.go:27-41` uses `session-%d` with second-level resolution.
- **Impact**: Rapid session creation overwrites previous sessions, causing history loss and inconsistent behavior.
- **Fix plan**:
  1. Switch to UUIDv4 (`github.com/google/uuid`) or `UnixNano` + random suffix.
  2. Write using `os.O_CREATE|os.O_EXCL` to prevent accidental overwrites.
  3. Provide migration script to rename existing files.
- **Tests**: Create regression test that spawns multiple sessions concurrently and asserts unique files.

### 6. CORS Misconfiguration

- **Where**: `internal/server/http/middleware.go:32` uses `(allowed || true)` while allowing credentials.
- **Impact**: Any origin can make authenticated browser requests, which is risky in production.
- **Fix plan**:
  1. Replace placeholder logic with environment-driven allowlist.
  2. Only set `Access-Control-Allow-Credentials` when the origin is explicitly trusted.
  3. Document configuration knobs in `DEPLOYMENT.md`.
- **Tests**: Unit test covering allowed vs blocked origins; manual verification in staging.

### 7. Unused Message Queue Dependency

- **Where**: `internal/agent/app/coordinator.go` stores `messageQueue` but never uses it.
- **Impact**: Increases mental overhead and hints at incomplete design. Could hide future bugs if developers assume queueing exists.
- **Fix options**:
  - Remove the dependency until real queuing is implemented.
  - Or implement a queue-backed ingestion path; e.g., background worker pulling from `MessageQueue`.
- **Recommendation**: Remove for now and track a follow-up issue if asynchronous ingestion is desired.

## Implementation Roadmap

| Phase | Theme | Key Deliverables | Notes |
|-------|-------|------------------|-------|
| Phase 1 | Reliability Hotfixes | Detached context, session ID propagation, session ID uniqueness | Unblocks SSE + CLI parity quickly |
| Phase 2 | Observability & Scoping | Event session tagging, filtered broadcasts, error event propagation | Enables safe multi-user operation |
| Phase 3 | Contract Alignment | Full task/session API, updated docs, end-to-end tests | Required for shipping web UI |
| Phase 4 | Hardening | CORS configuration, deployment guidance, dead-code cleanup | Prep for production |

## Follow-Up Actions

1. Create GitHub issues (or Linear tickets) mapping one-to-one with the critical findings.
2. Prioritize Phase 1 fixes for the next sprint; they are blockers for any meaningful server usage.
3. Schedule an integration testing effort to cover REST + SSE + web UI workflows after Phase 3.
4. Review telemetry/logging needs once the event metadata work lands—may want to push session IDs into structured logs.

## Appendix: Suggested Test Additions

- **Go**: New tests under `internal/server/http` for task creation, SSE isolation, and CORS handling.
- **Integration**: Docker-compose-based smoke test that runs `cmd/alex-server` alongside the Next.js frontend; asserts that a task submission yields streamed events.
- **Frontend**: Playwright test to submit a task and verify the AgentOutput renders tool events tied to the correct session.

---

Maintainers: please add notes or amendments as fixes land so this document stays current.
