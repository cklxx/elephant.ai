# Roadmap

## Vision

elephant.ai is an out-of-the-box Lark-native proactive AI assistant.
One LLM API key + one Lark bot config = a persistent, context-aware agent that manages your calendar, tasks, and daily workflows autonomously.
North-star metrics: **WTCR** (Weighted Task Completion Rate), **TimeSaved**, **Accuracy**.

## North Star: Calendar + Tasks Closed Loop

The primary vertical slice: the assistant reads your calendar and tasks, reminds you proactively, creates/updates events and tasks on your behalf (with approval), and learns from your patterns over time.

## Current State (2026-02-02)

M0 is ~95% complete. Lark API client layer (auth, Calendar, Tasks), CRUD tools, approval gates, scheduler reminders, and E2E wiring are all done. P1 quality layer (checkpoint schema, graceful shutdown, tool policy, NSM metrics, token counting) is complete. P2 batch ops, conflict detection, calendar summary, job persistence, and reminder pipeline are implemented. **Remaining gaps**: Codex X2 (checkpoint engine), X3 (retry middleware), X7 (scheduler concurrency) — prompts prepared. Coding Agent Gateway and Shadow Agent are future.

---

## P0: Blocks North Star (M0)

Items that must ship before the calendar + tasks loop works end-to-end.

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Lark API client layer | Auth, Calendar, Tasks wrappers | **Done** | Claude C1-C4 | `internal/lark/` |
| Calendar read/write tools | Query, create, update, delete events | **Done** | Claude C5 | `internal/tools/builtin/larktools/calendar_*.go` |
| Tasks read/write tools | CRUD + batch operations | **Done** | Claude C6 | `internal/tools/builtin/larktools/task_manage.go` |
| Write-op approval gate | Dangerous flag + approval executor | **Done** | Claude C8 | `internal/toolregistry/registry.go` |
| Scheduler reminders | Calendar trigger wired into scheduler | **Done** | Claude C10 | `internal/scheduler/` |
| Tool registration for new Lark tools | All tools registered in registry | **Done** | Claude C7 | `internal/toolregistry/registry.go` |
| E2E integration test | Full calendar flow E2E | **Done** | Codex X1 | `internal/scheduler/calendar_flow_e2e_test.go` |

## P1: M0 Quality

Items that don't block MVP but are required for production reliability.

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| ReAct checkpoint + resume | Agent can't recover from crashes mid-task | **Schema done, engine blocked** | Claude C11 + Codex X2 | `internal/agent/domain/react/` |
| Graceful shutdown | SIGTERM handling + drain logic | **Done** | Claude C15-C16 | `cmd/elephant/main.go`, `internal/lifecycle/` |
| Global tool timeout/retry | Unified timeout/retry + policy rules | **Schema done, middleware blocked** | Claude C13-C14 + Codex X3 | `internal/tools/`, `internal/toolregistry/` |
| NSM metric collection | WTCR/TimeSaved/Accuracy counters | **Done** | Claude C17 | `internal/observability/` |
| Token counting precision | tiktoken-go integration | **Done** | Claude C18 | `internal/llm/` |

## P2: Next Wave (M1)

Enhancements after the core loop is stable.

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Replan + sub-goal decomposition | Complex tasks need dynamic replanning | **Not started** | Codex X6 | `internal/agent/domain/react/` |
| Memory restructuring (D5) | Layered FileStore + daily summaries + long-term extraction | **Not started** | Codex X4 | `internal/memory/` |
| Tool policy framework (D1) | Allow/deny rules per tool per context | **Schema + rules done, engine blocked** | Claude C19-C20 + Codex X5 | `internal/tools/` |
| Scheduler enhancement (D4) | Job persistence, cooldown, concurrency control | **JobStore done, enhancement blocked** | Claude C21-C22 + Codex X7 | `internal/scheduler/` |
| Calendar/Tasks full CRUD | Batch ops, conflict detection, multi-calendar | **Done** | Claude C23-C24 | `internal/lark/`, `internal/tools/builtin/larktools/` |
| Proactive reminders + suggestions | Intent extraction → draft → confirm flow | **Done** | Claude C26 | `internal/reminder/` |
| Proactive context injection | Calendar summary builder for context assembly | **Done** | Claude C25 | `internal/context/` |

## P3: Future (M2+)

Larger bets that depend on M0+M1 foundations.

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Coding Agent Gateway | Full code-gen pipeline: plan, generate, test, fix | **Not started** | Codex X8 | `internal/coding/` (to create) |
| Shadow Agent | Self-iteration with mandatory human approval gates | **Not started** | Codex X9 | `internal/devops/` (to create) |
| macOS Companion (D6) | Native app + Node Host Gateway for desktop integration | **Not started** | TBD | TBD |
| Multi-agent collaboration | Parallel agent execution with shared context | **Not started** | TBD | `internal/agent/` |
| Deep Lark ecosystem | Docs, Sheets, Wiki, Approval workflow integration | **Not started** | TBD | `internal/tools/builtin/larktools/` |

---

## Completed (Reference)

| Capability | Code path |
|------------|-----------|
| ReAct loop (Think -> Act -> Observe) | `internal/agent/domain/react/` |
| 5 LLM providers (OpenAI, Claude, ARK, DeepSeek, Ollama) | `internal/llm/` |
| 69+ tools, 5 permission presets | `internal/tools/`, `internal/toolregistry/` |
| 12 skills (research, meeting notes, email, slides, video) | `skills/` |
| Lark IM: WebSocket, chat history, reactions, approval gates | `internal/channels/lark/` |
| Sandbox: code exec, shell, browser automation, file ops | `internal/tools/builtin/sandbox/` |
| 4-layer context assembly + dynamic compression | `internal/context/` |
| Memory: conversation persistence + vector retrieval (chromem-go) | `internal/memory/` |
| Web dashboard: SSE streaming, conversations, cost tracking | `web/` |
| CLI: TUI, approval flow, session persistence | `cmd/elephant/` |
| Observability: traces, metrics, cost accounting | `internal/observability/` |
| SIGTERM handling + cancelable base context | `cmd/elephant/main.go` |
| Evaluation suite (SWE-Bench, Agent Eval) | `internal/eval/` |

---

> Previous detailed roadmaps preserved in `docs/roadmap/draft/`.
> Task split & execution plan: [`docs/plans/2026-02-01-task-split-claude-codex.md`](../plans/2026-02-01-task-split-claude-codex.md)
