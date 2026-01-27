# Log files and writers

This document lists all file-based logs produced by the system, where they are
written, and how to correlate entries with `log_id`.

## Base directories
- `ALEX_LOG_DIR`: base directory for service, LLM, and latency logs.
  - Default: `$HOME` (resolved at runtime).
  - `dev.sh` sets this to `./logs` for local runs.
- `ALEX_REQUEST_LOG_DIR`: base directory for request payload logs.
  - Default: `${PWD}/logs/requests`.

In deploy mode (`ALEX_SERVER_MODE=deploy`), service/LLM/latency log lines are
also mirrored to stdout.

CLI runs share the same log files and generate a fresh `log_id` per invocation
via `cliBaseContext()`.

## File logs (system categories)

### `alex-service.log`
- **Category**: `SERVICE`
- **Writer**: `internal/utils.Logger` via `logging.NewComponentLogger(...)`.
- **Content**: general server/application logs (routing, orchestration, errors).
- **Format**:
  ```
  YYYY-MM-DD HH:MM:SS [LEVEL] [SERVICE] [Component] [log_id=<log_id>] file.go:line - message
  ```

### `alex-llm.log`
- **Category**: `LLM`
- **Writer**: LLM clients (`internal/llm/*_client.go`).
- **Content**: request metadata, headers (redacted), response summaries, errors.
- **Correlation**:
  - LLM logs include `[log_id=<id>]` in the prefix when available.
  - Each LLM call uses a `request_id` prefix like `[req:<id>]`.
  - `request_id` embeds `log_id` when available (`<log_id>:llm-...`), so `log_id`
    substring search works.

### `alex-latency.log`
- **Category**: `LATENCY`
- **Writer**: HTTP observability middleware (`internal/server/http`).
- **Content**: per-request latency: `route=... method=... status=... latency_ms=... bytes=...`.
- **Correlation**: includes `log_id` when the request context carries one.

## Request payload logs

### `logs/requests/streaming.log`
- **Writer**: `internal/utils/request_log.go`
- **Content**: raw streaming request/response/summary payloads for LLM calls.
- **Format**:
  ```
  2026-01-27T12:34:56.123Z [req:<request_id>] [request|response|summary] body_bytes=123
  <payload>
  ```
- **Correlation**: `request_id` embeds `log_id` when present, so `log_id` search
  returns the relevant request/response blocks.

## Dev helper logs (process stdout/stderr)

### `logs/server.log`, `logs/web.log`, `logs/acp.log`
- **Writer**: `./dev.sh` process wrappers.
- **Content**: raw stdout/stderr from server, web, and ACP processes.
- **Note**: not structured; use for local debugging only.

## Correlation guidance
- Primary key: `log_id`.
- Service logs include `log_id=<log_id>`.
- LLM/request logs include `request_id` which embeds `log_id`.
- Latency log includes `log_id` when present on the request context.
