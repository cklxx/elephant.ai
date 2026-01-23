# ACP Test Helper Defaults Plan

**Goal:** Make `scripts/acp_test.py` work without manual `--addr` by auto-detecting the ACP port, so local smoke tests do not fail with connection refused or missing flags.

## Plan
1) Resolve ACP address from environment or `.pids/acp.port` when `--addr` is omitted.
2) Normalize `cwd` to an absolute path to match ACP session requirements.
3) Run full lint and test suite to validate changes.

## Progress Log
- 2026-01-22: Implemented ACP address auto-detection (env + `.pids/acp.port`) and absolute cwd normalization in `scripts/acp_test.py`.
- 2026-01-22: Added cwd fallback order (`ACP_CWD` → `/workspace` → local cwd`) when `--cwd` omitted.
- 2026-01-22: Tests run: `./dev.sh lint`, `./dev.sh test`.
