# Coding Agent Resilience Architecture

**Created**: 2026-02-10
**Status**: Implemented (Phase 1-4 complete, integration wired)

## Summary

Four-phase architecture making coding agent execution (Claude Code, Codex) survive process death, persist all state durably, and provide unified monitoring.

## Phase Status

### Phase 1: Unified Durable Task Store - COMPLETE
- `internal/domain/task/store.go` — Domain port + Task model (30 fields)
- `internal/infra/task/postgres_store.go` — Postgres implementation with transition audit trail
- `internal/infra/task/server_adapter.go` — `ports.TaskStore` adapter for web/CLI
- `internal/infra/task/lark_adapter.go` — `lark.TaskStore` adapter for Lark gateway
- DI wiring in `container_builder.go` with graceful degradation
- Server bootstrap uses adapter when Postgres available, falls back to in-memory
- Lark gateway uses adapter when Postgres available, falls back to legacy store
- **Commit**: `c6dbfc01`

### Phase 2: Subprocess Decoupling - COMPLETE
- `subprocess.go` — Detached mode: `Setsid`, file output, status file
- `output_reader.go` — JSONL file tailer with offset-based resume
- `orphan_detector.go` — Scans `.elephant/bridge/` for orphaned processes
- `executor.go` — Routes to attached/detached execution, `--output-file` flag
- Bridge scripts (`cc_bridge.py`, `codex_bridge.py`) support `--output-file` + `.done` sentinel
- **Commit**: `83905bca`

### Phase 3: Task Resumption - COMPLETE
- `resumer.go` — Five resume strategies: adopt, harvest, retry_with_context, retry_fresh, mark_failed
- `ClassifyOrphan()` determines action from process liveness + task metadata
- `buildResumePrompt()` enriches prompt with previous attempt context
- Checkpoint: saves LastOffset, LastIteration, TokensUsed, FilesTouched to task store
- **Commit**: `41a84dce`

### Phase 4: Unified Monitoring - COMPLETE
- `api_handler_tasks.go` — Full CRUD + `/active`, `/stats` endpoints
- `sse_handler_stream.go` — Task-scoped SSE at `GET /api/tasks/{id}/events`
- `event_broadcaster.go` — Session-scoped registration with history replay
- `task_command.go` — Lark `/tasks`, `/task status`, `/task cancel`, `/task history`
- **Commit**: `b363c276`

### Integration Wiring - COMPLETE
- `ResumePendingTasks` now calls bridge orphan resumer before re-dispatching tasks
- `BridgeOrphanResumer` interface in `task_execution_service.go`
- `bridgeResumerAdapter` in `server.go` wraps `bridge.Resumer`
- Wired via `WithCoordinatorBridgeResumer` option in server bootstrap
- 4 unit tests covering resumer invocation and edge cases
- **Commit**: `3f27e53e`

### Lint Cleanup - COMPLETE
- Fixed errcheck, gosimple, ineffassign findings in Phase 1-4 code
- **Commit**: `bc449bf1`

## Architecture

```
Server Restart
  ↓
ResumePendingTasks()
  ↓
Phase 1: resumeOrphanedBridges() → bridge.Resumer.ResumeOrphans()
  ├── DetectOrphanedBridges() scans .elephant/bridge/
  ├── ClassifyOrphan() → adopt/harvest/retry/fail
  └── processOrphan() → update unified task store
  ↓
Phase 2: ListByStatus(pending, running) → re-dispatch via executeTaskInBackground()
```

## Key Design Decisions

1. **Interface decoupling**: `BridgeOrphanResumer` interface in `app` package prevents import cycles. Bridge package can't import server app layer.
2. **Adapter pattern**: `bridgeResumerAdapter` in bootstrap converts `bridge.ResumeResult` → `app.OrphanResumeResult`.
3. **Graceful degradation**: Bridge resumer only wired when unified task store is available.
4. **Detached subprocess**: Uses `Setsid` (session leader), NOT `exec.CommandContext` (which sends SIGKILL on cancel).
5. **File-based output**: Bridge writes JSONL to disk instead of stdout pipe → survives parent death.
