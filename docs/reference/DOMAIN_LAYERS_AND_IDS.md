# Domain Layers and ID Semantics

Updated: 2026-03-10

## Core Rule

IDs are created at entry/orchestration boundaries (delivery/app) and propagated through domain/runtime. Domain code carries IDs but must not introduce ad-hoc correlation schemes.

For layer boundaries: see [ARCHITECTURE.md](ARCHITECTURE.md) §2.

## ID Taxonomy

| ID | Scope |
|----|-------|
| `session_id` | Stable conversation scope |
| `task_id` | Single agent execution under a session |
| `parent_task_id` | Parent execution for delegated/subagent runs |
| `run_id` | Workflow run identifier for event streams |
| `parent_run_id` | Parent run for nested workflow events |
| `log_id` | Log correlation across service, LLM, latency, request logs |
| `request_id` | Vendor-facing key (typically embeds `log_id`) |

## Propagation

1. Delivery entrypoints (CLI/server/Lark) ensure base IDs exist.
2. App coordinator propagates IDs into context and workflow envelopes.
3. Domain emits typed events with run/task correlation fields.
4. Delivery adapters serialize IDs unchanged to SSE/Lark/CLI streams.
5. Logs must include `log_id` whenever available.

## Debugging Order

1. Locate `log_id` in `alex-service.log`.
2. Join with `alex-llm.log` and `logs/requests/llm.jsonl` via `request_id`/`log_id`.
3. `task_id` + `parent_task_id` → delegation tree.
4. `run_id` + `parent_run_id` → event sequence in web/Lark timelines.
