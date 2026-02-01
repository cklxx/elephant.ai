# Roadmap

## Vision

elephant.ai is an out-of-the-box Lark-native proactive AI assistant.
One LLM API key + one Lark bot config = a persistent, context-aware agent that manages your calendar, tasks, and daily workflows autonomously.
North-star metrics: **WTCR** (Weighted Task Completion Rate), **TimeSaved**, **Accuracy**.

## North Star: Calendar + Tasks Closed Loop

The primary vertical slice: the assistant reads your calendar and tasks, reminds you proactively, creates/updates events and tasks on your behalf (with approval), and learns from your patterns over time.

## Current State (2026-02-01)

M0 is ~85% complete for the agent core. Lark IM integration (WebSocket, chat history, reactions, approval gates) is production-ready. **Critical gaps**: Lark API client layer for Calendar/Tasks does not exist yet; Coding Agent Gateway is unbuilt; Shadow Agent framework is unbuilt. The ReAct loop, 5 LLM providers, 69+ tools, 12 skills, sandbox execution, and web dashboard are all operational.

---

## P0: Blocks North Star (M0)

Items that must ship before the calendar + tasks loop works end-to-end.

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Lark API client layer | No wrapper for Calendar/Tasks/Docs APIs exists | **Not started** | Codex X1 | `internal/lark/` (to create) |
| Calendar read/write tools | Can't query or create calendar events | **In progress** | Claude C1 | `internal/tools/builtin/larktools/calendar_*.go` |
| Tasks read/write tools | Can't query or manage tasks | **In progress** | Claude C2 | `internal/tools/builtin/larktools/task_manage.go` |
| Write-op approval gate | Calendar/task writes need explicit user confirmation | **Partial** | Claude C4 | `internal/agent/domain/react/approval.go` (extend) |
| Scheduler reminders | No proactive nudges for upcoming events/deadlines | **Not started** | Claude C7 | `internal/scheduler/` (extend) |
| Tool registration for new Lark tools | New tools must be wired into the registry | **In progress** | Claude C3 | `internal/toolregistry/registry.go` |

## P1: M0 Quality

Items that don't block MVP but are required for production reliability.

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| ReAct checkpoint + resume | Agent can't recover from crashes mid-task | **Not started** | Codex X2 | `internal/agent/domain/react/` |
| Graceful shutdown | SIGTERM handling added but needs drain logic | **Partial** | Codex X6 | `cmd/elephant/main.go` |
| Global tool timeout/retry | No unified timeout or retry strategy across tools | **Not started** | Codex X3 | `internal/tools/` |
| NSM metric collection | WTCR/TimeSaved/Accuracy not instrumented | **Not started** | Claude C6 + Codex X7 | `internal/observability/` |
| Token counting precision | Current approximation (len/4) is unreliable | **Not started** | Codex X5 | `internal/llm/` |

## P2: Next Wave (M1)

Enhancements after the core loop is stable.

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Replan + sub-goal decomposition | Complex tasks need dynamic replanning | **Not started** | Codex X10 | `internal/agent/domain/react/` |
| Memory restructuring (D5) | Layered FileStore + daily summaries + long-term extraction | **Not started** | Codex X8 | `internal/memory/` |
| Tool policy framework (D1) | Allow/deny rules per tool per context | **Not started** | Codex X9 | `internal/tools/` |
| Scheduler enhancement (D4) | Job persistence, cooldown, concurrency control | **Not started** | Codex X11 | `internal/scheduler/` |
| Calendar/Tasks full CRUD | Update, delete, conflict detection, multi-calendar | **Not started** | Claude C11 | `internal/tools/builtin/larktools/` |
| Proactive reminders + suggestions | Intent extraction -> draft -> confirm flow | **Not started** | Claude C12 | `internal/agent/` |
| Proactive context injection | Auto-inject recent chat/calendar context before user asks | **Not started** | Claude C10 | `internal/context/` |

## P3: Future (M2+)

Larger bets that depend on M0+M1 foundations.

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Coding Agent Gateway | Full code-gen pipeline: plan, generate, test, fix | **Not started** | Codex X12 | `internal/coding/` (to create) |
| Shadow Agent | Self-iteration with mandatory human approval gates | **Not started** | Codex X13 | `internal/devops/` (to create) |
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
