# 2026-02-26 Lark Restart Storm Hardening

## Context
- User reported repeated `restart storm` incidents in Lark supervisor health.
- Logs show frequent `upgrade restart failed ... signal: terminated`, `restart failed ... exit status 143`, and `context canceled` during component restart.
- Existing `dev lark` command routing was also enhanced in the same workstream to support component-targeted restart commands.

## Hypothesis
1. Lark component processes are launched in the caller process group.
2. Stop logic sends signals to process groups by PGID.
3. During restart, stopping `main` can terminate the restart script/supervisor itself, causing restart churn and storms.

## Plan
- [completed] Harden process signaling to avoid same-group collateral termination.
- [completed] Launch Lark component processes in isolated process groups/sessions.
- [completed] Validate restart behavior with repeated restart commands and targeted tests.
- [completed] Update plan with final verification results and residual risks.

## Changes Applied
- `scripts/lark/component.sh`
  - Reworked process launch to use explicit env/arg arrays (removed `eval`-based launcher).
  - Launches `alex-server` via `python3` session isolation (`os.setsid()`) when available, so component PGID is no longer shared with caller/supervisor.
- `scripts/lib/common/process.sh`
  - Added same-PGID guard in `_signal_process`: when target PGID equals caller PGID, signal only the target PID to prevent collateral caller termination.
- `cmd/alex/dev_lark.go` + `cmd/alex/dev_lark_command_parse_test.go`
  - Added component-targeted command parsing (`lark main restart`, `lark restart main`, etc.) and tests.

## Verification
- Lint/Test gates:
  - `./dev.sh lint`
  - `./dev.sh test`
- Runtime checks:
  - `./dev.sh lark up` starts supervisor/components healthy.
  - Repeated `./dev.sh lark main restart` returns `0` and keeps supervisor alive.
  - Forced SHA drift by writing stale `lark-main.sha` triggers supervisor upgrade path and logs:
    - `upgrading component for SHA drift ...`
    - `upgrade restart succeeded`
- Regression signal check:
  - No new `context canceled`/`signal: terminated` entries were emitted during post-fix restart validation window.

## Residual Risk
- Session isolation uses `python3` availability for `setsid`; fallback path still runs without explicit session isolation. On this environment `python3` is present and validated.
