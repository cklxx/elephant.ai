# Coding Agent Resilience Architecture

**Status**: Complete
**Created**: 2026-02-10
**Updated**: 2026-02-10 16:00

## Goal

Make coding agent execution survive process death, persist all state durably, and provide unified monitoring.

## Phase 1: Unified Durable Task Store (Foundation) — DONE

### Implementation Progress

- [x] Domain types + port (`internal/domain/task/`)
- [x] Postgres store (`internal/infra/task/postgres_store.go`)
- [x] Server adapter (`internal/infra/task/server_adapter.go`)
- [x] Lark adapter (`internal/infra/task/lark_adapter.go`)
- [x] Wiring in DI + bootstrap
- [x] Tests
- [x] Committed

### Key Decisions

1. **Schema**: Single `tasks` table + `task_transitions` audit trail
2. **Adapters**: Wrap unified store behind existing `ports.TaskStore` and `lark.TaskStore` interfaces
3. **Migration**: EnsureSchema pattern for DDL; one-time data migration from old tables
4. **Status set**: pending/running/waiting_input/completed/failed/cancelled (superset of both stores)

## Phase 2: Subprocess Decoupling — DONE

### Implementation Progress

- [x] Detached subprocess mode in `subprocess.go` (Setsid, file output, PID status file)
- [x] Bridge script `--output-file` support (`cc_bridge.py`, `codex_bridge.py`)
- [x] `.done` sentinel + SIGTERM handler in both bridge scripts
- [x] OutputReader — JSONL file tailer with poll-based tailing (`output_reader.go`)
- [x] Orphan detector — detect & inspect orphaned bridge dirs (`orphan_detector.go`)
- [x] Modified executor — `executeDetached()` path with file-based output
- [x] `OnBridgeStarted` callback on `ExternalAgentRequest` for bridge meta persistence
- [x] Tests for all new components

### Key Decisions

1. **Detached mode**: `Setsid=true` (session leader) instead of `Setpgid=true` (process group) — subprocess survives parent death
2. **No exec.CommandContext**: Detached mode uses plain `exec.Command` to prevent context-based SIGKILL
3. **Output file path**: `{workDir}/.elephant/bridge/{taskID}/output.jsonl`
4. **Sentinel**: `.done` file signals bridge completion; bridge scripts write it on all exit paths
5. **SIGTERM handler**: Both bridge scripts emit final error event + write sentinel before exit
6. **OutputReader**: Poll-based tailing (200ms base, backs off to 2s) — simpler than fsnotify, works cross-platform
7. **Bridge args**: `--output-file <path>` via argparse; when omitted, writes to stdout (backward compatible)

### Files Changed/Created

| File | Change |
|------|--------|
| `internal/infra/external/subprocess/subprocess.go` | Added `Detached`, `OutputFile`, `StatusFile` config; `startDetached()` method; `Done()` accessor |
| `internal/infra/external/bridge/executor.go` | Split `Execute` into `executeAttached`/`executeDetached`; extracted `applyEvent`; `BridgeStartedInfo` type |
| `internal/infra/external/bridge/output_reader.go` | NEW: JSONL file tailer with offset resume |
| `internal/infra/external/bridge/orphan_detector.go` | NEW: Scan bridge dirs, check PID liveness, read status files |
| `internal/domain/agent/ports/agent/external_agent.go` | Added `OnBridgeStarted` callback to `ExternalAgentRequest` |
| `scripts/cc_bridge/cc_bridge.py` | Added `--output-file`, `.done` sentinel, `SIGTERM` handler |
| `scripts/codex_bridge/codex_bridge.py` | Same as above |

## Phase 3: Task Resumption — DONE

### Implementation Progress

- [x] Bridge `Resumer` — orphan classification, adoption, harvesting, retry logic
- [x] Resume strategy matrix: adopt (running), harvest (done), retry with context, retry fresh, mark failed
- [x] `buildResumePrompt` — enriches original prompt with [Resume Context] for partially-completed tasks
- [x] `BridgeMetaPersister` interface on agent ports — type-safe bridge meta persistence
- [x] `BridgeInfoProvider` interface on task infra — extracts PID/OutputFile from opaque info
- [x] `LarkAdapter.SetBridgeMeta` — persists bridge meta through the Lark task store adapter
- [x] `Gateway.PersistBridgeMeta` — wires through to task store in Lark gateway
- [x] `BackgroundTaskManager.Dispatch` — `OnBridgeStarted` callback wired for external executors
- [x] Tests for all new components

### Key Decisions

1. **Resume classification**: 5 actions based on (process alive, .done exists, files touched, prompt exists)
2. **BridgeInfoProvider interface**: Avoids circular imports between infra/task and infra/external/bridge; JSON fallback for untyped info
3. **BridgeMetaPersister opt-in**: Only persists bridge meta if the CompletionNotifier also implements BridgeMetaPersister — backward compatible
4. **Resume prompt**: Wraps original prompt with iteration count, files touched, and "[Resume Context]" prefix
5. **Orphan cleanup**: Bridge directories cleaned up after harvest/retry/mark-failed — running orphans kept alive for adoption

### Files Changed/Created

| File | Change |
|------|--------|
| `internal/infra/external/bridge/resumer.go` | NEW: Orphan classification, adoption, harvesting, retry, checkpoint update |
| `internal/infra/external/bridge/resumer_test.go` | NEW: Tests for classify, buildResumePrompt, harvest, markFailed, skipTerminal |
| `internal/domain/agent/ports/agent/background.go` | Added `BridgeMetaPersister` interface |
| `internal/domain/agent/react/background.go` | Wired `OnBridgeStarted` callback in external executor dispatch |
| `internal/infra/task/lark_adapter.go` | Added `SetBridgeMeta`, `BridgeInfoProvider`, `extractBridgeMeta` |
| `internal/infra/external/bridge/executor.go` | Added `BridgePID()`, `BridgeOutputFile()` on `BridgeStartedInfo` |
| `internal/delivery/channels/lark/gateway.go` | Added `PersistBridgeMeta` method |

## Phase 4: Unified Monitoring — DONE

### Implementation Progress

- [x] Enhanced Task API (`GET /api/tasks/active`, `GET /api/tasks/stats`)
- [x] Task-scoped SSE (`GET /api/tasks/{task_id}/events`)
- [x] Lark monitoring commands (already existed: `/tasks`, `/task status|cancel|history`)
- [x] Proactive completion notifications (already existed: `backgroundProgressListener`, `NotifyCompletion`)
- [x] Tests for new endpoints
- [x] All tests passing

### Key Decisions

1. **Task-scoped SSE**: Subscribes to session events and filters by `run_id` — avoids changing broadcaster core architecture
2. **Auto-close**: Task SSE stream auto-closes when `EventResultFinal` or `EventResultCancelled` is received
3. **Route ordering**: `/api/tasks/active` and `/api/tasks/stats` registered before `{task_id}` wildcard to avoid capture
4. **Stats aggregation**: `GetTaskStats` iterates all tasks to compute counts — acceptable for current scale

### Files Changed/Created

| File | Change |
|------|--------|
| `internal/delivery/server/app/task_execution_service.go` | Added `ListActiveTasks`, `GetTaskStats`, `TaskStats` struct |
| `internal/delivery/server/app/server_coordinator.go` | Added `ListActiveTasks`, `GetTaskStats` delegate methods |
| `internal/delivery/server/http/api_handler_tasks.go` | Added `HandleListActiveTasks`, `HandleGetTaskStats` handlers |
| `internal/delivery/server/http/sse_handler_stream.go` | Added `HandleTaskSSEStream` for task-scoped SSE |
| `internal/delivery/server/http/router.go` | Registered `/api/tasks/active`, `/api/tasks/stats`, `/api/tasks/{task_id}/events` routes |
| `internal/delivery/server/http/api_handler_test.go` | Added `TestHandleListActiveTasks`, `TestHandleGetTaskStats` |
