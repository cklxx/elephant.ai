# Server Architecture Phase 2 — Domain Error Types + Test Coverage + Health Enrichment

**Created:** 2026-01-31
**Status:** In Progress
**Prerequisite:** Phase 1 complete (all 10 batches)

---

## Research Synthesis

Based on Go best practices research (hexagonal architecture, AI agent orchestration patterns, graceful shutdown, CQRS/SSE) and a deep codebase audit, the following areas offer the highest impact-to-risk ratio for the next round of improvements:

### Architecture Grade: B+ → Target: A-

| Area | Current | Target |
|------|---------|--------|
| Domain errors | Ad-hoc `fmt.Errorf` + manual status codes | Typed sentinel errors + central HTTP mapper |
| SessionService tests | Indirect via coordinator | Dedicated unit test suite |
| /health degraded | Tracked at bootstrap, not exposed | Exposed via DegradedProbe |
| Lint cleanliness | 1 unused helper warning | Zero warnings |

---

## Execution Plan (4 Batches)

### Batch 1: Domain Error Types + HTTP Error Mapper

**Risk:** Low | **Impact:** High — eliminates 59+ ad-hoc `fmt.Errorf` calls with typed errors

**New file: `internal/server/app/errors.go`**

Define domain-level sentinel errors:
- `ErrNotFound` — resource not found (→ 404)
- `ErrValidation` — invalid input (→ 400)
- `ErrConflict` — duplicate or state conflict (→ 409)
- `ErrUnavailable` — dependency not ready (→ 503)

Wrap with `Is()`-compatible error types for context enrichment.

**New file: `internal/server/http/error_mapper.go`**

Central function `mapDomainError(err error) (int, string)` that maps domain errors → HTTP status + message. Used by all handlers to replace scattered `writeJSONError` calls with manual status selection.

**Modified files:**
- `session_service.go` — Return `ErrNotFound` from Get/Delete when store returns not-found
- `task_execution_service.go` — Return `ErrNotFound` from GetTask, `ErrValidation` from bad input
- `snapshot_service.go` — Return `ErrNotFound` for missing snapshots
- HTTP handler files — Use `mapDomainError()` for service-returned errors

### Batch 2: SessionService Unit Tests

**Risk:** None | **Impact:** High — covers critical untested path

**New file: `internal/server/app/session_service_test.go`**

Test cases:
1. `TestGetSession` — found / not found
2. `TestCreateSession` — success / coordinator nil / state store init failure (non-fatal)
3. `TestDeleteSession` — success / partial failure (errors.Join) / cascading cleanup
4. `TestUpdateSessionPersona` — success / nil persona / session not found
5. `TestForkSession` — success / original not found / save failure / metadata copy
6. `TestListSessions` — pagination
7. `TestEnsureSessionShareToken` — new token / existing token / reset / empty ID
8. `TestValidateShareToken` — valid / invalid / empty / no metadata

### Batch 3: Degraded Components in /health Endpoint

**Risk:** Low | **Impact:** Medium — ops visibility into bootstrap failures

**Modified files:**
- `internal/server/app/health.go` — Add `DegradedProbe` that wraps `*DegradedComponents`
- `internal/server/bootstrap/server.go` — Register `DegradedProbe` with health checker
- `internal/server/http/api_handler_misc.go` — Health response already handles "degraded" status

The existing health handler already computes `overallStatus = "degraded"` when components are `not_ready`. DegradedProbe just exposes bootstrap failures (attachments, event-history, analytics, gateways) as additional `ComponentHealth` entries with `status: "not_ready"` and the recorded error message.

### Batch 4: Lint Fix + Validation

**Risk:** None | **Impact:** Clean CI

- Remove unused `isStarted` in `internal/server/bootstrap/subsystem_test.go`
- Full `go vet`, `go build`, `go test` validation

---

## Verification Checklist (per batch)

- [ ] `go build ./internal/server/...`
- [ ] `go vet ./internal/server/...`
- [ ] `go test ./internal/server/app/... -count=1`
- [ ] `go test ./internal/server/http/... -count=1`
- [ ] `go test ./internal/server/bootstrap/... -count=1`
