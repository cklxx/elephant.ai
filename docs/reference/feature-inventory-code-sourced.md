# Feature Inventory (Code-Sourced)

Updated: 2026-02-26

This document inventories currently supported features from source code only (not docs), with product and technical views.

Code surface scanned:
- `cmd/`
- `internal/delivery/`
- `internal/app/`
- `internal/domain/agent/`
- `internal/infra/`
- `web/app/`, `web/lib/api.ts`

Priority definition:
- `P0`: core path; broken means primary task execution is blocked.
- `P1`: high-value extension; degraded experience if missing.
- `P2`: developer/ops/advanced or optional capability.

## Product View

### P0 Features

1. Multi-surface agent task execution (CLI/Web/Lark)
- User value: users can submit tasks and get streamed progress/final answers across interfaces.
- Surfaces: `alex <task>`, `/conversation`, Lark inbound message handling.
- Evidence: `cmd/alex/cli.go`, `web/app/conversation/ConversationPageContent.tsx`, `internal/delivery/channels/lark/gateway.go`.

2. ReAct execution engine (think-act-observe with tool use)
- User value: tasks can iterate with reasoning + tools until completion/stop condition.
- Evidence: `internal/domain/agent/react/engine.go`, `internal/domain/agent/react/observe.go`, `internal/domain/agent/react/tool_batch.go`.

3. Unified context assembly + execution preparation
- User value: tasks run with coherent system prompt, history, memory, toolset, attachments, and config.
- Evidence: `internal/app/context/manager_window.go`, `internal/app/context/manager_prompt.go`, `internal/app/agent/preparation/service.go`.

4. HTTP API backbone + SSE streaming
- User value: web/clients consume real-time task events and management APIs.
- Evidence: `internal/delivery/server/http/router.go`, `internal/delivery/server/http/sse_handler_stream.go`, `internal/delivery/server/app/event_broadcaster.go`.

5. Session lifecycle management
- User value: users can create/list/delete/fork/replay/share sessions; fetch snapshots/turns/persona.
- Evidence: `internal/delivery/server/http/router.go`, `internal/delivery/server/app/session_service.go`, `internal/delivery/server/app/snapshot_service.go`.

6. Core built-in tooling for local execution
- User value: agent can read/write/replace files, run shell commands, and execute code.
- Evidence: `internal/app/toolregistry/registry_builtins.go`, `internal/infra/tools/builtin/aliases/read_file.go`, `internal/infra/tools/builtin/aliases/shell_exec.go`, `internal/infra/tools/builtin/aliases/execute_code.go`.

7. Tool governance and safety wrappers
- User value: risky operations pass approval policy/retry/SLA/circuit-breaker layers.
- Evidence: `internal/app/toolregistry/registry.go`, `internal/infra/tools/policy.go`, `internal/domain/agent/ports/tools/approval.go`.

8. Persistent memory (Markdown-backed) + retrieval tools
- User value: memory survives sessions and can be queried/linked from agent runs.
- Evidence: `internal/infra/memory/md_store.go`, `internal/infra/tools/builtin/memory/memory_search.go`, `internal/infra/tools/builtin/memory/memory_related.go`.

9. Multi-provider LLM access with resilience
- User value: runtime can route to OpenAI/Claude/Kimi/etc with retry/rate-limit/tool-call parsing.
- Evidence: `internal/infra/llm/factory.go`, `internal/infra/llm/retry_client.go`, `internal/infra/llm/tool_call_parsing_client.go`.

10. Observability + cost accounting
- User value: token/cost/latency/tracing are measurable in production.
- Evidence: `internal/infra/observability/instrumentation.go`, `internal/infra/observability/metrics.go`, `internal/infra/observability/tracing.go`.

11. Lark unified channel operations (enterprise integration)
- User value: one tool handles messaging/calendar/tasks/docs/wiki/bitable/drive/sheets/OKR/contact/mail/VC actions in Lark context.
- Evidence: `internal/infra/tools/builtin/larktools/channel.go`.

### P1 Features

1. ACP server + ACP HTTP/SSE gateway
- User value: external ACP clients can open/load sessions, prompt, switch mode, and receive updates.
- Evidence: `cmd/alex/acp_server.go`, `cmd/alex/acp_http.go`, `cmd/alex/acp.go`.

2. MCP integration and dynamic tool loading
- User value: users can add/remove/restart MCP servers and consume server-provided tools/resources.
- Evidence: `cmd/alex/mcp.go`, `internal/infra/mcp/`, `internal/app/toolregistry/registry.go`.

3. External coding agent routing (Claude Code / Codex / Kimi)
- User value: coding tasks can be delegated to external executors with execution controls.
- Evidence: `internal/infra/external/registry.go`, `internal/infra/external/bridge/executor.go`, `internal/infra/coding/gateway.go`.

4. Background orchestration tools
- User value: taskfile/template-driven subtasks can run asynchronously with status reporting.
- Evidence: `internal/infra/tools/builtin/orchestration/run_tasks.go`, `internal/infra/tools/builtin/orchestration/reply_agent.go`, `internal/domain/agent/taskfile/`.

5. Lark task-control command set
- User value: `/cc`, `/codex`, `/task`, `/tasks`, `/stop`, `/new`, `/model`, `/plan`, `/notice` control execution directly in chat.
- Evidence: `internal/delivery/channels/lark/task_command.go`, `internal/delivery/channels/lark/task_manager.go`, `internal/delivery/channels/lark/model_command.go`, `internal/delivery/channels/lark/plan_mode.go`, `internal/delivery/channels/lark/notice_command.go`.

6. Web session management and sharing UX
- User value: browse session archive, delete/fork sessions, create public share links, replay timeline.
- Evidence: `web/app/sessions/page.tsx`, `web/app/share/SharePageContent.tsx`, `web/lib/api.ts`.

7. Web evaluation dashboard
- User value: launch and inspect evaluation jobs in browser.
- Evidence: `web/app/evaluation/page.tsx`, `internal/delivery/server/http/api_handler_evaluations.go`, `internal/delivery/server/app/evaluation_service.go`.

8. Proactive scheduler + timer manager
- User value: recurring/one-shot proactive tasks execute automatically.
- Evidence: `internal/app/scheduler/scheduler.go`, `internal/shared/timer/manager.go`, `internal/delivery/server/bootstrap/scheduler.go`, `internal/delivery/server/bootstrap/timer.go`.

9. Kernel daemon autonomous cycle
- User value: dedicated daemon runs plan/dispatch loop from kernel state.
- Evidence: `cmd/alex-server/main.go`, `internal/delivery/server/bootstrap/kernel_daemon.go`, `internal/app/agent/kernel/engine.go`.

10. Standalone eval server (RL + eval task mgmt)
- User value: separate eval service for larger offline/ops workloads.
- Evidence: `cmd/eval-server/main.go`, `internal/delivery/eval/bootstrap/server.go`, `internal/delivery/eval/http/router.go`.

### P2 Features

1. Devops CLI workflow (`alex dev`)
- User value: local up/down/logs/restart/test/lint/cleanup/config workflows.
- Evidence: `cmd/alex/dev.go`.

2. Lark scenario and inject tooling
- User value: scenario-driven regression tests and message injection for Lark channel.
- Evidence: `cmd/alex/lark_scenario_cmd.go`.

3. Reminder intent/draft/confirmation pipeline
- User value: reminder suggestions can be confirmed/modified before send.
- Evidence: `internal/app/reminder/pipeline.go`, `internal/app/reminder/confirm.go`.

4. Lark multi-bot chat coordination
- User value: prevents bot-loop collisions in multi-bot group mentions.
- Evidence: `internal/delivery/channels/lark/ai_chat_coordinator.go`.

5. Dev/internal debug endpoints and tools
- User value: inspect memory/context/logs/config safely in development mode.
- Evidence: `internal/delivery/server/http/router.go` (`/api/dev/*`, `/api/internal/*`), `web/app/dev/*`.

## Technical View

### 1) Delivery Layer

- CLI binaries
  - `alex` (interactive + command mode): `cmd/alex/main.go`, `cmd/alex/cli.go`
  - `alex-web` (web API + frontend runtime): `cmd/alex-web/main.go`
  - `alex-server` (Lark standalone + kernel subcommands): `cmd/alex-server/main.go`
  - `eval-server` (evaluation/RL API): `cmd/eval-server/main.go`

- Web routes (Next app)
  - Core: `/`, `/conversation`, `/sessions`, `/share`, `/evaluation`
  - Locale pages: `/en`, `/zh`
  - Dev pages: `/dev/*` (config, diagnostics, context-window, log tools, etc.)
  - Evidence: `web/app/**/page.tsx`.

- Main HTTP API groups
  - Streaming/share: `/api/sse`, `/api/share/sessions/{id}`, `/api/tasks/{id}/events`
  - Tasks: `/api/tasks*`
  - Sessions: `/api/sessions*`
  - Evaluations: `/api/evaluations*`
  - Agent catalog: `/api/agents*`
  - Config/onboarding/dev/internal: `/api/internal/*`, `/api/dev/*`, `/api/lark/oauth/*`
  - Health/metrics: `/health`, `/api/metrics/web-vitals`
  - Evidence: `internal/delivery/server/http/router.go`.

- Eval server API groups
  - `/api/evaluations*`, `/api/agents*`, `/api/rl/*`, `/api/eval-tasks*`, `/health`
  - Evidence: `internal/delivery/eval/http/router.go`.

### 2) Agent Runtime Layer

- Coordinator orchestrates end-to-end task execution and workflow event translation.
- Preparation service resolves runtime config/model/tool presets and builds execution environment.
- ReAct engine runs iteration loop, tool batches, checkpoints, and stop handling.
- Workflow/event envelope path supports downstream stream rendering.
- Evidence: `internal/app/agent/coordinator/coordinator.go`, `internal/app/agent/preparation/service.go`, `internal/domain/agent/react/engine.go`.

### 3) Tooling Layer

- Static built-in tools
  - UI/control: `plan`, `clarify`, `request_user`, `context_checkpoint`
  - Memory: `memory_search`, `memory_get`, `memory_related`
  - Local: `read_file`, `write_file`, `replace_in_file`, `shell_exec`, `execute_code`
  - Search: `web_search`
  - Session: `skills`
  - Lark: `channel`
  - Orchestration (registered separately): `run_tasks`, `reply_agent`
  - Evidence: `internal/app/toolregistry/registry_builtins.go`, `internal/app/toolregistry/registry.go`.

- Tool wrappers
  - validation, approval, retry, SLA metrics, circuit-breaker, fallback degradation.
  - Evidence: `internal/app/toolregistry/registry.go`, `internal/app/toolregistry/retry.go`, `internal/app/toolregistry/degradation.go`.

- Dynamic tool surface
  - MCP tools are runtime-loaded and exposed as `mcp__*` names.
  - Evidence: `internal/app/toolregistry/registry.go`, `internal/infra/mcp/`.

### 4) Lark Channel Tool Action Matrix (`channel.action`)

Enumerated actions (code-defined):
- Messaging: `send_message`, `upload_file`, `history`
- Calendar: `create_event`, `query_events`, `update_event`, `delete_event`
- Tasks: `list_tasks`, `create_task`, `update_task`, `delete_task`
- Documents: `create_doc`, `read_doc`, `read_doc_content`, `list_doc_blocks`
- Wiki: `list_wiki_spaces`, `list_wiki_nodes`, `create_wiki_node`, `get_wiki_node`
- Bitable: `list_bitable_tables`, `list_bitable_records`, `create_bitable_record`, `update_bitable_record`, `delete_bitable_record`, `list_bitable_fields`
- Drive: `list_drive_files`, `create_drive_folder`, `copy_drive_file`, `delete_drive_file`
- Sheets: `create_spreadsheet`, `get_spreadsheet`, `list_sheets`
- OKR: `list_okr_periods`, `list_user_okrs`, `batch_get_okrs`
- Contact: `get_user`, `list_users`, `get_department`, `list_departments`
- Mail: `list_mailgroups`, `get_mailgroup`, `create_mailgroup`
- VC: `list_meetings`, `get_meeting`, `list_rooms`

Evidence: `internal/infra/tools/builtin/larktools/channel.go`.

### 5) Memory and Context Layer

- Markdown memory engine (`AppendDaily`, `Search`, `Related`, `GetLines`, long-term/daily loaders).
- Optional indexer/index store for hybrid retrieval.
- Context manager injects memory snapshots and system/history windows.
- Evidence: `internal/infra/memory/engine.go`, `internal/infra/memory/md_store.go`, `internal/infra/memory/indexer.go`, `internal/app/context/manager_memory.go`.

### 6) LLM + Observability Layer

- Supported providers (factory switch):
  - `openai`, `openrouter`, `deepseek`, `kimi`, `glm`, `minimax`
  - `openai-responses` / `responses` / `codex`
  - `anthropic` / `claude`
  - `llama.cpp` / `llama-cpp` / `llamacpp`
  - `mock`
- Resilience: retries, user/global rate limits, tool-call parsing wrappers, health registry.
- Observability: instrumented LLM/tool metrics, cost estimation, tracing.
- Evidence: `internal/infra/llm/factory.go`, `internal/infra/observability/instrumentation.go`, `internal/infra/observability/metrics.go`.

### 7) Proactive/Kernel Layer

- Scheduler (cron triggers + OKR sync + calendar + heartbeat + persistence).
- Timer manager (one-shot/recurring timers, persistence, execution callbacks).
- Kernel engine (cycle planning, dispatch enqueue, execution, runtime state persistence).
- Hooks (OKR context injection, memory capture on completion).
- Evidence: `internal/app/scheduler/scheduler.go`, `internal/shared/timer/manager.go`, `internal/app/agent/kernel/engine.go`, `internal/app/agent/hooks/*.go`.

### 8) External Execution Layer

- External agent types currently wired: `claude_code`, `codex`, `kimi`.
- Bridge executor supports interactive permission relay (Claude path), detached mode, resume, execution policy mapping.
- Coding gateway abstracts adapter routing and status/cancel.
- Evidence: `internal/infra/external/registry.go`, `internal/infra/external/bridge/executor.go`, `internal/infra/coding/gateway.go`.

## Exhaustive Appendices (Code Enumeration)

### Appendix A: Command Surface (Complete from Code)

#### A.1 `alex` standalone entry path (`cmd/alex/main.go`)

- `help`, `-h`, `--help`
- `version`, `-v`, `--version`
- `config ...` (standalone path)
- `dev ...`
- `lark ...`

#### A.2 `alex` regular CLI dispatch (`cmd/alex/cli.go`)

- `<task>` (default: stream task execution)
- `resume <session-id>`
- `sessions` / `session`
- `config`
- `cost` / `costs`
- `model` / `models`
- `setup`
- `llama-cpp` / `llamacpp`
- `mcp`
- `eval` / `evaluation`
- `acp`
- `mcp-permission-server`
- `help`, `-h`, `--help`
- `version`, `-v`, `--version`

#### A.3 Sessions subcommands (`cmd/alex/cli.go`)

- `alex sessions` / `alex sessions list`
- `alex sessions cleanup|clean|prune`
- `alex sessions pull <session-id>`

#### A.4 Cost subcommands (`cmd/alex/cost.go`)

- `alex cost show|summary`
- `alex cost session <session-id>`
- `alex cost day|daily [YYYY-MM-DD]`
- `alex cost month|monthly [YYYY-MM]`
- `alex cost export [--format csv|json] [--session ...] [--model ...] [--provider ...] [--start ...] [--end ...] [--output ...]`

#### A.5 Model subcommands (`cmd/alex/cli_model.go`)

- `alex model` / `alex model list|ls`
- `alex model use|select|set <provider/model>` (or interactive picker when no explicit model)
- `alex model clear|reset`
- `alex model help`

#### A.6 MCP subcommands (`cmd/alex/mcp.go`)

- `alex mcp list|ls`
- `alex mcp add <name> <command> [args...]`
- `alex mcp remove|rm <name>`
- `alex mcp tools [server]`
- `alex mcp restart <name>`
- `alex mcp help`

#### A.7 ACP subcommands (`cmd/alex/acp.go`)

- `alex acp [--initial-message ...]` (stdio)
- `alex acp serve [--host HOST] [--port PORT] [--initial-message ...]`

#### A.8 Lark scenario/inject commands (`cmd/alex/lark_scenario_cmd.go`)

- `alex lark scenario run [--mode http|mock] [--dir path] [--json-out file] [--md-out file] [--name ...] [--fail-fast] [--port ...] [--base-url ...] [--timeout ...] [--tag ...]`
- `alex lark inject ...`

#### A.9 Other binaries

- `alex-server` default: run Lark standalone gateway (`cmd/alex-server/main.go`)
- `alex-server kernel-daemon`
- `alex-server kernel-once`
- `eval-server --config <path>` (`cmd/eval-server/main.go`)

### Appendix B: HTTP Route Inventory (Complete from Code)

#### B.1 Main server routes (`internal/delivery/server/http/router.go`)

- stream/share
  - `GET /api/sse`
  - `GET /api/share/sessions/{session_id}`
- tasks
  - `POST /api/tasks`
  - `GET /api/tasks`
  - `GET /api/tasks/active`
  - `GET /api/tasks/stats`
  - `GET /api/tasks/{task_id}`
  - `GET /api/tasks/{task_id}/events`
  - `POST /api/tasks/{task_id}/cancel`
- evaluations
  - `GET /api/evaluations`
  - `POST /api/evaluations`
  - `GET /api/evaluations/{evaluation_id}`
  - `DELETE /api/evaluations/{evaluation_id}`
- agents
  - `GET /api/agents`
  - `GET /api/agents/{agent_id}`
  - `GET /api/agents/{agent_id}/evaluations`
- sessions
  - `GET /api/sessions`
  - `POST /api/sessions`
  - `GET /api/sessions/{session_id}`
  - `DELETE /api/sessions/{session_id}`
  - `GET /api/sessions/{session_id}/persona`
  - `PUT /api/sessions/{session_id}/persona`
  - `GET /api/sessions/{session_id}/snapshots`
  - `GET /api/sessions/{session_id}/turns/{turn_id}`
  - `POST /api/sessions/{session_id}/replay`
  - `POST /api/sessions/{session_id}/share`
  - `POST /api/sessions/{session_id}/fork`
- dev
  - `GET /api/dev/sessions/{session_id}/context-window`
  - `GET /api/dev/logs`
  - `GET /api/dev/logs/structured`
  - `GET /api/dev/logs/index`
  - `GET /api/dev/memory`
  - `GET /api/dev/context-config`
  - `PUT /api/dev/context-config`
  - `GET /api/dev/context-config/preview`
- internal
  - `GET /api/internal/sessions/{session_id}/context`
  - `GET /api/internal/config/runtime`
  - `PUT /api/internal/config/runtime`
  - `GET /api/internal/config/runtime/stream`
  - `GET /api/internal/config/runtime/models`
  - `GET /api/internal/subscription/catalog`
  - `GET /api/internal/onboarding/state`
  - `PUT /api/internal/onboarding/state`
  - `GET /api/internal/config/apps`
  - `PUT /api/internal/config/apps`
- oauth/hooks/health/metrics
  - `GET /api/lark/oauth/start`
  - `GET /api/lark/oauth/callback`
  - `POST /api/hooks/claude-code`
  - `POST /api/metrics/web-vitals`
  - `GET /health`

#### B.2 Eval server routes (`internal/delivery/eval/http/router.go`)

- eval/agents
  - `GET /api/evaluations`
  - `POST /api/evaluations`
  - `GET /api/evaluations/{evaluation_id}`
  - `DELETE /api/evaluations/{evaluation_id}`
  - `GET /api/agents`
  - `GET /api/agents/{agent_id}`
  - `GET /api/agents/{agent_id}/evaluations`
- RL
  - `GET /api/rl/stats`
  - `GET /api/rl/trajectories`
  - `GET /api/rl/trajectories/{trajectory_id}`
  - `GET /api/rl/config`
  - `PUT /api/rl/config`
  - `GET /api/rl/export`
- eval task management
  - `GET /api/eval-tasks`
  - `POST /api/eval-tasks`
  - `GET /api/eval-tasks/{task_id}`
  - `PUT /api/eval-tasks/{task_id}`
  - `DELETE /api/eval-tasks/{task_id}`
  - `POST /api/eval-tasks/{task_id}/run`
- health
  - `GET /health`

### Appendix C: Tool Inventory (Complete from Code)

#### C.1 Static built-in tools (`internal/app/toolregistry/registry_builtins.go`)

- `plan`
- `clarify`
- `memory_search`
- `memory_get`
- `memory_related`
- `request_user`
- `context_checkpoint`
- `web_search`
- `skills`
- `read_file`
- `write_file`
- `replace_in_file`
- `shell_exec`
- `execute_code`
- `channel`

#### C.2 Orchestration tools (`internal/app/toolregistry/registry.go`, `RegisterOrchestration`)

- `run_tasks`
- `reply_agent`

#### C.3 `channel.action` enum (`internal/infra/tools/builtin/larktools/channel.go`)

- `send_message`
- `upload_file`
- `history`
- `create_event`
- `query_events`
- `update_event`
- `delete_event`
- `list_tasks`
- `create_task`
- `update_task`
- `delete_task`
- `create_doc`
- `read_doc`
- `read_doc_content`
- `list_doc_blocks`
- `list_wiki_spaces`
- `list_wiki_nodes`
- `create_wiki_node`
- `get_wiki_node`
- `list_bitable_tables`
- `list_bitable_records`
- `create_bitable_record`
- `update_bitable_record`
- `delete_bitable_record`
- `list_bitable_fields`
- `list_drive_files`
- `create_drive_folder`
- `copy_drive_file`
- `delete_drive_file`
- `create_spreadsheet`
- `get_spreadsheet`
- `list_sheets`
- `list_okr_periods`
- `list_user_okrs`
- `batch_get_okrs`
- `get_user`
- `list_users`
- `get_department`
- `list_departments`
- `list_mailgroups`
- `get_mailgroup`
- `create_mailgroup`
- `list_meetings`
- `get_meeting`
- `list_rooms`

#### C.4 `actionSafetyLevel` mapping (`internal/infra/tools/builtin/larktools/channel.go`)

- ReadOnly
  - `history`
  - `query_events`
  - `list_tasks`
  - `read_doc`
  - `read_doc_content`
  - `list_doc_blocks`
  - `list_wiki_spaces`
  - `list_wiki_nodes`
  - `get_wiki_node`
  - `list_bitable_tables`
  - `list_bitable_records`
  - `list_bitable_fields`
  - `list_drive_files`
  - `get_spreadsheet`
  - `list_sheets`
  - `list_okr_periods`
  - `list_user_okrs`
  - `batch_get_okrs`
  - `get_user`
  - `list_users`
  - `get_department`
  - `list_departments`
  - `list_mailgroups`
  - `get_mailgroup`
  - `list_meetings`
  - `get_meeting`
  - `list_rooms`
- Reversible
  - `send_message`
  - `upload_file`
- HighImpact
  - `create_event`
  - `update_event`
  - `create_task`
  - `update_task`
  - `create_doc`
  - `create_wiki_node`
  - `create_bitable_record`
  - `update_bitable_record`
  - `create_drive_folder`
  - `copy_drive_file`
  - `create_spreadsheet`
  - `create_mailgroup`
- Irreversible
  - `delete_event`
  - `delete_task`
  - `delete_bitable_record`
  - `delete_drive_file`

## Completeness Notes

1. This inventory is complete for statically declared surfaces in code paths above.
2. Runtime dynamic MCP tool lists are intentionally not frozen here; they depend on live MCP server configuration (`.mcp.json` and runtime process state).
3. If MCP server config changes, only the dynamic-tool subsection needs refresh.
