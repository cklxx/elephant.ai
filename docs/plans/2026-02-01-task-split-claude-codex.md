# Task Split: Claude Code vs Codex

Created: 2026-02-01

## Splitting Principle

- **Claude Code** (you, interactive): architecture decisions, multi-file integration, complex wiring, debugging, planning — anything that requires reading context across packages and making judgment calls.
- **Codex** (batch, autonomous): well-scoped, isolated implementations with clear inputs/outputs — single-file tools, tests, mechanical additions.

---

## P0: Blocks North Star

### Claude Code Tasks

| # | Task | Why Claude Code | Touches |
|---|------|-----------------|---------|
| C1 | Design & implement `internal/lark/` API client layer | Architecture decision: thin wrapper vs full SDK facade, error mapping, auth token refresh, rate limiting. Multi-file, cross-package. | `internal/lark/`, `internal/channels/lark/`, config |
| C2 | Wire scheduler reminders for calendar/tasks | Requires understanding scheduler→agent coordinator→tool flow, designing trigger format for calendar events, integrating with existing OKR trigger pattern. | `internal/scheduler/`, `internal/agent/` |
| C3 | Extend approval gate for calendar/task writes | Decide: per-tool Dangerous flag (current) vs per-action granularity. May need approval message formatting for calendar event previews. | `internal/toolregistry/`, `internal/agent/domain/react/` |
| C4 | Integration test: full calendar flow end-to-end | Requires mock Lark client, approval flow simulation, scheduler trigger → tool execution → result verification. Multi-package. | `internal/tools/builtin/larktools/`, `internal/toolregistry/`, `internal/scheduler/` |
| C5 | ReAct checkpoint + resume (P1) | Core architecture change: serialize engine state, define resume points, handle tool-in-flight recovery. | `internal/agent/domain/react/` |
| C6 | Global tool timeout/retry strategy (P1) | Cross-cutting concern: needs design decision on retry policy (exponential backoff, circuit breaker), timeout inheritance from parent context. | `internal/tools/`, `internal/toolregistry/` |

### Codex Tasks

| # | Task | Spec | Touches |
|---|------|------|---------|
| X1 | Implement `lark_calendar_update` tool | Same pattern as `calendar_create.go`. Input: event_id + fields to update. Use Lark SDK `PatchCalendarEvent`. Dangerous=true. | `internal/tools/builtin/larktools/calendar_update.go` |
| X2 | Implement `lark_calendar_delete` tool | Same pattern. Input: event_id. Use Lark SDK `DeleteCalendarEvent`. Dangerous=true. | `internal/tools/builtin/larktools/calendar_delete.go` |
| X3 | Implement `lark_task_update` tool | Same pattern as `task_manage.go` create action. Add "update" action: task_id + fields. Dangerous=true. | Extend `task_manage.go` or new file |
| X4 | Implement `lark_task_delete` tool | Same pattern. Input: task_id. Dangerous=true. | Extend `task_manage.go` or new file |
| X5 | Add unit tests for calendar_update, calendar_delete | Follow existing test pattern in `calendar_create_test.go`: missing client, invalid args, invalid client type. | `*_test.go` |
| X6 | Add unit tests for task_update, task_delete | Follow existing test pattern in `task_manage_test.go`. | `*_test.go` |
| X7 | Add NSM metric stubs to observability (P1) | Add `wtcr`, `timeSaved`, `accuracy` counters/histograms to `MetricsCollector`. No business logic — just instrument definitions. | `internal/observability/metrics.go` |
| X8 | Token counting: replace len/4 with tiktoken-go (P1) | Isolated change in LLM token counting. Use `github.com/pkoukk/tiktoken-go`. Single file change. | `internal/llm/` (token counting file) |

---

## P1: Quality

### Claude Code Tasks

| # | Task | Why Claude Code |
|---|------|-----------------|
| C7 | Graceful shutdown drain logic | Needs to understand in-flight tool execution, SSE connections, scheduler jobs. Coordinate across multiple subsystems. |
| C8 | NSM metric collection wiring | After X7 creates stubs, wire actual measurement points: WTCR at task completion, TimeSaved estimation, Accuracy from user feedback. Cross-package. |

### Codex Tasks

Already covered by X7, X8 above.

---

## P2: Next Wave

### Claude Code Tasks

| # | Task | Why Claude Code |
|---|------|-----------------|
| C9 | Replan + sub-goal decomposition | Core ReAct loop change. Needs design: when to trigger replan, how to decompose, state management for sub-goals. |
| C10 | Memory restructuring (D5) | Architecture: layered FileStore, daily summary pipeline, long-term extraction. Touches memory, context, and agent layers. |
| C11 | Tool policy framework (D1) | Design allow/deny rules, per-context tool filtering, policy evaluation in registry. |
| C12 | Scheduler enhancement (D4) | Job persistence (what backend?), cooldown logic, concurrency control. Extends existing scheduler significantly. |

### Codex Tasks

| # | Task | Spec |
|---|------|------|
| X9 | Calendar conflict detection | Given a time range, query existing events and return conflicts. Pure function, well-scoped. |
| X10 | Proactive context injection: calendar summary builder | Given today's events, format a markdown summary for context injection. Pure function. |

---

## Execution Order

```
Phase 1 (now):  C1 → X1,X2,X3,X4 (parallel once C1 provides client)
                C3 (can start immediately)
Phase 2:        X5,X6 (after X1-X4)
                C2, C4 (after Phase 1)
Phase 3 (P1):   C5,C6,C7 (parallel)
                X7,X8 (parallel, independent)
                C8 (after X7)
Phase 4 (P2):   C9,C10,C11,C12 (sequential, each is significant)
                X9,X10 (after C12, C2)
```

---

## Codex Prompt Template

When dispatching to Codex, use this format:

```
Context: elephant.ai Go project. Look at internal/tools/builtin/larktools/calendar_create.go
for the exact pattern to follow (BaseTool, ports.ToolDefinition, Execute method,
shared.LarkClientFromContext, approval via Dangerous flag).

Task: Implement lark_calendar_update tool in internal/tools/builtin/larktools/calendar_update.go

Requirements:
- Input: event_id (required), calendar_id (optional, default "primary"), summary, description,
  start_time, end_time (unix seconds)
- Use Lark SDK client.Calendar.PatchCalendarEvent()
- Set Dangerous: true in ToolMetadata
- Return updated event details as JSON
- Follow exact same error handling pattern as calendar_create.go

Test: Add calendar_update_test.go following calendar_create_test.go pattern.
```
