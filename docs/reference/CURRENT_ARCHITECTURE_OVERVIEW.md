# Current Architecture Overview (Systematic)

Updated: 2026-02-16

## Scope

This document summarizes the **current runtime architecture** of elephant.ai based on the active code paths.
It is a practical map for implementation/debugging, not a historical design proposal.

## 1. Runtime Surfaces and Binaries

- CLI/TUI entry: `cmd/alex/main.go`
- Web/API/SSE service: `cmd/alex-web/main.go`
- Lark standalone gateway: `cmd/alex-server/main.go`
  - Supports `kernel-once` mode for single kernel cycle execution.

Current split:
- `alex-web`: full HTTP API + SSE + web-facing backend.
- `alex-server`: Lark-first runtime + debug HTTP server.

## 2. Layer Model (Code-Mapped)

- Delivery layer: `internal/delivery/*`
  - Lark gateway, HTTP router/SSE, CLI output adapters.
- Application layer: `internal/app/*`
  - Coordinator/orchestration, context building, tool registry, DI wiring.
- Domain layer: `internal/domain/*`
  - ReAct runtime, workflow model, domain events, ports.
- Infrastructure layer: `internal/infra/*`
  - LLM clients/factory, tools, memory engine, MCP, storage, observability.
- Shared layer: `internal/shared/*`
  - Config, logging, IDs, utils.

Reference docs:
- `docs/reference/ARCHITECTURE_AGENT_FLOW.md`
- `docs/reference/DOMAIN_LAYERS_AND_IDS.md`

## 3. Bootstrap and Dependency Wiring

Primary boot flow is managed by:
- `internal/delivery/server/bootstrap/foundation.go`
- `internal/app/di/container_builder.go`

Typical sequence:
1. Load runtime config (`internal/shared/config`).
2. Initialize observability.
3. Build DI container (`internal/app/di`).
4. Build coordinator/tool registry/session/memory/checkpoint dependencies.
5. Start optional subsystems (MCP, scheduler, timer, kernel depending on mode/config).

## 4. Core Agent Execution Chain

Main entry point:
- `internal/app/agent/coordinator/coordinator.go` (`ExecuteTask`)

Execution phases:
1. **Prepare**
   - `internal/app/agent/preparation/service.go`
   - Session loading/history replay/context window/system prompt/tool preset/model resolution.
2. **Execute (ReAct loop)**
   - `internal/domain/agent/react/engine.go`
   - `internal/domain/agent/react/runtime.go`
   - Think -> plan tools -> execute tools -> observe -> checkpoint.
3. **Summarize + Persist**
   - Session/history persistence + workflow snapshot + cost logging.

## 5. Event Model and Propagation

Domain events:
- `internal/domain/agent/events.go`

Application translation to workflow envelope:
- `internal/app/agent/coordinator/workflow_event_translator.go`

Downstream delivery:
- Lark listeners (`internal/delivery/channels/lark/*`)
- SSE broadcaster (`internal/delivery/server/app/event_broadcaster.go`)
- CLI output renderer (`internal/delivery/output/*`)

## 6. Tool Architecture

Registry and wrapping:
- `internal/app/toolregistry/registry.go`
- `internal/app/toolregistry/registry_builtins.go`

Execution wrappers (outer to inner):
- SLA measurement (optional)
- ID propagation
- Retry/circuit breaker
- Approval executor
- Argument validation
- Concrete tool executor

Builtins live in:
- `internal/infra/tools/builtin/*`

Subagent/delegation tools are dynamically registered after coordinator creation:
- `subagent`, `explore`, `bg_*`, `ext_*`

## 7. Context and Memory

Context window/system prompt assembly:
- `internal/app/context/manager_window.go`
- `internal/app/context/manager_prompt.go`

Compression and budget control:
- `internal/app/context/manager_compress.go`

Memory engine (Markdown-first):
- `internal/infra/memory/md_store.go`
- `internal/infra/memory/engine.go`

Optional indexing path:
- `internal/infra/memory/indexer.go`
- `internal/infra/memory/index_store.go`

## 8. Session/State Persistence

Session store:
- `internal/infra/session/filestore/store.go`

State snapshots / turn history:
- `internal/infra/session/state_store/file_store.go`

ReAct checkpoint store:
- `internal/domain/agent/react/checkpoint.go`

Cost storage:
- `internal/infra/storage/cost_store.go`

## 9. Delivery Channels

### 9.1 Web / HTTP / SSE

- Router: `internal/delivery/server/http/router.go`
- Async task execution service: `internal/delivery/server/app/task_execution_service.go`
- Event broadcaster: `internal/delivery/server/app/event_broadcaster.go`
- SSE handler: `internal/delivery/server/http/sse_handler.go`

Frontend SSE consumption and event pipeline:
- `web/hooks/useSSE/useSSE.ts`
- `web/hooks/useSSE/useSSEConnection.ts`
- `web/lib/events/eventPipeline.ts`
- Event types: `web/lib/types/events/*`

### 9.2 Lark

- Gateway: `internal/delivery/channels/lark/gateway.go`
- Bootstrap wiring: `internal/delivery/server/bootstrap/lark_gateway.go`
- Progress listeners:
  - `progress_listener.go`
  - `background_progress_listener.go`
  - `plan_clarify_listener.go`

## 10. Proactivity Subsystems

Scheduler:
- `internal/app/scheduler/scheduler.go`
- Trigger execution: `internal/app/scheduler/executor.go`
- Notification adapters: `internal/app/scheduler/notifier.go`

Kernel periodic loop:
- Engine: `internal/app/agent/kernel/engine.go`
- Bootstrap stage: `internal/delivery/server/bootstrap/kernel.go`
- Single cycle command: `internal/delivery/server/bootstrap/kernel_once.go`

## 11. IDs and Correlation (Operational)

Key IDs carried end-to-end:
- `session_id`
- `run_id` / `parent_run_id`
- `task_id` / `parent_task_id` (delivery/task-store semantics)
- `log_id`
- `correlation_id` / `causation_id`

Primary references:
- `docs/reference/DOMAIN_LAYERS_AND_IDS.md`
- `internal/shared/utils/id/*`

## 12. Architecture Guardrails (Current)

- Keep domain ports independent from concrete memory/RAG infra.
- Keep policy enforcement in app/infra layers.
- Keep event correlation fields unchanged across translation and delivery.
- Keep config canonical in YAML-based runtime config pipeline.

This aligns with the current guardrails documented in:
- `docs/reference/ARCHITECTURE_AGENT_FLOW.md`
- `docs/reference/DOMAIN_LAYERS_AND_IDS.md`
- `docs/guides/engineering-practices.md`
