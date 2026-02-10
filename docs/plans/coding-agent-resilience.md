# Coding Agent Resilience Architecture

**Status**: In Progress
**Created**: 2026-02-10
**Updated**: 2026-02-10 14:00

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

## Phase 3: Task Resumption

- [ ] Bridge checkpoint via unified store
- [ ] Resume strategy matrix
- [ ] Intelligent retry with context
- [ ] Enhanced ResumePendingTasks
- [ ] BackgroundTaskManager persistence

## Phase 4: Unified Monitoring

- [ ] Enhanced Task API
- [ ] Task-scoped SSE
- [ ] Lark monitoring commands
- [ ] Proactive completion notifications
