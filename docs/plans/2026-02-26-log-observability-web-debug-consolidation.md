# 2026-02-26 Log Observability + Web Debug Consolidation

## Goal
- Make LLM failure chains directly traceable by `request_id/log_id`.
- Consolidate web debug logging surface while preserving existing debugging capability.

## Scope
- Backend: request log failure entries, structured parsing, log index error metadata.
- Web: structured workbench error-first visibility, remove duplicated log-analyzer implementation.
- CLI: dev summary link alignment.

## Progress
- [x] Pre-work checklist on `main` completed.
- [x] Added `entry_type=error` request-log write path with normalized failure metadata.
- [x] Ensured retry-layer summary uses non-empty request IDs (fallback generation).
- [x] Extended structured log parser/types for error fields.
- [x] Added structured `errors` snippet in log bundle response.
- [x] Extended log index with `error_count`, `last_error_class`, `last_error_at`.
- [x] Updated workbench UI with error filters and dedicated error tab.
- [x] Consolidated `/dev/log-analyzer` route into diagnostics anchor redirect.
- [x] Updated dev tool link to diagnostics anchor.
- [x] Run targeted tests for touched packages.
- [x] Run full lint/tests + mandatory code review.
- [x] Add good-experience entry + summary; backfill memory graph.

## Notes
- Keep `/api/dev/logs` text endpoint for compatibility; structured endpoint remains the primary debug surface.
