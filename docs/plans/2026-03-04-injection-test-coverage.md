# 2026-03-04 Injection Test Coverage

## Status
- [x] Audit existing injection path and test gaps
- [x] Add deterministic command-injection seam for tmux send-keys
- [x] Add `reply_agent` branch coverage tests (validation + inject + external reply)
- [x] Add `BackgroundTaskManager.InjectBackgroundInput` branch coverage tests
- [x] Run lint/tests and commit

## Scope
- Cover all meaningful cases for user-input injection path:
1. Tool-layer validation and branch routing (`reply_agent`)
2. Runtime injection behavior (`InjectBackgroundInput`) including validation, not-found, no-pane, command failure, success
