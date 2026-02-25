# Domain Layers and ID Semantics

Updated: 2026-02-10

## Scope
Defines the current layer boundaries and correlation ID semantics used across CLI, Lark, server, and web event delivery.

## Layering
- **Domain**: `internal/domain/*`
  - Agent runtime and domain events: `internal/domain/agent/*`
  - Workflow model/state machine: `internal/domain/workflow/*`
- **Application**: `internal/app/*`
  - Agent coordination: `internal/app/agent/*`
  - Context/session orchestration: `internal/app/context/*`
  - Tool orchestration policy/registry: `internal/app/toolregistry/*`
- **Delivery**: `internal/delivery/*`
  - HTTP/SSE surfaces: `internal/delivery/server/*`
  - Lark channel adapter: `internal/delivery/channels/lark/*`
  - CLI output: `internal/delivery/output/*`
- **Infrastructure**: `internal/infra/*`
  - LLM, memory, MCP, session, tools, external executors, storage.
- **Shared**: `internal/shared/*`
  - Config, logging, parser, utilities.

## Core Rule
IDs are created/ensured at entry or orchestration boundaries (delivery/app) and propagated through domain/runtime. Domain code may carry IDs but should avoid introducing ad-hoc, delivery-specific correlation schemes.

## ID Taxonomy
- `session_id`: stable conversation scope.
- `task_id`: single agent execution under a session.
- `parent_task_id`: parent execution for delegated/subagent runs.
- `run_id`: workflow run identifier for event streams.
- `parent_run_id`: parent run for nested/subagent workflow events.
- `log_id`: log correlation key across service, LLM, latency, and request payload logs.
- `request_id`: vendor-facing request key (typically embeds `log_id` for LLM calls).

## Propagation Guidance
1. Delivery entrypoints (CLI/server/Lark) ensure base IDs exist before execution.
2. App coordinator propagates IDs into context and workflow envelopes.
3. Domain emits typed events with run/task correlation fields.
4. Delivery adapters serialize IDs unchanged to SSE/Lark/CLI streams.
5. Logs must include `log_id` whenever available.

## Practical Debugging Order
1. Locate `log_id` in `alex-service.log`.
2. Join with `alex-llm.log` and `logs/requests/llm.jsonl` via `request_id`/`log_id`.
3. Use `task_id` + `parent_task_id` to trace delegation tree.
4. Use `run_id` + `parent_run_id` to reconstruct event sequence in web/Lark timelines.

## Legacy Path Mapping
- `internal/agent/domain` -> `internal/domain/agent`
- `internal/agent/app` -> `internal/app/agent`
- `internal/agent/ports` -> `internal/domain/agent/ports`
- `internal/server/http` -> `internal/delivery/server/http`
- `internal/server/app` -> `internal/delivery/server/app`
- `internal/llm` -> `internal/infra/llm`
- `internal/utils/id` -> `internal/shared/utils/id`
