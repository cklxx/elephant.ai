# Roadmap

## Vision

elephant.ai is an out-of-the-box Lark-native proactive AI assistant.
One LLM API key + one Lark bot config = a persistent, context-aware agent that manages your calendar, tasks, and daily workflows autonomously.
North-star metrics: **WTCR** (Weighted Task Completion Rate), **TimeSaved**, **Accuracy**.

## North Star: Calendar + Tasks Closed Loop

The primary vertical slice: the assistant reads your calendar and tasks, reminds you proactively, creates/updates events and tasks on your behalf (with approval), and learns from your patterns over time.

## North Star Metrics (Definitions)

- **WTCR (Weighted Task Completion Rate):** weighted by task difficulty.
- **TimeSaved:** baseline_time − actual_time (track p50/p90).
- **Accuracy:** auto-verification + user confirmation.

**Task difficulty levels (suggested):**
- **L1:** single-step / low risk (retrieval, simple answers).
- **L2:** multi-step / write operations (Doc edits, spreadsheet updates, calendar/task changes).
- **L3:** cross-system / high risk (multi-agent flows, code changes, approval chains).

## OKR Tree (Global)

- **O0 (Product NSM):** complete the Calendar + Tasks closed loop, improve WTCR + TimeSaved.
- **O1 (Agent Core):** planning reliability + proactive context + memory structure.
- **O2 (System Interaction):** tool SLA baseline + routing + scheduler reliability.
- **O3 (Lark Ecosystem):** Calendar/Tasks CRUD + approval gate + proactive follow-up.
- **O4 (Shadow DevOps):** eval/baseline/reporting + human-gated release loop.
- **OS (Shared Infra):** event bus + observability + config/auth/error handling.

## Current State (2026-02-03)

**M0 is complete. M1 (P2) is ~85% complete.** All P0 and P1 items are done. P2 progress: context engineering (priority sorting, cost-aware trimming, token budget, memory flush), tool chain enhancements (SLA metrics, result caching, degradation chain, dynamic scheduler tools), Lark ecosystem (approval API, smart cards, group summary, rich content), LLM intelligence (dynamic model selection, provider health, token budget), and DevOps foundations (signal collection, CI eval gating) are implemented. **Remaining P2 gaps**: replan/sub-goal decomposition (Codex), memory restructuring D5 (Codex), tool SLA profile + dynamic routing. **Evaluation automation + evaluation set construction are in progress** (baseline/challenge eval set scaffolding, rubric + auto/agent judgement pipeline), with remaining work in dataset expansion, judge integration, and reporting. P3 (Coding Agent Gateway, Shadow Agent, Deep Lark) remains future.

## Implementation Audit Notes (2026-02-01)

Snapshot from `docs/roadmap/draft/implementation-audit-2026-02-01.md` (verify against current code):
- Tool registry counted **69+ tools**; permission presets were **5** (Full/ReadOnly/Safe/Sandbox/Architect).
- Skill count recorded **12** (see draft for list).
- RAG used **chromem-go cosine similarity** (no pgvector/BM25); token estimation used **÷4 heuristic**.
- Lark IM auto-sense (all group messages) and reply-to support were already implemented at audit time.
- Lark Docs/Sheets/Wiki remained unimplemented at audit time.

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
| ReAct checkpoint + resume | Agent can't recover from crashes mid-task | **Done** | Claude C11 + Codex X2 | `internal/agent/domain/react/` |
| Graceful shutdown | SIGTERM handling + drain logic | **Done** | Claude C15-C16 | `cmd/alex/main.go`, `internal/lifecycle/` |
| Global tool timeout/retry | Unified timeout/retry + policy rules | **Done** | Claude C13-C14 + Codex X3 | `internal/tools/`, `internal/toolregistry/` |
| NSM metric collection | WTCR/TimeSaved/Accuracy counters | **Done** | Claude C17 | `internal/observability/` |
| Token counting precision | tiktoken-go integration | **Done** | Claude C18 | `internal/llm/` |
| Tool SLA baseline collection | Per-tool latency/cost/reliability/success-rate metrics | **Done** | Claude C27 | `internal/tools/sla.go` |

## P2: Next Wave (M1)

Enhancements after the core loop is stable.

### Agent Core & Memory

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Replan + sub-goal decomposition | Complex tasks need dynamic replanning (DAG) | **Not started** | Codex X6 | `internal/agent/domain/react/`, `internal/agent/planner/` |
| Memory restructuring (D5) | Layered FileStore (entries/daily/MEMORY.md) + daily summaries + long-term extraction + migration | **Not started** | Codex X4 | `internal/memory/` |
| Memory Flush-before-Compaction (D3) | Save context before compression — `AutoCompact` fires event, MemoryFlushHook extracts key info to disk | **Done** | Claude C28 | `internal/context/`, `internal/memory/` |
| Context priority sorting | Rank context fragments by relevance/freshness/importance instead of fixed layer order | **Done** | Claude C32 | `internal/context/priority.go` |
| Cost-aware context trimming | Token budget drives which context to keep; prefer high-value content | **Done** | Claude C33 | `internal/context/trimmer.go` |
| Proactive context injection | Calendar summary builder for context assembly | **Done** | Claude C25 | `internal/context/` |

### Tool Chain & Scheduler

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Tool policy framework (D1) | Allow/deny rules per tool per context | **Done** | Claude C19-C20 + Codex X5 | `internal/tools/` |
| Scheduler enhancement (D4) | Job persistence, cooldown, concurrency control, failure recovery | **Done** | Claude C21-C22 + Codex X7 | `internal/scheduler/` |
| Dynamic scheduler job tool | `scheduler_create/list/delete/pause` — Agent can create scheduled jobs from conversation | **Done** | Claude C29 | `internal/tools/builtin/scheduler/` |
| Scheduler startup recovery | Reload persisted jobs from JobStore on restart, auto-register cron | **Done** | Claude C21 | `internal/scheduler/job_runtime.go` |
| Tool SLA profile + dynamic routing | Build per-tool performance profiles; auto-select tool chain based on SLA | **Not started** | Claude → Codex | `internal/tools/router.go` |
| Auto degradation chain | Cache hit → weaker tool → prompt user, try in sequence | **Done** | Claude C39 | `internal/toolregistry/degradation.go` |
| Tool result caching | Semantic dedup — same query doesn't re-execute | **Done** | Claude C38 | `internal/toolregistry/cache.go` |

### Calendar/Tasks & Lark

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Calendar/Tasks full CRUD | Batch ops, conflict detection, multi-calendar | **Done** | Claude C23-C24 | `internal/lark/` |
| Proactive reminders + suggestions | Intent extraction → draft → confirm flow | **Done** | Claude C26 | `internal/reminder/` |
| Lark smart card interaction | Interactive Cards with buttons for approval/selection/feedback | **Done** | Claude C35 | `internal/lark/cards/` |
| Lark Approval API | Query pending approvals, submit approval requests, track status changes | **Done** | Claude C31 | `internal/lark/approval.go` |
| Proactive group summary | Auto-summarize long group discussions (by message count / time window) | **Done** | Claude C36 | `internal/lark/summary/` |
| Message type enrichment | Send tables, code blocks, rendered Markdown messages | **Done** | Claude C37 | `internal/channels/lark/richcontent/` |

### LLM Intelligence

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Dynamic model selection | Auto-select model based on task type/complexity/context length | **Done** | Claude C44 | `internal/llm/router/` |
| Provider health detection | Real-time probe provider availability, auto-switch on failure | **Done** | Claude C30 | `internal/llm/health.go` |
| Token budget management | Per-task/session token budget; auto-downgrade model on overspend | **Done** | Claude C34 | `internal/context/budget/` |

### DevOps Foundations

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Signal collection framework | Failure trajectories, user feedback (thumbs/text), implicit signals (retries/abandons), usage patterns | **Done** | Claude C43 | `internal/signals/` |
| Evaluation automation | Dimensional scoring (reasoning/tools/UX/cost), baseline management, benchmark dashboard | **In progress** | Claude → Codex | `internal/devops/evaluation/` |
| Evaluation set construction (评测集构建) | 分层评测：基础任务准出评测 + 引导模块升级优化的挑战性评测 | **In progress** | Claude → Codex | `evaluation/` |
| CI evaluation gating | Manual + tag-triggered quick eval with PR gate + result archiving | **Done** | Claude C40 | `evaluation/gate/`, `.github/workflows/eval.yml` |

## P3: Future (M2+)

Larger bets that depend on M0+M1 foundations.

### Coding Agent Gateway

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Gateway abstraction | Unified interface: Submit / Stream / Cancel / Status | **Not started** | Codex X8 | `internal/coding/gateway.go` |
| Multi-adapter framework | Codex CLI, Claude Code CLI, Kimi K2 — pluggable registration | **Not started** | Claude | `internal/coding/adapters/` |
| Local CLI auto-detect | Detect installed coding agent CLIs (`which codex`/`which claude`), auto-register | **Not started** | Claude | `internal/coding/adapters/detect.go` |
| Task translation | User natural language → coding agent structured instructions | **Not started** | Claude | `internal/coding/task.go` |
| Build/test/lint verification | Auto-verify agent output compiles, passes tests, passes lint | **Not started** | Claude | `internal/coding/verify*.go` |
| Fix loop | Verify fail → inject error → agent retry → re-verify, multi-round | **Not started** | Codex | `internal/coding/fix_loop.go` |
| Auto commit + PR | On acceptance: auto commit + create PR + generate description | **Not started** | Claude | `internal/coding/deliver.go` |

### Shadow Agent & DevOps

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Shadow Agent framework | Lifecycle (wake/execute/sleep), intake from issues/signals, task decomposition | **Not started** | Codex X9 | `internal/devops/shadow/` |
| Coding Agent dispatch | Shadow calls Coding Gateway, injects project context + coding standards | **Not started** | Claude | `internal/devops/shadow/dispatcher.go` |
| Verification orchestration | Shadow calls `coding/verify` for build/test/lint/diff review | **Not started** | Claude | `internal/devops/shadow/verify_orchestrator.go` |
| Mandatory human approval | Publish + promotion gated by human approval (non-bypassable) | **Not started** | Claude | `internal/devops/shadow/approval.go` |
| PR automation | Auto-create PR, generate description, monitor CI, fix CI failures | **Not started** | Claude | `internal/devops/merge/` |
| Release automation | Semver from commits, changelog generation, multi-platform build, git tag | **Not started** | Claude | `internal/devops/release/` |
| Agent-driven ops | Anomaly alerts via Lark, error log analysis, cost analysis dashboard | **Not started** | Claude → Codex | `internal/devops/ops/` |
| Self-healing | Auto diagnosis, fix playbooks, auto-rollback on health check failure | **Not started** | Codex | `internal/devops/ops/` |

### Advanced Agent Intelligence

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Multi-agent collaboration | Inter-agent messaging, task dispatch by capability profile, conflict arbitration | **Not started** | Codex | `internal/agent/orchestration/` |
| Multi-path sampling + voting | Critical decisions: sample multiple times + vote for reliability | **Not started** | Codex | `internal/agent/domain/react/voting.go` |
| Confidence modeling | Conclusions bound to evidence + confidence score; low confidence triggers clarification | **Not started** | Codex | `internal/agent/domain/confidence.go` |
| Decision memory | Record key decisions (what + why + context) for future reference | **Done** | Claude C41 | `internal/memory/decision.go` |
| Entity memory | Extract people/projects/concepts from conversations, build entity relations | **Done** | Claude C42 | `internal/memory/entity.go` |
| User preference learning | Extract preferences (language/format/tools/style) from interaction patterns | **Not started** | Claude → Codex | `internal/memory/preferences.go` |

### Deep Lark Ecosystem

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Lark Docs read/write | Read doc content (Docx → Markdown), create/edit/comment, permission management | **Not started** | Claude | `internal/lark/docs/` |
| Lark Sheets/Bitable | Read/write cells/records, data analysis, chart generation | **Not started** | Claude | `internal/lark/sheets/`, `internal/lark/bitable/` |
| Lark Wiki | Browse spaces, read/search/create/update pages, auto-knowledge sedimentation | **Not started** | Claude | `internal/lark/wiki/` |
| Meeting preparation assistant | Auto-summarize related docs, previous minutes, and TODOs before meetings | **Library done, wiring pending** | Claude | `internal/lark/calendar/meetingprep/` |
| Meeting notes auto-generation | Post-meeting: auto-generate and push minutes | **Skill done, automation pending** | Claude | `skills/meeting-notes/` |
| Calendar suggestions | Suggest meeting times based on historical patterns | **Library done, wiring pending** | Claude | `internal/lark/calendar/suggestions/` |

### Platform & Interaction

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| macOS Companion (D6) | Menu bar app, Node Host server, TCC permissions, screen capture + system run | **Not started** | Claude (Swift) | `macos/ElephantCompanion/` |
| Node Host Gateway | Proxy executor for node host tools, dynamic register/unregister, config | **Not started** | Claude | `internal/tools/builtin/nodehost/` |
| Cross-surface session sync | Seamless Lark/Web/CLI handoff for the same session | **Not started** | Codex | `internal/session/` |
| Unified notification center | Push to user's preferred channel | **Not started** | Claude | `internal/notification/` |
| Web execution replay | Step-by-step replay of agent execution + Gantt-style timeline | **Not started** | Claude | `web/components/agent/` |
| CLI pipe mode + daemon | stdin/stdout pipe integration with shell toolchain; background service mode | **Not started** | Claude | `cmd/alex/` |

### Data Processing

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| PDF parsing | Text/table/image extraction from PDFs | **Not started** | Claude | `internal/tools/builtin/fileops/` |
| Excel/CSV processing | Read/write/analyze tabular data | **Not started** | Claude | `internal/tools/builtin/fileops/` |
| Audio transcription | Speech → text | **Not started** | Claude | `internal/tools/builtin/media/` |
| Data analysis + visualization | Statistical analysis + chart generation | **Not started** | Claude | `internal/tools/builtin/data/` |
| User-defined skills | Users define custom skills via Markdown | **Not started** | Claude | `internal/skills/custom.go` |
| Skill composition | Chain multiple skills: research → report → PPT | **Not started** | Claude → Codex | `internal/skills/compose.go` |

### Self-Evolution (M3)

| Item | Why | Status | Owner | Code path |
|------|-----|--------|-------|-----------|
| Self-fix loop | Shadow Agent: detect bug → write fix → test → PR → merge → release, fully automated | **Not started** | Codex | `internal/devops/evolution/self_fix.go` |
| Prompt auto-optimization | Iterate prompts based on eval results + feedback signals | **Not started** | Codex | `internal/devops/evolution/prompt_tuner.go` |
| A/B testing framework | Online vs Test agent comparison with auto-promotion/rollback | **Not started** | Claude → Codex | `internal/devops/evaluation/ab_test.go` |
| Knowledge graph | Entity/relation/event triples for structured reasoning | **Not started** | Codex | `internal/memory/knowledge_graph.go` |
| Cloud execution environments | Per-agent Docker containers, K8s job orchestration, CRIU checkpoints | **Not started** | Claude | `internal/environment/` |

---

## 共享基础设施 (Cross-Track)

以下模块不归属任何单一 Track，为所有 Track 提供基础能力：

| 模块 | 包路径 | 说明 | OpenClaw Delta |
|------|--------|------|------|
| **Event Bus** | `internal/events/` | 统一 pub/sub 事件总线，task/session/system 三级事件，现有 Hook 平滑迁移为 subscriber | **D2** |
| **Observability** | `internal/observability/` | 全链路 Trace、Prometheus Metrics、结构化日志、成本核算 | |
| **Config** | `internal/config/` | YAML 配置管理、环境变量覆盖 | |
| **Auth** | `internal/auth/` | OAuth/Token、路由鉴权 | |
| **Errors** | `internal/errors/` | 错误分类、重试策略 | |
| **Storage** | `internal/storage/` | 通用持久化 | |
| **DI** | `internal/di/` | 依赖注入、服务装配 | |

---

## 跨 Track 边界约定 (OKR 对齐)

| 边界点 | 约定 |
|--------|------|
| **Event Bus** _(D2)_ | `internal/events/` 是共享基础设施；为 KR-S1 提供事件能力。 |
| **验证逻辑** | 统一在 Track 2 的 `coding/verify` 包中实现（Build/Test/Lint/DiffReview）；Track 4 仅编排与审批。 |
| **Coding Agent Gateway** | Track 2 构建能力，Track 4 构建工作流；Gateway 同时服务 Online/Shadow。 |
| **Lark 工具封装** | Track 2 提供工具注册端口，Track 3 提供 Lark 工具实现。 |
| **Node Host** _(D6)_ | Track 2 负责 proxy executor，Track 3 负责 macOS companion app。 |
| **审批门禁** | Shadow Agent 发布**必须人工审批**；写操作在 Lark 侧需审批。 |

---

## 跨 Track 依赖关系 (OKR 驱动)

```
O0 (日程+任务闭环)
├── O1 (规划与记忆) ────── 提升准确率与可恢复性
├── O2 (工具与执行) ────── 提升可靠性与可路由性
├── O3 (Lark 生态) ─────── 交互面闭环 + 审批门禁
└── O4 (Shadow DevOps) ─── 自我迭代但强审批
```

---

## 子 ROADMAP 索引

| Track | 子 ROADMAP 文件 | 内容 |
|-------|----------------|------|
| Track 1 | `docs/roadmap/draft/track1-agent-core.md` | ReAct 循环、LLM 路由、上下文工程、记忆系统的 OKR 拆解 |
| Track 2 | `docs/roadmap/draft/track2-system-interaction.md` | 工具引擎、沙箱、Coding Agent Gateway、数据处理、技能系统的 OKR 拆解 |
| Track 3 | `docs/roadmap/draft/track3-lark-ecosystem.md` | Calendar/Tasks 优先的 Lark 生态 OKR 拆解 |
| Track 4 | `docs/roadmap/draft/track4-shadow-agent-devops.md` | 影子 Agent DevOps OKR 拆解（强人工审批） |
| Master | `docs/roadmap/draft/roadmap-lark-native-proactive-assistant.md` | OKR-First 总体草案 |

---

## 进度追踪

| 日期 | 里程碑 | Track | 更新 |
|------|--------|-------|------|
| 2026-02-01 | M0 | All | Roadmap 创建。M0 大部分基础能力已实现（ReAct、69+ 工具、三端交互、可观测性）。主要缺口：断点续跑、Coding Agent Gateway、Lark API client 封装、CI 评测门禁。 |
| 2026-02-01 | M0 | All | Review 优化：更新产品定位为"开箱即用个人 AI"；修正跨 Track 边界；Shadow Agent 从 M1 移至 M2；新增渐进式能力解锁和本地 CLI 自动探测。 |
| 2026-02-01 | M0 | All | 实现审计：对照代码库校验 Roadmap 标注。修正工具数 83→69+、权限预设三档→五档、技能数 13→12、向量检索 ✅→⚙️（chromem-go，无 pgvector/BM25）、事件一致性 ⚙️→✅、用户干预点 ⚙️→✅、群聊自动感知 ❌→✅、消息引用回复 ❌→✅、定时提醒 ❌→⚙️、超时重试限流 ✅→⚙️。详见 `docs/roadmap/draft/implementation-audit-2026-02-01.md`。 |
| 2026-02-01 | M0-M3 | All | **Roadmap 重构为 OKR-First。** 北极星切片聚焦"日程+任务"闭环，NSM 以 WTCR + TimeSaved + Accuracy 为核心。 |
| 2026-02-02 | M0 | All | Roadmap 复查：对齐 tool policy/timeout-retry 与 scheduler D4 状态；新增评测集构建（基础准出 + 挑战性评测）；补充跨 Track 结构与索引。 |
| 2026-02-02 | M1 | All | **Phase 6 complete (C27-C40).** 14 tasks across 3 batches: Tool SLA, memory flush, scheduler tools, provider health, Lark approval, context engineering (priority/trimming/budget), Lark ecosystem (cards/summary/rich content), tool chain (caching/degradation), CI eval gating. All P0+P1 done, P2 ~85% complete. |
| 2026-02-03 | M1 | All | Roadmap 更新：修正 P1 checkpoint+resume 标记；补齐 Deep Lark 的“库已实现/未接入”状态（meeting prep/suggestions、meeting notes skill）。 |

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
| CLI: TUI, approval flow, session persistence | `cmd/alex/` |
| Observability: traces, metrics, cost accounting | `internal/observability/` |
| SIGTERM handling + cancelable base context | `cmd/alex/main.go` |
| Evaluation suite (SWE-Bench, Agent Eval) | `evaluation/` |
| Tool SLA metrics (latency/error-rate/call-count) | `internal/tools/sla.go` |
| Memory flush before compaction (D3) | `internal/context/`, `internal/memory/` |
| Dynamic scheduler job tools | `internal/tools/builtin/scheduler/` |
| Provider health detection (circuit breaker) | `internal/llm/health.go` |
| Lark Approval API | `internal/lark/approval.go` |
| Context priority sorting + cost-aware trimming | `internal/context/priority.go`, `internal/context/trimmer.go` |
| Token budget management | `internal/context/budget/` |
| Lark smart cards (interactive) | `internal/lark/cards/` |
| Proactive group summary | `internal/lark/summary/` |
| Rich content (posts, tables, Markdown) | `internal/channels/lark/richcontent/` |
| Tool result caching (semantic dedup) | `internal/toolregistry/cache.go` |
| Auto degradation chain (fallback executor) | `internal/toolregistry/degradation.go` |
| CI eval gating (PR soft gate) | `evaluation/gate/`, `.github/workflows/eval.yml` |

---

> Previous detailed roadmaps preserved in `docs/roadmap/draft/`.
> Task split & execution plan: [`docs/plans/2026-02-01-task-split-claude-codex.md`](../plans/2026-02-01-task-split-claude-codex.md)
