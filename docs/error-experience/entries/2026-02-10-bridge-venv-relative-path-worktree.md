# Bridge Venv Relative Path + Missing Auto-Provision

**Date:** 2026-02-10
**Severity:** P1 — blocks all Claude Code background tasks
**Component:** `internal/infra/external/bridge/executor.go`

## Symptom

```
start bridge: fork/exec scripts/cc_bridge/.venv/bin/python3: no such file or directory
```

All Claude Code background tasks fail to start.

## Root Cause

Two compounding bugs in `resolvePython()` / `resolveBridgeScript()`:

1. **Relative path resolution**: When the exe-relative bridge script lookup fails (common when `go run` or the binary isn't in the repo tree), `resolveBridgeScript()` fell back to a relative path `scripts/cc_bridge/cc_bridge.py`. The derived venv path `scripts/cc_bridge/.venv/bin/python3` was also relative. `os.Stat` succeeded because the main process CWD is the repo root, but the subprocess started with `WorkingDir: req.WorkingDir` (a background worktree like `.elephant/worktrees/bg-xxx/`), so `fork/exec` couldn't resolve the relative path.

2. **No auto-provisioning**: If the venv was missing entirely (e.g., fresh clone, worktree, or deleted), no recovery path existed. The executor silently fell back to system `python3`, which lacked `claude-agent-sdk`.

## Fix

- `resolveBridgeScript()`: Always convert paths to absolute via `filepath.Abs()`
- `resolvePython()`: Added `ensureVenv()` that auto-runs `setup.sh` when the venv is missing
- 5 new tests covering absolute path resolution, venv detection, auto-provisioning, and fallback

## Lesson

- Any path resolved for subprocess execution must be absolute — the subprocess `WorkingDir` may differ from the resolver's CWD.
- External dependencies (venvs, binaries) should be auto-provisioned with clear error messages, not silently skipped.
