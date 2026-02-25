# 2026-02-24 — Scan scripts/ for reusable helper coverage

## Context
- `scripts/lib/common/logging.sh` already exposes colored logging + `die`/`command_exists` helpers.
- Need to ensure other scripts stop reimplementing the same blocks and reuse shared libs.

## Plan
1. Inventory existing `scripts/lib/common` helpers for logging/error handling and recognize their exported functions.
2. Walk through `scripts/` (excluding `web/`) to find scripts that still redefine logging/error utilities or recreate argument parsing patterns that could rely on shared helpers/dispatchers.
3. Collect concrete candidate files and the repeated functions/blocks they contain so we can centralize them (logging, error, argument parsing).
4. Summarize findings so they can be acted upon later (refactors or documentation updates).

## Findings
- `scripts/install.sh` duplicates the entire logging stack (`log_info`, `log_success`, `log_warning`, `log_error`, `supports_color`) plus a local `command_exists`. The shared `scripts/lib/common/logging.sh` already provides those utilities with the same semantics.
- `scripts/setup-docker-mirrors.sh` redefines the same four logging helpers and `die`; it, too, could just source `scripts/lib/common/logging.sh` and use that single `die`.
