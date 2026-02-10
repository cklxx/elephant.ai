# Log Files and Writers

Updated: 2026-02-10

This document lists runtime log files, default locations, and correlation strategy.

## 1) Base directories

- `ALEX_LOG_DIR`
  - Purpose: service/LLM/latency logs.
  - Default: `$HOME`.
- `ALEX_REQUEST_LOG_DIR`
  - Purpose: request payload logs (`llm.jsonl`).
  - Default: `${PWD}/logs/requests`.

## 2) Structured runtime logs

### `alex-service.log`
- Category: `SERVICE`.
- Writer: `internal/shared/utils/logger.go` via `internal/shared/logging.NewComponentLogger`.
- Content: bootstrap, coordinator, routing, tool/runtime errors.

### `alex-llm.log`
- Category: `LLM`.
- Writer: LLM clients in `internal/infra/llm/*`.
- Content: request metadata, provider response summaries, retry/failure traces.

### `alex-latency.log`
- Category: `LATENCY`.
- Writer: server middleware in `internal/delivery/server/http`.
- Content: per-request latency, route, status, payload-size stats.

## 3) Request payload logs

### `${ALEX_REQUEST_LOG_DIR}/llm.jsonl`
- Writer: `internal/shared/utils/request_log.go`.
- Content: serialized streaming request/response payload entries.
- Format: JSONL with `request_id`, optional `log_id`, and payload body.

## 4) Dev process logs (`alex dev`)

`alex dev` process manager writes per-service stdout/stderr files under configured `log_dir` (default `logs/`), e.g.:
- `logs/server.log`
- `logs/web.log`
- `logs/sandbox.log`
- `logs/lark-supervisor.log`

These are process-level logs, complementary to structured runtime category logs above.

## 5) Correlation keys

Primary key order:
1. `log_id` (service-wide correlation)
2. `request_id` (LLM/vendor call correlation; often embeds `log_id`)
3. `task_id` / `parent_task_id` (execution tree)
4. `run_id` / `parent_run_id` (workflow event lineage)

## 6) Retrieval helpers

For API-side retrieval and filtering logic, see:
- `internal/shared/logging/log_fetch.go`
- `internal/shared/logging/log_structured.go`
