# Kernel Agent Loop: A Persistent AI Employee

Created: 2026-02-11
Status: **Implemented** (V1)

---

## Overview

The kernel agent loop is a cron-driven orchestrator that periodically dispatches
agent tasks. State is managed entirely in an opaque `STATE.md` file that the agent
reads and writes through tools — the system never parses its content.

## Architecture

```
Cron tick
  │
  ▼
RunCycle
  │
  ├─ 1. StateFile.Read() ──── ~/.alex/kernel/{id}/STATE.md (opaque text)
  │
  ├─ 2. Store.ListRecentByAgent() ──── per-agent most recent dispatch status
  │
  ├─ 3. Planner.Plan(stateContent, recentByAgent)
  │      └─ Inject {STATE} into prompts, skip running agents
  │
  ├─ 4. Store.EnqueueDispatches(specs) ──── kernel_dispatch_tasks table
  │
  ├─ 5. executeDispatches (parallel, maxConcurrent)
  │      ├─ MarkDispatchRunning
  │      ├─ Executor.Execute → AgentCoordinator.ExecuteTask
  │      │    └─ Agent reads/writes STATE.md via file tools during execution
  │      └─ MarkDispatchDone / MarkDispatchFailed
  │
  └─ 6. Return CycleResult
```

## Design Decisions

| # | Decision | Choice |
|---|----------|--------|
| 1 | State storage | **Document-driven** — STATE.md is the sole state source |
| 2 | System responsibility | Loop scheduling + dispatch queue + process management only |
| 3 | Trigger mechanism | Independent cron goroutine (robfig/cron) |
| 4 | Dispatch queue | Postgres `kernel_dispatch_tasks` table |
| 5 | Failure policy | Single-task failure does not block others; partial success |
| 6 | Architecture boundary | Domain types in `domain/kernel`, app layer in `app/agent/kernel` |

## File Layout

### New Files

```
internal/domain/kernel/
  types.go          # Dispatch, DispatchSpec, CycleResult
  store.go          # Store interface (dispatch queue port)

internal/app/agent/kernel/
  config.go         # KernelConfig, AgentConfig
  state_file.go     # Atomic STATE.md read/write (+ tests)
  planner.go        # Planner interface + StaticPlanner (+ tests)
  executor.go       # Executor interface + CoordinatorExecutor (+ tests)
  engine.go         # Engine — RunCycle + Run loop (+ tests)

internal/infra/kernel/
  postgres_store.go # Postgres dispatch store (+ integration tests)

internal/delivery/server/bootstrap/
  kernel.go         # KernelStage bootstrap
```

### Modified Files

| File | Change |
|------|--------|
| `internal/shared/config/types.go` | Added `KernelProactiveConfig` + defaults |
| `internal/shared/config/env_usage_guard_test.go` | Allowlisted kernel test |
| `internal/app/di/container.go` | Added `KernelEngine` interface + field |
| `internal/app/di/container_builder.go` | Added `buildKernelEngine()` wiring |
| `internal/delivery/server/bootstrap/server.go` | Added `KernelStage` to stages |
| `internal/delivery/server/bootstrap/lark.go` | Added `KernelStage` to stages |

## Config

```yaml
proactive:
  kernel:
    enabled: true
    kernel_id: "inner-brain"
    schedule: "*/10 * * * *"
    state_dir: "~/.alex/kernel"
    seed_state: |
      # STATE
      ## identity
      I am an independent researcher focused on AI/tech.
      ## world
      (empty)
      ## priorities
      1. Track AI foundation model progress
      ## boss_model
      - Communication preference: unknown
      ## queue
      (empty)
      ## self_critique
      Cycle 0.
    timeout_seconds: 900
    max_concurrent: 3
    channel: "lark"
    chat_id: "oc_xxx"
    agents:
      - agent_id: "pooda"
        prompt: |
          You are a continuously running research agent...
          Current STATE:
          {STATE}
          Execute the POODA five-step cycle...
        priority: 10
        enabled: true
```

## Defaults

| Parameter | Default |
|-----------|---------|
| `kernel_id` | `"default"` |
| `schedule` | `"*/10 * * * *"` |
| `state_dir` | `"~/.alex/kernel"` |
| `timeout_seconds` | `900` |
| `lease_seconds` | `1800` |
| `max_concurrent` | `3` |

## Test Coverage

- **StateFile**: read non-existent, write+read round-trip, seed idempotency, atomic write, nested dir creation
- **StaticPlanner**: placeholder replacement, disabled agent skip, running agent skip, empty agents
- **Engine**: empty plan, all succeed, partial failure, all fail, seed state, running agent not re-dispatched, stop idempotent
- **PostgresStore**: schema idempotent, enqueue+list, mark done/failed, claim with priority (integration, requires TEST_DATABASE_URL)

## Progress Log

- 2026-02-11: V1 implemented and all tests passing.
