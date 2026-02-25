# Go Best Practices Optimization

**Date:** 2026-01-31
**Status:** Completed

## Summary

Cross-cutting quality improvements applying Go best practices across `internal/server/` and `internal/agent/`.

## Batches Completed

### Batch 1: Fix Silent Error Suppression
- `async_event_history_store.go`: log flush error on shutdown instead of `_ =`
- `session_migration.go`: log marker write errors in both skip and completion paths

### Batch 2: Sentinel Errors for TaskStore
- Replaced 8 string-based "task not found" errors with `app.NotFoundError()` wrapping `app.ErrNotFound`
- Leveraged existing domain error infrastructure instead of creating new `ports.ErrTaskNotFound`
- `api_handler_tasks.go`: changed default fallback to 500 (ErrNotFound routes to 404 via `writeMappedError`)
- Added `errors.Is(err, ErrNotFound)` assertions in tests

### Batch 3: Error Wrapping — postgres_event_history_store.go
- Wrapped 14 bare `return err` returns with contextual `fmt.Errorf("verb noun: %w", err)` messages
- Covers: EnsureSchema, Append, AppendBatch, Stream, DeleteSession, HasSessionEvents, fetchBatch, pruneOnce, eventFromRecord

### Batch 4: Error Wrapping — Service & Handler Layer
- `session_service.go`: 5 bare returns wrapped (get/save session in persona, share token, validation)
- `evaluation_service.go`: 4 bare returns wrapped (config validation, output dir, dataset path)
- `runtime_models.go`: 4 bare returns wrapped (HTTP request build/fetch/read, JSON unmarshal)

### Batch 5: sync.Once Channel Close Protection
- Added `closeInputOnce sync.Once` to `BackgroundTaskManager`
- Protected both `close(m.externalInputCh)` calls in `forwardExternalInputRequests()`

## Design Decisions

- **Batch 2**: Used existing `app.NotFoundError()` / `app.ErrNotFound` pattern instead of creating a new `ports.ErrTaskNotFound` sentinel. The codebase already had a domain error layer with HTTP error mapper integration — adding a separate sentinel would have been redundant.
- **Batch 3**: Skipped wrapping the `fn(event)` callback error in `Stream()` — this is a caller-provided callback and wrapping it would add an artificial layer to the caller's own error.

## Commits

```
985d1ef7 fix(agent): protect external input channel close with sync.Once
901f837a fix(server): add contextual error wrapping in service and handler layer
138246b3 fix(server): add contextual error wrapping in postgres event history store
0a8114e1 fix(server): use domain sentinel errors for task not-found
19086bec fix(server): log suppressed errors instead of silently discarding
```

## Validation

- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./internal/server/... ./internal/agent/... -count=1` — all pass
- Race detector: `go test ./internal/agent/domain/react/... -count=1 -race` — pass
