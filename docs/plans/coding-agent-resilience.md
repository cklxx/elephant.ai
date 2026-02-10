# Coding Agent Resilience Architecture

**Status**: In Progress
**Created**: 2026-02-10
**Updated**: 2026-02-10 10:00

## Goal

Make coding agent execution survive process death, persist all state durably, and provide unified monitoring.

## Phase 1: Unified Durable Task Store (Foundation)

### Implementation Progress

- [x] Domain types + port (`internal/domain/task/`)
- [x] Postgres store (`internal/infra/task/postgres_store.go`)
- [ ] Server adapter (`internal/infra/task/server_adapter.go`)
- [ ] Lark adapter (`internal/infra/task/lark_adapter.go`)
- [ ] Wiring in DI + bootstrap
- [ ] Tests
- [ ] Migration

### Key Decisions

1. **Schema**: Single `tasks` table + `task_transitions` audit trail
2. **Adapters**: Wrap unified store behind existing `ports.TaskStore` and `lark.TaskStore` interfaces
3. **Migration**: EnsureSchema pattern for DDL; one-time data migration from old tables
4. **Status set**: pending/running/waiting_input/completed/failed/cancelled (superset of both stores)

## Phase 2: Subprocess Decoupling

- [ ] Detached bridge mode (Setsid, file output)
- [ ] Bridge script `--output-file` support
- [ ] OutputReader (file tailer)
- [ ] Modified executor
- [ ] Orphan detector

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
