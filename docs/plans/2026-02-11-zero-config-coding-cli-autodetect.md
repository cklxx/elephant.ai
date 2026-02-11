# Plan: Zero-Config Coding CLI Auto-Detect (2026-02-11)

## Goal
- Make coding-agent delegation work without manual `runtime.external_agents` configuration.
- Auto-detect local coding CLIs at startup (no extra user command).

## Scope
- Upgrade runtime external-agent auto-enable logic (codex/claude) to be configless-first.
- Add startup-time coding CLI diagnostics (`codex`, `claude`, `kimi` variants) without introducing new user commands.
- Auto-apply detected supported CLIs into DI bootstrap so external executor is available without manual enable flags.
- Ensure Codex bridge can use absolute binary paths (not PATH-only).
- Keep explicit YAML settings highest priority (auto-detect must not override user intent).

## Out of Scope
- Implementing new external bridge executors for unsupported CLIs (e.g., Kimi CLI execution path).
- Expanding `bg_dispatch` supported `agent_type` beyond currently integrated executors.

## Checklist
- [x] Update runtime external-agent auto-enable logic for zero-config startup.
- [x] Extend coding CLI detector to include structured output and extra candidates.
- [x] Wire startup-time coding CLI detection logs in container bootstrap.
- [x] Add/adjust tests for auto-detect and CLI detector behavior.
- [x] Run lint + focused tests + full tests required for delivery.
- [x] Run mandatory code review workflow, fix issues, then commit incrementally.

## Progress Log
- 2026-02-11 15:58: Plan created after user requested zero-config auto-detect strategy (codex/claude/kimi visibility).
- 2026-02-11 16:06: Scope adjusted: no new commands; detection runs automatically during startup.
- 2026-02-11 16:28: Implemented startup auto-detect logging and structured local CLI detector (`codex` / `claude` / `kimi` visibility).
- 2026-02-11 16:35: Added fallback binary detection with binary-source precedence rules; explicit configured binary no longer falls back.
- 2026-02-11 16:40: Ran focused and full validations (`./dev.sh lint`, `./dev.sh test`) in worktree; all green.
- 2026-02-11 16:55: Completed mandatory code review (P0/P1 none outstanding); added regression coverage for binary-source precedence and production startup auto-enable.
- 2026-02-11 17:20: Fixed runtime startup gap where detected CLIs were only logged but not applied; DI now auto-enables codex/claude executors from detection.
- 2026-02-11 17:22: Added codex binary path passthrough from runtime config -> bridge config -> `scripts/codex_bridge/codex_bridge.py` execution command.
- 2026-02-11 17:25: Fixed upstream race regressions after rebasing latest `main` by serializing session persistence and async save snapshots.
- 2026-02-11 17:30: Re-ran full validation on rebased branch (`./dev.sh lint`, `./dev.sh test`), all packages green including previously failing coordinator/scheduler/integration race paths.
