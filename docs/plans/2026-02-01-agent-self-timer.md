# Plan: Agent Self-Timer with Session Context Resume

**Status:** Implemented
**Date:** 2026-02-01

## Overview

Added tools that allow the agent to dynamically create/list/cancel timers at runtime. When a timer fires, it resumes the originating session context (full conversation history) and executes a complete agent task (ReAct loop + tools). Timers persist to disk and survive server restarts.

## Architecture Decision

**New `internal/timer/` package** (separate from `internal/scheduler/`).

- Scheduler = admin-configured, server-level, YAML-based proactive triggers
- Timer Manager = agent-initiated, runtime-created, dynamic timers with session context
- Both share execution pipeline (`AgentCoordinator` + `Notifier`) but own their own state

## Implementation

### Commit 1: Core types + persistence + config
- `internal/timer/timer.go` — Timer type, constants, validation, ID generation
- `internal/timer/store.go` — File-based YAML store (~/.alex/timers/)
- `internal/timer/store_test.go` — TDD store and validation tests
- `internal/config/types.go` — TimerConfig in ProactiveConfig + defaults

### Commit 2: Timer manager
- `internal/timer/manager.go` — Full lifecycle management
- `internal/timer/manager_test.go` — Tests for: one-shot fires, recurring, cancel, max limit, restart recovery, stop cleanup, session ID passthrough, notifications

### Commit 3: Tools + context injection
- `internal/tools/builtin/shared/context.go` — timerManagerKey, WithTimerManager, TimerManagerFromContext
- `internal/tools/builtin/timer/set_timer.go` — Creates one-shot or recurring timers
- `internal/tools/builtin/timer/list_timers.go` — Lists timers by status/user
- `internal/tools/builtin/timer/cancel_timer.go` — Cancels active timers
- `internal/tools/builtin/timer/tools_test.go` — Tool-level tests
- `internal/toolregistry/registry.go` — Static registration of 3 tools

### Commit 4: Bootstrap + integration
- `internal/server/bootstrap/timer.go` — startTimerManager(), store path resolution
- `internal/server/bootstrap/server.go` — timer-manager subsystem stage
- `internal/agent/app/coordinator/coordinator.go` — SetTimerManager + context injection in ExecuteTask

## Key Design: Session Context Resume

When `set_timer` is called, the timer captures `SessionID` from the current execution context. When the timer fires, it passes this sessionID to `ExecuteTask`, which automatically loads all previous conversation history. The agent resumes with full context of why the timer was set.

## Configuration

```yaml
proactive:
  timer:
    enabled: true
    store_path: ~/.alex/timers
    max_timers: 100
    task_timeout_seconds: 900
```
