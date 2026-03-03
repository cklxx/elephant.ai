# Product Feature Map (User Journey)

Date: 2026-03-03  
Scope: Current `elephant.ai` product logic and feature details, organized by user journey.

## 1) Product design logic

1. Product positioning is an always-on AI teammate, not a passive Q&A bot.
   - Ref: [README.md:8](/Users/bytedance/code/elephant.ai/README.md:8), [README.md:23](/Users/bytedance/code/elephant.ai/README.md:23), [README.md:25](/Users/bytedance/code/elephant.ai/README.md:25)
2. Core direction emphasizes attention-saving + proactive behavior, while keeping user override rights.
   - Ref: [docs/roadmap/roadmap.md:9](/Users/bytedance/code/elephant.ai/docs/roadmap/roadmap.md:9), [docs/roadmap/roadmap.md:13](/Users/bytedance/code/elephant.ai/docs/roadmap/roadmap.md:13)
3. North-star business loop is Calendar + Tasks (read context -> proactive remind/push -> execute updates).
   - Ref: [docs/roadmap/roadmap.md:15](/Users/bytedance/code/elephant.ai/docs/roadmap/roadmap.md:15), [docs/roadmap/roadmap.md:17](/Users/bytedance/code/elephant.ai/docs/roadmap/roadmap.md:17)
4. Runtime layering is Delivery -> Application -> Domain -> Infrastructure, with unified kernel across CLI/Web/Lark.
   - Ref: [docs/reference/lark-web-agent-event-flow.md:14](/Users/bytedance/code/elephant.ai/docs/reference/lark-web-agent-event-flow.md:14), [docs/reference/CURRENT_ARCHITECTURE_OVERVIEW.md:12](/Users/bytedance/code/elephant.ai/docs/reference/CURRENT_ARCHITECTURE_OVERVIEW.md:12)

## 2) Journey-based feature map

| Journey Stage | Trigger | Processing | Output | Constraints / Key Files |
|---|---|---|---|---|
| 1. Input Ingestion | User sends message in Lark/Web | Lark gateway accepts only `text/post`, applies chat-type filtering, empty-content filtering, dedup, and mention normalization | Standardized task text | [message_handler.go:17](/Users/bytedance/code/elephant.ai/internal/delivery/channels/lark/message_handler.go:17), [message_handler.go:49](/Users/bytedance/code/elephant.ai/internal/delivery/channels/lark/message_handler.go:49), [message_handler.go:67](/Users/bytedance/code/elephant.ai/internal/delivery/channels/lark/message_handler.go:67) |
| 2. Session Routing | New task reaches channel task manager | Session selection priority: `awaiting_input` -> `lastSessionID` -> persisted binding -> create new session | Stable `session_id` + context continuity | [task_manager.go:203](/Users/bytedance/code/elephant.ai/internal/delivery/channels/lark/task_manager.go:203), [task_manager.go:219](/Users/bytedance/code/elephant.ai/internal/delivery/channels/lark/task_manager.go:219) |
| 3. Plan & Clarification | User uses `/plan` or flow requires clarification | Plan mode resolves scope and mode (`on/off/auto/status/...`); `ask_user` supports `clarify/request` with strict schema checks | Executable plan or waiting-for-user-input state | [plan_mode.go:25](/Users/bytedance/code/elephant.ai/internal/delivery/channels/lark/plan_mode.go:25), [plan_mode.go:100](/Users/bytedance/code/elephant.ai/internal/delivery/channels/lark/plan_mode.go:100), [ask_user.go:101](/Users/bytedance/code/elephant.ai/internal/infra/tools/builtin/ui/ask_user.go:101), [ask_user.go:171](/Users/bytedance/code/elephant.ai/internal/infra/tools/builtin/ui/ask_user.go:171) |
| 4. Core Execution | Coordinator receives `ExecuteTask` | Preparation (pre-analysis timeout, preset resolution, context assembly) -> ReAct loop (`think/observe/tool`) | Final answer + tool outputs + execution events | [analysis.go:19](/Users/bytedance/code/elephant.ai/internal/app/agent/preparation/analysis.go:19), [preset_resolver.go:56](/Users/bytedance/code/elephant.ai/internal/app/agent/preparation/preset_resolver.go:56), [coordinator.go:207](/Users/bytedance/code/elephant.ai/internal/app/agent/coordinator/coordinator.go:207), [solve.go:17](/Users/bytedance/code/elephant.ai/internal/domain/agent/react/solve.go:17) |
| 5. Multi-agent Orchestration | Complex task requiring decomposition | `run_tasks` loads YAML taskfile/team template, applies `team/swarm/auto`, controls `wait/timeout`, emits status artifact | Subtask results + status file | [run_tasks.go:35](/Users/bytedance/code/elephant.ai/internal/infra/tools/builtin/orchestration/run_tasks.go:35), [run_tasks.go:160](/Users/bytedance/code/elephant.ai/internal/infra/tools/builtin/orchestration/run_tasks.go:160), [run_tasks.go:292](/Users/bytedance/code/elephant.ai/internal/infra/tools/builtin/orchestration/run_tasks.go:292) |
| 6. Streaming Response | Execution emits runtime events | Frontend `useSSE` does connection + pipeline + batching; UI filters by active `run_id` and clears state on terminal events | Real-time delta/final/error rendering | [ConversationPageContent.tsx:103](/Users/bytedance/code/elephant.ai/web/app/conversation/ConversationPageContent.tsx:103), [ConversationPageContent.tsx:141](/Users/bytedance/code/elephant.ai/web/app/conversation/ConversationPageContent.tsx:141), [useSSE.ts:29](/Users/bytedance/code/elephant.ai/web/hooks/useSSE/useSSE.ts:29), [useSSE.ts:301](/Users/bytedance/code/elephant.ai/web/hooks/useSSE/useSSE.ts:301) |
| 7. Memory Injection | Task needs historical experience/context | Memory engine provides search/related/get-lines over markdown-based storage | Retrieved memory snippets for context injection | [md_store.go:91](/Users/bytedance/code/elephant.ai/internal/infra/memory/md_store.go:91), [md_store.go:143](/Users/bytedance/code/elephant.ai/internal/infra/memory/md_store.go:143), [md_store.go:197](/Users/bytedance/code/elephant.ai/internal/infra/memory/md_store.go:197) |
| 8. Risk Control | Before and during tool execution | Tool policy suppresses retries for higher risk (`L3/L4`, dangerous), with first-match rule ordering | Controlled tool execution risk profile | [policy.go:120](/Users/bytedance/code/elephant.ai/internal/infra/tools/policy.go:120), [policy.go:159](/Users/bytedance/code/elephant.ai/internal/infra/tools/policy.go:159), [policy.go:265](/Users/bytedance/code/elephant.ai/internal/infra/tools/policy.go:265) |

## 3) Simplified end-to-end sequence

1. User input (Lark/Web)  
2. Channel layer parses and binds session  
3. Coordinator `ExecuteTask` starts preparation and domain solve  
4. ReAct loop performs reasoning/tool calls  
5. Events stream to channel/UI and terminal state is rendered

Refs: [task_manager.go:310](/Users/bytedance/code/elephant.ai/internal/delivery/channels/lark/task_manager.go:310), [coordinator.go:207](/Users/bytedance/code/elephant.ai/internal/app/agent/coordinator/coordinator.go:207), [solve.go:38](/Users/bytedance/code/elephant.ai/internal/domain/agent/react/solve.go:38), [useSSE.ts:301](/Users/bytedance/code/elephant.ai/web/hooks/useSSE/useSSE.ts:301)

## 4) Current documentation consistency risks

1. Multiple roadmap docs are maintained in parallel, with possible status drift.
   - Ref: [docs/roadmap/roadmap-2026-02-27.md:5](/Users/bytedance/code/elephant.ai/docs/roadmap/roadmap-2026-02-27.md:5), [docs/roadmap/roadmap.md:1](/Users/bytedance/code/elephant.ai/docs/roadmap/roadmap.md:1)
2. Tool policy rule order directly affects risk behavior due to first-match semantics.
   - Ref: [policy.go:265](/Users/bytedance/code/elephant.ai/internal/infra/tools/policy.go:265), [policy.go:279](/Users/bytedance/code/elephant.ai/internal/infra/tools/policy.go:279)
