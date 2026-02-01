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
| Tool SLA baseline collection | Per-tool latency/cost/reliability/success-rate metrics | **Not started** | TBD | `internal/tools/sla.go` |

## P2: Next Wave (M1)

Enhancements after the core loop is stable.

### Agent Core & Memory

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Replan + sub-goal decomposition | Complex tasks need dynamic replanning (DAG) | **Not started** | Codex X6 | `internal/agent/domain/react/`, `internal/agent/planner/` |
| Memory restructuring (D5) | Layered FileStore (entries/daily/MEMORY.md) + daily summaries + long-term extraction + migration | **Not started** | Codex X4 | `internal/memory/` |
| Memory Flush-before-Compaction (D3) | Save context before compression — `AutoCompact` fires event, MemoryFlushHook extracts key info to disk | **Not started** | TBD | `internal/context/`, `internal/memory/` |
| Context priority sorting | Rank context fragments by relevance/freshness/importance instead of fixed layer order | **Not started** | TBD | `internal/context/manager.go` |
| Cost-aware context trimming | Token budget drives which context to keep; prefer high-value content | **Not started** | TBD | `internal/context/manager_window.go` |
| Proactive context injection | Calendar summary builder for context assembly | **Done** | Claude C25 | `internal/context/` |

### Tool Chain & Scheduler

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Tool policy framework (D1) | Allow/deny rules per tool per context | **Schema + rules done, engine blocked** | Claude C19-C20 + Codex X5 | `internal/tools/` |
| Scheduler enhancement (D4) | Job persistence, cooldown, concurrency control, failure recovery | **JobStore done, enhancement blocked** | Claude C21-C22 + Codex X7 | `internal/scheduler/` |
| Dynamic scheduler job tool | `scheduler_create/list/delete/pause` — Agent can create scheduled jobs from conversation | **Not started** | TBD | `internal/tools/builtin/session/scheduler_tool.go` |
| Scheduler startup recovery | Reload persisted jobs from JobStore on restart, auto-register cron | **Not started** | TBD | `internal/scheduler/scheduler.go` |
| Tool SLA profile + dynamic routing | Build per-tool performance profiles; auto-select tool chain based on SLA | **Not started** | TBD | `internal/tools/router.go` |
| Auto degradation chain | Cache hit → weaker tool → prompt user, try in sequence | **Not started** | TBD | `internal/tools/fallback.go` |
| Tool result caching | Semantic dedup — same query doesn't re-execute | **Not started** | TBD | `internal/tools/cache.go` |

### Calendar/Tasks & Lark

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Calendar/Tasks full CRUD | Batch ops, conflict detection, multi-calendar | **Done** | Claude C23-C24 | `internal/lark/` |
| Proactive reminders + suggestions | Intent extraction → draft → confirm flow | **Done** | Claude C26 | `internal/reminder/` |
| Lark smart card interaction | Interactive Cards with buttons for approval/selection/feedback | **Not started** | TBD | `internal/channels/lark/cards/` |
| Lark Approval API | Query pending approvals, submit approval requests, track status changes | **Not started** | TBD | `internal/lark/approval/` |
| Proactive group summary | Auto-summarize long group discussions (by message count / time window) | **Not started** | TBD | `internal/channels/lark/proactive.go` |
| Message type enrichment | Send tables, code blocks, rendered Markdown messages | **Not started** | TBD | `internal/channels/lark/` |

### LLM Intelligence

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Dynamic model selection | Auto-select model based on task type/complexity/context length | **Not started** | TBD | `internal/llm/router.go` |
| Provider health detection | Real-time probe provider availability, auto-switch on failure | **Not started** | TBD | `internal/llm/health.go` |
| Token budget management | Per-task/session token budget; auto-downgrade model on overspend | **Not started** | TBD | `internal/llm/budget.go` |

### DevOps Foundations

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Signal collection framework | Failure trajectories, user feedback (thumbs/text), implicit signals (retries/abandons), usage patterns | **Not started** | TBD | `internal/devops/signals/` |
| Evaluation automation | Dimensional scoring (reasoning/tools/UX/cost), baseline management, benchmark dashboard | **Not started** | TBD | `internal/devops/evaluation/` |
| CI evaluation gating | Manual + tag-triggered quick eval with result archiving | **Partial** | TBD | `.github/workflows/eval.yml` |

## P3: Future (M2+)

Larger bets that depend on M0+M1 foundations.

### Coding Agent Gateway

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Gateway abstraction | Unified interface: Submit / Stream / Cancel / Status | **Not started** | Codex X8 | `internal/coding/gateway.go` |
| Multi-adapter framework | Codex CLI, Claude Code CLI, Kimi K2 — pluggable registration | **Not started** | TBD | `internal/coding/adapters/` |
| Local CLI auto-detect | Detect installed coding agent CLIs (`which codex`/`which claude`), auto-register | **Not started** | TBD | `internal/coding/adapters/detect.go` |
| Task translation | User natural language → coding agent structured instructions | **Not started** | TBD | `internal/coding/task.go` |
| Build/test/lint verification | Auto-verify agent output compiles, passes tests, passes lint | **Not started** | TBD | `internal/coding/verify*.go` |
| Fix loop | Verify fail → inject error → agent retry → re-verify, multi-round | **Not started** | TBD | `internal/coding/fix_loop.go` |
| Auto commit + PR | On acceptance: auto commit + create PR + generate description | **Not started** | TBD | `internal/coding/deliver.go` |

### Shadow Agent & DevOps

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Shadow Agent framework | Lifecycle (wake/execute/sleep), intake from issues/signals, task decomposition | **Not started** | Codex X9 | `internal/devops/shadow/` |
| Coding Agent dispatch | Shadow calls Coding Gateway, injects project context + coding standards | **Not started** | TBD | `internal/devops/shadow/dispatcher.go` |
| Verification orchestration | Shadow calls `coding/verify` for build/test/lint/diff review | **Not started** | TBD | `internal/devops/shadow/verify_orchestrator.go` |
| Mandatory human approval | Publish + promotion gated by human approval (non-bypassable) | **Not started** | TBD | `internal/devops/shadow/approval.go` |
| PR automation | Auto-create PR, generate description, monitor CI, fix CI failures | **Not started** | TBD | `internal/devops/merge/` |
| Release automation | Semver from commits, changelog generation, multi-platform build, git tag | **Not started** | TBD | `internal/devops/release/` |
| Agent-driven ops | Anomaly alerts via Lark, error log analysis, cost analysis dashboard | **Not started** | TBD | `internal/devops/ops/` |
| Self-healing | Auto diagnosis, fix playbooks, auto-rollback on health check failure | **Not started** | TBD | `internal/devops/ops/` |

### Advanced Agent Intelligence

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Multi-agent collaboration | Inter-agent messaging, task dispatch by capability profile, conflict arbitration | **Not started** | TBD | `internal/agent/orchestration/` |
| Multi-path sampling + voting | Critical decisions: sample multiple times + vote for reliability | **Not started** | TBD | `internal/agent/domain/react/voting.go` |
| Confidence modeling | Conclusions bound to evidence + confidence score; low confidence triggers clarification | **Not started** | TBD | `internal/agent/domain/confidence.go` |
| Decision memory | Record key decisions (what + why + context) for future reference | **Not started** | TBD | `internal/memory/decision_store.go` |
| Entity memory | Extract people/projects/concepts from conversations, build entity relations | **Not started** | TBD | `internal/memory/entity.go` |
| User preference learning | Extract preferences (language/format/tools/style) from interaction patterns | **Not started** | TBD | `internal/memory/preferences.go` |

### Deep Lark Ecosystem

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Lark Docs read/write | Read doc content (Docx → Markdown), create/edit/comment, permission management | **Not started** | TBD | `internal/lark/docs/` |
| Lark Sheets/Bitable | Read/write cells/records, data analysis, chart generation | **Not started** | TBD | `internal/lark/sheets/`, `internal/lark/bitable/` |
| Lark Wiki | Browse spaces, read/search/create/update pages, auto-knowledge sedimentation | **Not started** | TBD | `internal/lark/wiki/` |
| Meeting preparation assistant | Auto-summarize related docs, previous minutes, and TODOs before meetings | **Not started** | TBD | `internal/lark/calendar/` |
| Meeting notes auto-generation | Post-meeting: auto-generate and push minutes | **Not started** | TBD | `skills/meeting-notes/` |
| Calendar suggestions | Suggest meeting times based on historical patterns | **Not started** | TBD | `internal/lark/calendar/` |

### Platform & Interaction

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| macOS Companion (D6) | Menu bar app, Node Host server, TCC permissions, screen capture + system run | **Not started** | TBD | `macos/ElephantCompanion/` |
| Node Host Gateway | Proxy executor for node host tools, dynamic register/unregister, config | **Not started** | TBD | `internal/tools/builtin/nodehost/` |
| Cross-surface session sync | Seamless Lark/Web/CLI handoff for the same session | **Not started** | TBD | `internal/session/` |
| Unified notification center | Push to user's preferred channel | **Not started** | TBD | `internal/notification/` |
| Web execution replay | Step-by-step replay of agent execution + Gantt-style timeline | **Not started** | TBD | `web/components/agent/` |
| CLI pipe mode + daemon | stdin/stdout pipe integration with shell toolchain; background service mode | **Not started** | TBD | `cmd/elephant/` |

### Data Processing

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| PDF parsing | Text/table/image extraction from PDFs | **Not started** | TBD | `internal/tools/builtin/fileops/` |
| Excel/CSV processing | Read/write/analyze tabular data | **Not started** | TBD | `internal/tools/builtin/fileops/` |
| Audio transcription | Speech → text | **Not started** | TBD | `internal/tools/builtin/media/` |
| Data analysis + visualization | Statistical analysis + chart generation | **Not started** | TBD | `internal/tools/builtin/data/` |
| User-defined skills | Users define custom skills via Markdown | **Not started** | TBD | `internal/skills/custom.go` |
| Skill composition | Chain multiple skills: research → report → PPT | **Not started** | TBD | `internal/skills/compose.go` |

### Self-Evolution (M3)

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Self-fix loop | Shadow Agent: detect bug → write fix → test → PR → merge → release, fully automated | **Not started** | TBD | `internal/devops/evolution/self_fix.go` |
| Prompt auto-optimization | Iterate prompts based on eval results + feedback signals | **Not started** | TBD | `internal/devops/evolution/prompt_tuner.go` |
| A/B testing framework | Online vs Test agent comparison with auto-promotion/rollback | **Not started** | TBD | `internal/devops/evaluation/ab_test.go` |
| Knowledge graph | Entity/relation/event triples for structured reasoning | **Not started** | TBD | `internal/memory/knowledge_graph.go` |
| Cloud execution environments | Per-agent Docker containers, K8s job orchestration, CRIU checkpoints | **Not started** | TBD | `internal/environment/` |

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
