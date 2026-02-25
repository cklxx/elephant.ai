# Plan: Lark DevOps Process Unification (No Unexpected Auto-Fix)

## Status: Completed

## Goal
- Remove unexpected auto-fix triggers during local `./dev.sh lark` usage.
- Keep a single global process-management model with shared PID dir under public config dir.
- Keep strict main/test config isolation and surface config file paths in status output.
- Make command/status output intuitive during validation cycles.

## Batch 1: Re-review + root-cause confirmation ✅
- [x] Trace `dev.sh -> lark.sh -> supervisor.sh -> loop.sh` control flow.
- [x] Confirm trigger points for supervisor autofix vs loop gate autofix.
- [x] Confirm why `test` may be down/behind during validation.

## Batch 2: Behavior fixes (no surprise mutation)
- [x] Add explicit auto-fix switches (loop + supervisor both default off) so codex repair is opt-in.
- [x] Keep status/report explicit when validation phase intentionally suppresses test restart.
- [x] Avoid misleading "will auto-upgrade" text for test during active validation.

## Batch 3: Command UX cleanup
- [x] Keep canonical lark commands in help output; legacy aliases remain supported but de-emphasized.
- [x] Add status hints for current process mode and auto-fix policy.

## Batch 4: Validation + review
- [x] Run bash syntax checks and targeted lark script smoke tests.
- [x] Run `./dev.sh lint` and `./dev.sh test` if feasible.
- [x] Perform mandatory code review workflow report (P0-P3), then commit incremental changes.

## Validation Notes
- `bash -n dev.sh lark.sh scripts/lark/loop.sh scripts/lark/supervisor.sh tests/scripts/lark-supervisor-smoke.sh tests/scripts/lark-loop-autofix-toggle.sh` ✅
- `./tests/scripts/lark-loop-autofix-toggle.sh` ✅
- `./tests/scripts/lark-loop-lock-release.sh` ✅
- `./tests/scripts/lark-supervisor-smoke.sh` ✅
- `./tests/scripts/lark-autofix-smoke.sh` ✅
- `./dev.sh lint` ✅
- `./dev.sh test` ❌ (pre-existing unrelated failures):
  - `cmd/alex`: `TestExecuteConfigCommandValidateQuickstartAllowsMissingLLMKey`
  - `cmd/alex`: `TestExecuteConfigCommandValidateProductionFailsWithoutLLMKey`
  - `internal/delivery/server/bootstrap`: `TestLoadConfig_ProductionProfileRequiresAPIKey`
