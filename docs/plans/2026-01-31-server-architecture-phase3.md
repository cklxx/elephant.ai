# Server Architecture Phase 3 — Go 1.22+ Router + Critical Test Coverage

**Created:** 2026-01-31
**Status:** Complete
**Prerequisite:** Phase 2 complete

---

## Summary

Refactor router to use Go 1.22+ stdlib ServeMux method-specific patterns (`"GET /path/{param}"`), eliminating 23+ manual `strings.TrimPrefix` path parsing sites. Then fill the two critical test gaps (SnapshotService, AsyncEventHistoryStore).

---

## Batch 1: Router Rewrite (router.go)

**Risk:** Medium | **Impact:** High — architectural improvement

Rewrite route registrations from manual dispatch to Go 1.22 patterns:
- `"GET /api/tasks/{task_id}"` instead of `/api/tasks/` + manual parsing
- `"POST /api/sessions"` instead of switch on `r.Method`
- Each handler gets direct registration; no anonymous dispatch wrappers

Key changes:
- Sessions: 11 routes (GET/DELETE session, GET/PUT persona, GET snapshots, GET turn, POST replay/share/fork, GET/POST sessions list/create)
- Tasks: 4 routes (POST create, GET list, GET task, POST cancel)
- Evaluations: 4 routes (GET list, POST create, GET eval, DELETE eval)
- Agents: 3 routes (GET list, GET agent, GET agent evaluations)

## Batch 2: Handler Adaptation

**Risk:** Low | **Impact:** High — removes 23+ manual parsing sites

Replace `strings.TrimPrefix(r.URL.Path, "/api/...")` with `r.PathValue("param")` in:
- `api_handler_sessions.go` — 9 sites
- `api_handler_tasks.go` — 2 sites
- `api_handler_evaluations.go` — 4 sites
- `api_handler_context.go` — 2 sites
- `share_handler.go` — 1 site
- Remove redundant `requireMethod()` calls from handlers that now have method-specific routes

## Batch 3: SnapshotService Tests

**New file:** `internal/server/app/snapshot_service_test.go`

## Batch 4: AsyncEventHistoryStore Tests ✅

**New file:** `internal/server/app/async_event_history_store_test.go`

26 tests covering all public methods and internal paths:
- Append: normal enqueue, nil event, nil store, queue full (ErrAsyncHistoryQueueFull), context cancellation
- Stream: flush-before-read consistency, nil store, inner error propagation
- DeleteSession: flush-before-delete, nil store, inner error propagation
- HasSessionEvents: flush-before-check, nil store, inner error propagation
- Close: flush remaining events, idempotent (sync.Once), nil store noop
- Batch path: AppendBatch used when inner implements batchEventAppender
- Single append fallback: individual Append calls when no batch support
- Batch size trigger: automatic flush when buffer reaches batchSize
- Options: applied correctly, invalid values (0, negative) preserve defaults, nil option skipped
- Ordering: events streamed in insertion order
- Flush context cancellation: propagates context.Canceled
- Error propagation: inner append error and batch error surface through flush
