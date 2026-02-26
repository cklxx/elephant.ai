# 2026-02-26 — Error-first LLM log observability and diagnostics consolidation

Impact: LLM failure chains are now directly traceable from index -> structured bundle -> request/error payload, reducing mean-time-to-diagnosis for retry/timeout/provider failures.

## What changed

- Added `entry_type=error` request-log writes with normalized metadata (`mode/provider/model/intent/stage/error_class/error/latency_ms`) in `internal/shared/utils/request_log.go`.
- Ensured retry-layer summary logging always has a non-empty `request_id` via context/log-id fallback and persisted failure details back to request log in `internal/infra/llm/retry_client.go`.
- Extended structured parsing and fetch flow to expose a dedicated `errors` snippet alongside `requests` in `internal/shared/logging/log_structured.go` and `internal/shared/logging/log_fetch.go`.
- Extended log index aggregation with `error_count`, `last_error_class`, `last_error_at` in `internal/shared/logging/log_index.go`.
- Consolidated web debug surface:
  - `/dev/log-analyzer` now redirects to diagnostics anchor.
  - Structured workbench adds error filter, error tab, and error metadata chips.
  - Dev summary link points to diagnostics anchor.

## Why this worked

- Reused existing `request_id/log_id` chain instead of introducing new correlation IDs.
- Kept old request/response log paths intact and layered error entries on top, so compatibility stayed stable.
- Separated error browsing from mixed request/response streams in UI, reducing noise during incident triage.

## Validation

- `go test ./internal/shared/utils ./internal/shared/logging ./internal/infra/llm`
- `./scripts/pre-push.sh`
- `python3 skills/code-review/run.py '{"action":"review"}'` + manual P0/P1 review

## Metadata
- id: good-2026-02-26-error-first-llm-log-observability
- tags: [good, observability, llm, diagnostics, debugability]
- links:
  - docs/plans/2026-02-26-log-observability-web-debug-consolidation.md
  - internal/shared/utils/request_log.go
  - internal/infra/llm/retry_client.go
  - internal/shared/logging/log_fetch.go
  - internal/shared/logging/log_index.go
  - web/components/dev-tools/StructuredLogWorkbench.tsx
