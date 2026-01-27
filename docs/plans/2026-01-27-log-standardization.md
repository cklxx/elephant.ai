# Plan: Log standardization and log_id coverage (2026-01-27)

## Goal
- Ensure all log streams include `log_id` when available (service/LLM/latency/CLI).
- Standardize log key naming (`log_id`) and document logging conventions.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Standardize log_id key in file logs and wrappers.
2. Add log_id to latency and CLI latency logs via context-aware helpers.
3. Ensure LLM logs include log_id by default.
4. Write logging conventions doc and update log files reference.
5. Run full lint + tests.
6. Commit changes.

## Progress
- 2026-01-27: Plan created; engineering practices reviewed.
- 2026-01-27: Standardized log_id key usage, added context-aware CLI latency logging, and updated logging conventions + log files docs (including subagent log_id derivation).
- 2026-01-27: Ran `./dev.sh test` (pass; LC_DYSYMTAB linker warnings emitted). `./dev.sh lint` failed due to pre-existing `internal/channels/wechat/gateway.go` errcheck.
