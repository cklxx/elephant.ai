# Log Files

Updated: 2026-03-10

## Base Directories

| Env var | Purpose | Default |
|---------|---------|---------|
| `ALEX_LOG_DIR` | Service/LLM/latency logs | `$HOME` |
| `ALEX_REQUEST_LOG_DIR` | Request payload logs | `${PWD}/logs/requests` |

## Runtime Logs

| File | Category | Writer | Content |
|------|----------|--------|---------|
| `alex-service.log` | SERVICE | `internal/shared/logging.NewComponentLogger` | Bootstrap, routing, tool/runtime errors |
| `alex-llm.log` | LLM | `internal/infra/llm/*` | Request metadata, provider responses, retries |
| `alex-latency.log` | LATENCY | `internal/delivery/server/http` | Per-request latency, route, status |
| `llm.jsonl` | — | `internal/shared/utils/request_log.go` | Streaming request/response payloads (JSONL) |

## Dev Process Logs

`alex dev` writes per-service stdout/stderr under `logs/`: `server.log`, `web.log`, `sandbox.log`, `lark-supervisor.log`.

## Correlation Keys

1. `log_id` — service-wide correlation
2. `request_id` — LLM/vendor call (often embeds `log_id`)
3. `task_id` / `parent_task_id` — execution tree
4. `run_id` / `parent_run_id` — workflow event lineage

## Retrieval

- `internal/shared/logging/log_fetch.go`
- `internal/shared/logging/log_structured.go`
